package payment

import "github.com/admpub/decimal"

// CutFloat 非四舍五入的方式保留小数位数
// @param money 金额
// @param precision 小数位数
func CutFloat(money float64, precision int32) string {
	return decimal.NewFromFloat(money).Truncate(precision).String()
}

// MulFloat 小数相乘
// @param money 金额
// @param multiple 乘数
// @param precision 小数位数
func MulFloat(money float64, multiple float64, precision int32) string {
	aDecimal := decimal.NewFromFloat(money)
	bDecimal := decimal.NewFromFloat(multiple)
	return aDecimal.Mul(bDecimal).Truncate(precision).String()
}
