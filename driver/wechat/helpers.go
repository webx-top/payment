package wechat

import (
	"strconv"
	"time"

	"github.com/objcoding/wxpay"
	"github.com/shopspring/decimal"
	"github.com/webx-top/payment/config"
)

func wxpayAmount(amount float64) string {
	aDecimal := decimal.NewFromFloat(amount)
	bDecimal := decimal.NewFromFloat(100)
	return aDecimal.Mul(bDecimal).Truncate(0).String()
}

func (a *Wechat) translateWxpayAppResult(tradePay *config.Pay, params wxpay.Params) map[string]string {
	if tradePay.Device == config.App {
		p := make(wxpay.Params)
		p["appid"] = params["appid"]
		p["partnerid"] = params["mch_id"]
		p["noncestr"] = params["nonce_str"]
		p["prepayid"] = params["prepay_id"]
		p["timestamp"] = strconv.FormatInt(time.Now().Unix(), 10)
		p["package"] = "Sign=WXPay"
		p["sign"] = a.client.Sign(p)
		return map[string]string(p)
	}
	return map[string]string(params)
}
