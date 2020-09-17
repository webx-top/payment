package paypal

import (
	"net/url"

	"github.com/webx-top/echo"
	"github.com/webx-top/payment"
	"github.com/webx-top/payment/config"
)

// MoneyFeeToString 支付宝金额转字符串
func MoneyFeeToString(moneyFee float64) string {
	return payment.CutFloat(moneyFee, 2)
}

func (a *Paypal) VerifySign(ctx echo.Context, req url.Values) error {
	return config.ErrUnsupported
}
