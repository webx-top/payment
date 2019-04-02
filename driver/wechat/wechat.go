package wechat

import (
	"fmt"
	"io/ioutil"

	"github.com/objcoding/wxpay"
	"github.com/webx-top/echo"
	"github.com/webx-top/echo/param"
	"github.com/webx-top/payment"
	"github.com/webx-top/payment/config"
	//"github.com/objcoding/wxpay"
)

func init() {
	payment.Register(config.Platform(`wechat`), &Wechat{})
}

type Wechat struct {
	account *config.Account
	client  *wxpay.Client
}

func (a *Wechat) SetAccount(account *config.Account) payment.Hook {
	a.account = account
	wechatAccount := wxpay.NewAccount(account.ClientID, account.MerchantID, account.ClientSecret, account.Debug)
	wechatAccount.SetCertData(account.CertPath)
	a.client = wxpay.NewClient(wechatAccount)
	return a
}

func (a *Wechat) Pay(cfg *config.Pay) (param.StringMap, error) {
	wxParams := wxpay.Params{
		"notify_url":   cfg.NotifyURL,
		"trade_type":   cfg.DeviceType(),
		"total_fee":    wxpayAmount(cfg.Amount),
		"out_trade_no": cfg.TradeNo,
		"body":         cfg.Subject,
	}
	params, err := a.client.UnifiedOrder(wxParams)
	if err != nil {
		return nil, err
	}
	return param.ToStringMap(a.translateWxpayAppResult(cfg, params)), nil
}

func (a *Wechat) Notify(ctx echo.Context) (param.StringMap, error) {
	result := param.StringMap{}
	body := ctx.Request().Body()
	defer body.Close()
	b, err := ioutil.ReadAll(body)
	if err != nil {
		return result, err
	}
	params := wxpay.XmlToMap(string(b))
	if !a.client.ValidSign(params) {
		return result, fmt.Errorf("签名失败")
	}
	result = param.ToStringMap(params)
	if params["return_code"] != "SUCCESS" {
		return result, fmt.Errorf("支付失败")
	}
	return result, nil
}

func (a *Wechat) Refund(cfg *config.Refund) (param.StringMap, error) {
	result := param.StringMap{}
	refundConfig := &wxpay.Params{
		"out_trade_no":  cfg.TradeNo,
		"out_refund_no": cfg.RefundNo,
		"total_fee":     wxpayAmount(cfg.TotalAmount),
		"refund_fee":    wxpayAmount(cfg.RefundAmount),
	}
	resp, err := a.client.Refund(*refundConfig)
	if err != nil {
		return nil, err
	}
	_ = resp
	return result, err
}
