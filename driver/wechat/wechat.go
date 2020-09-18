package wechat

import (
	"encoding/base64"
	"errors"
	"fmt"
	"io/ioutil"
	"strconv"
	"strings"

	"github.com/admpub/log"
	"github.com/objcoding/wxpay"
	"github.com/webx-top/codec"
	"github.com/webx-top/com"
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

func (a *Wechat) Pay(ctx echo.Context, cfg *config.Pay) (param.StringMap, error) {
	var tradeType string
	switch cfg.Device {
	case config.Web:
		tradeType = `NATIVE`
	case config.Wap:
		tradeType = `MWEB`
	case config.App:
		tradeType = `App`
	default:
		tradeType = `MWEB`
	}
	if strings.Contains(ctx.Request().UserAgent(), `MicroMessenger`) {
		tradeType = `JSAPI`
	}
	wxParams := wxpay.Params{
		"notify_url":   cfg.NotifyURL,
		"trade_type":   tradeType, //JSAPI:JSAPI支付（或小程序支付）、NATIVE:Native支付、APP:app支付，MWEB:H5支付
		"total_fee":    MoneyFeeToString(cfg.Amount),
		"out_trade_no": cfg.OutTradeNo,
		"body":         cfg.Subject,
		"scene_info":   ``,
	}
	if cfg.Options != nil {
		params := cfg.Options.Store(`params`)
		for k := range params {
			wxParams[k] = params.String(k)
		}
	}
	params, err := a.Client().UnifiedOrder(wxParams)
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
	if !a.Client().ValidSign(params) {
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

	result[`operation`] = `payment`
	if reqInfo, y := result[`req_info`]; y {
		result[`operation`] = `refund`
		b, err := base64.StdEncoding.DecodeString(reqInfo.String())
		if err != nil {
			fmt.Println(b, err)
			return nil
		}
		key := strings.ToLower(com.Md5(a.account.AppSecret))
		crypto := codec.NewAesECBCrypto(`AES-256`)
		b = crypto.DecodeBytes(b, []byte(key))
		for k, v := range XmlToMap(string(b)) {
			result[k] = param.String(v)
		}
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

func (a *Wechat) Query(ctx echo.Context, cfg *config.Query) (config.TradeStatus, error) {
	params := make(wxpay.Params)
	if len(cfg.TradeNo) > 0 {
		params.SetString("transaction_id", cfg.TradeNo)
	} else {
		params.SetString("out_trade_no", cfg.OutTradeNo)
	}
	resp, err := a.Client().OrderQuery(params)
	if err != nil {
		return config.EmptyTradeStatus, err
	}
	if resp.GetString(`return_code`) != wxpay.Success {
		return config.EmptyTradeStatus, errors.New(resp.GetString(`return_msg`))
	}
	tradeStatus := resp.GetString(`trade_state`)
	/*
		SUCCESS—支付成功
		REFUND—转入退款
		NOTPAY—未支付
		CLOSED—已关闭
		REVOKED—已撤销（付款码支付）
		USERPAYING--用户支付中（付款码支付）
		PAYERROR--支付失败(其他原因，如银行返回失败)
	*/
	switch tradeStatus {
	case `SUCCESS`:
		tradeStatus = config.TradeStatusSuccess
	case `REFUND`, `CLOSED`:
		tradeStatus = config.TradeStatusClosed
	case `NOTPAY`, `REVOKED`, `USERPAYING`, `PAYERROR`:
		tradeStatus = config.TradeStatusWaitBuyerPay
	}
	return config.NewTradeStatus(tradeStatus, echo.H{
		`trade_no`:     resp.GetString(`transaction_id`),
		`out_trade_no`: resp.GetString(`out_trade_no`),
		`currency`:     resp.GetString(`fee_type`),
		`total_amount`: payment.CutFloat(float64(resp.GetInt64(`total_fee`))/100, 2),
	}), err
}

func (a *Wechat) Refund(ctx echo.Context, cfg *config.Refund) (param.StringMap, error) {
	refundConfig := wxpay.Params{
		"out_trade_no":  cfg.OutTradeNo,
		"out_refund_no": cfg.OutRefundNo,
		"total_fee":     MoneyFeeToString(cfg.TotalAmount),
		"refund_fee":    MoneyFeeToString(cfg.RefundAmount),
	}
	resp, err := a.Client().Refund(refundConfig)
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
