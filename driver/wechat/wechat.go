package wechat

import (
	"io/ioutil"

	"github.com/admpub/log"
	"github.com/objcoding/wxpay"
	"github.com/webx-top/echo"
	"github.com/webx-top/echo/param"
	"github.com/webx-top/payment"
	"github.com/webx-top/payment/config"
	//"github.com/objcoding/wxpay"
)

func init() {
	payment.Register(config.Platform(`wechat`), New)
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
	wechatAccount := wxpay.NewAccount(
		account.AppID,
		account.MerchantID,
		account.AppSecret,
		account.Debug,
	)
	wechatAccount.SetCertData(account.CertPath)
	a.client = wxpay.NewClient(wechatAccount)
	return a
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
	result = param.ToStringMap(params)
	if params["return_code"] != "SUCCESS" {
		return config.ErrPaymentFailed
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

	return ctx.Blob([]byte(xmlString))
}

func (a *Wechat) Refund(cfg *config.Refund) (param.StringMap, error) {
	result := param.StringMap{}
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
	_ = resp
	return result, err
}
