package wechat

import (
	"io/ioutil"
	"strconv"

	"github.com/admpub/log"
	"github.com/objcoding/wxpay"
	"github.com/webx-top/echo"
	"github.com/webx-top/echo/param"
	"github.com/webx-top/payment"
	"github.com/webx-top/payment/config"
)

const Name = `wechat`

func init() {
	payment.Register(Name, `微信支付`, New)
}

func New() payment.Hook {
	return &Wechat{}
}

type Wechat struct {
	account        *config.Account
	client         *wxpay.Client
	notifyCallback func(echo.Context) error
}

func (a *Wechat) SetNotifyCallback(callback func(echo.Context) error) payment.Hook {
	a.notifyCallback = callback
	return a
}

func (a *Wechat) SetAccount(account *config.Account) payment.Hook {
	a.account = account
	return a
}

func (a *Wechat) Client() *wxpay.Client {
	if a.client != nil {
		return a.client
	}
	wechatAccount := wxpay.NewAccount(
		a.account.AppID,
		a.account.MerchantID,
		a.account.AppSecret,
		a.account.Debug,
	)
	if len(a.account.CertPath) > 0 {
		wechatAccount.SetCertData(a.account.CertPath)
	}
	a.client = wxpay.NewClient(wechatAccount)
	return a.client
}

func (a *Wechat) Pay(cfg *config.Pay) (param.StringMap, error) {
	wxParams := wxpay.Params{
		"notify_url":   cfg.NotifyURL,
		"trade_type":   cfg.DeviceType(),
		"total_fee":    MoneyFeeToString(cfg.Amount),
		"out_trade_no": cfg.TradeNo,
		"body":         cfg.Subject,
	}
	params, err := a.client.UnifiedOrder(wxParams)
	if err != nil {
		return nil, err
	}
	return param.ToStringMap(a.translateWxpayAppResult(cfg, params)), nil
}

func (a *Wechat) Notify(ctx echo.Context) error {
	result := param.StringMap{}
	body := ctx.Request().Body()
	defer body.Close()
	b, err := ioutil.ReadAll(body)
	if err != nil {
		return err
	}
	params := wxpay.XmlToMap(string(b))
	if !a.client.ValidSign(params) {
		return config.ErrSignature
	}
	if params["return_code"] != "SUCCESS" {
		return config.ErrPaymentFailed
	}
	result = param.ToStringMap(params)
	if v, y := result[`total_fee`]; y {
		cents, err := strconv.ParseInt(v.String(), 10, 64)
		if err != nil {
			log.Error(err)
		}
		result[`total_amount`] = param.String(payment.CutFloat(float64(cents)/100, 2))
	}
	if v, y := result[`transaction_id`]; y {
		result[`trade_no`] = v
	}

	var isSuccess = true
	var xmlString string
	noti := wxpay.Notifies{}
	if a.notifyCallback != nil {
		ctx.Set(`notify`, result)
		if err := a.notifyCallback(ctx); err != nil {
			log.Error(err)
			isSuccess = false
		}
	}
	if !isSuccess {
		xmlString = noti.NotOK("faild")
	} else {
		xmlString = noti.OK()
	}

	return ctx.XMLBlob([]byte(xmlString))
}

func (a *Wechat) Refund(cfg *config.Refund) (param.StringMap, error) {
	refundConfig := wxpay.Params{
		"out_trade_no":  cfg.TradeNo,
		"out_refund_no": cfg.RefundNo,
		"total_fee":     MoneyFeeToString(cfg.TotalAmount),
		"refund_fee":    MoneyFeeToString(cfg.RefundAmount),
	}
	resp, err := a.client.Refund(refundConfig)
	if err != nil {
		return nil, err
	}
	returnCode := resp.GetString("return_code")
	resp[`success`] = ``
	if returnCode == wxpay.Fail {
		resp[`success`] = `0`
	} else if returnCode == wxpay.Success {
		resp[`success`] = `1`
	}
	return param.ToStringMap(resp), err
}
