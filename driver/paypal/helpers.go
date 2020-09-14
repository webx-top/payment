package paypal

import "github.com/webx-top/payment"

// MoneyFeeToString 支付宝金额转字符串
func MoneyFeeToString(moneyFee float64) string {
	return payment.CutFloat(moneyFee, 2)
}
