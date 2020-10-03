package payjs

import (
	"github.com/webx-top/echo"
	"github.com/webx-top/payment"
)

// MoneyFeeToString 微信金额浮点转字符串
func MoneyFeeToString(moneyFee float64) string {
	return payment.MulFloat(moneyFee, 100, 0)
}

func (a *PayJS) VerifySign(ctx echo.Context) error {
	return nil
}
