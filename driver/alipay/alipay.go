package alipay

import (
	"net/url"

	"github.com/admpub/log"
	"github.com/smartwalle/alipay"
	"github.com/webx-top/echo"
	"github.com/webx-top/echo/param"
	"github.com/webx-top/payment"
	"github.com/webx-top/payment/config"
)

func init() {
	payment.Register(config.Platform(`alipay`), New)
}

func New() payment.Hook {
	return &Alipay{}
}

type Alipay struct {
	account        *config.Account
	client         *alipay.AliPay
	notifyCallback func(echo.Context) error
}

func (a *Alipay) SetNotifyCallback(callback func(echo.Context) error) payment.Hook {
	a.notifyCallback = callback
	return a
}

func (a *Alipay) SetAccount(account *config.Account) payment.Hook {
	a.account = account
	a.client = alipay.New(
		account.AppID,
		account.PublicKey,
		account.PrivateKey,
		!account.Debug,
	)
	return a
}

func (a *Alipay) Pay(cfg *config.Pay) (param.StringMap, error) {
	payConfig := alipay.TradePay{
		NotifyURL:   cfg.NotifyURL,
		Subject:     cfg.Subject,
		OutTradeNo:  cfg.TradeNo,
		TotalAmount: MoneyFeeToString(cfg.Amount),
		ProductCode: "QUICK_WAP_WAY",
	}
	var err error
	result := param.StringMap{}
	switch cfg.Device {
	case config.App:
		pay := alipay.AliPayTradeAppPay{TradePay: payConfig}
		results, err := a.client.TradeAppPay(pay)
		if err != nil {
			return result, err
		}
		result["orderString"] = param.String(results)
	case config.Web:
		pay := alipay.AliPayTradePagePay{TradePay: payConfig}
		url, err := a.client.TradePagePay(pay)
		if err != nil {
			return result, err
		}
		result["orderString"] = param.String(url.String())
	default:
		return nil, config.ErrUnknowDevice
	}
	return result, err
}

func (a *Alipay) Notify(ctx echo.Context) error {
	formData := url.Values(ctx.Forms())
	notify, err := a.getAlipayTradeNotificationData(formData)
	if err != nil {
		log.Error(err)
		return err
	}
	var isSuccess = true
	if a.notifyCallback != nil {
		ctx.Set(`notify`, notify)
		if err := a.notifyCallback(ctx); err != nil {
			log.Error(err)
			isSuccess = false
		}
	}
	if isSuccess {
		err = config.NewOKString(`success`)
	} else {
		err = config.NewOKString(`faild`)
	}
	return ctx.String(err.Error())
}

func (a *Alipay) Refund(cfg *config.Refund) (param.StringMap, error) {
	result := param.StringMap{}
	refundConfig := alipay.AliPayTradeRefund{
		OutTradeNo:   cfg.TradeNo,
		RefundAmount: MoneyFeeToString(cfg.RefundAmount),
	}
	resp, err := a.client.TradeRefund(refundConfig)
	_ = resp
	return result, err
}
