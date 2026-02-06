package payment

import "github.com/admpub/decimal"

// ==== 返回 string ====

// CutFloat 非四舍五入的方式保留小数位数
// * money 金额
// * precision 小数位数
func CutFloat(money float64, precision int32) string {
	return decimal.NewFromFloat(money).Truncate(precision).String()
}

// RoundFloat 四舍五入的方式保留小数位数
// * money 金额
// * precision 小数位数
func RoundFloat(money float64, precision int32) string {
	return decimal.NewFromFloat(money).Round(precision).String()
}

// MulFloat 小数相乘
// * money 金额
// * multiple 乘数
// * precision 小数位数
func MulFloat(money float64, multiple float64, precision int32) string {
	aDecimal := decimal.NewFromFloat(money)
	bDecimal := decimal.NewFromFloat(multiple)
	return aDecimal.Mul(bDecimal).Round(precision).String()
}

// AddFloat 小数相加
// * money 金额
// * money2 金额2
// * precision 小数位数
func AddFloat(money float64, money2 float64, precision int32) string {
	aDecimal := decimal.NewFromFloat(money)
	bDecimal := decimal.NewFromFloat(money2)
	return aDecimal.Add(bDecimal).Round(precision).String()
}

// SubFloat 小数相减
// * money 金额
// * money2 金额2
// * precision 小数位数
func SubFloat(money float64, money2 float64, precision int32) string {
	aDecimal := decimal.NewFromFloat(money)
	bDecimal := decimal.NewFromFloat(money2)
	return aDecimal.Sub(bDecimal).Round(precision).String()
}

// DivFloat 小数相除
// * money 金额
// * money2 金额2
// * precision 小数位数
func DivFloat(money float64, money2 float64, precision int32) string {
	aDecimal := decimal.NewFromFloat(money)
	bDecimal := decimal.NewFromFloat(money2)
	return aDecimal.Div(bDecimal).Round(precision).String()
}

// ==== 返回 float64 ====

// Cut 非四舍五入的方式保留小数位数
// * money 金额
// * precision 小数位数
func Cut(money float64, precision int32) float64 {
	return decimal.NewFromFloat(money).Truncate(precision).InexactFloat64()
}

// Round 四舍五入的方式保留小数位数
// * money 金额
// * precision 小数位数
func Round(money float64, precision int32) float64 {
	return decimal.NewFromFloat(money).Round(precision).InexactFloat64()
}

// Mul 小数相乘
// * money 金额
// * multiple 乘数
// * precision 小数位数
func Mul(money float64, multiple float64, precision int32) float64 {
	aDecimal := decimal.NewFromFloat(money)
	bDecimal := decimal.NewFromFloat(multiple)
	return aDecimal.Mul(bDecimal).Round(precision).InexactFloat64()
}

// Add 小数相加
// * money 金额
// * money2 金额2
// * precision 小数位数
func Add(money float64, money2 float64, precision int32) float64 {
	aDecimal := decimal.NewFromFloat(money)
	bDecimal := decimal.NewFromFloat(money2)
	return aDecimal.Add(bDecimal).Round(precision).InexactFloat64()
}

// Sub 小数相减
// * money 金额
// * money2 金额2
// * precision 小数位数
func Sub(money float64, money2 float64, precision int32) float64 {
	aDecimal := decimal.NewFromFloat(money)
	bDecimal := decimal.NewFromFloat(money2)
	return aDecimal.Sub(bDecimal).Round(precision).InexactFloat64()
}

// Div 小数相除
// * money 金额
// * money2 金额2
// * precision 小数位数
func Div(money float64, money2 float64, precision int32) float64 {
	aDecimal := decimal.NewFromFloat(money)
	bDecimal := decimal.NewFromFloat(money2)
	return aDecimal.Div(bDecimal).Round(precision).InexactFloat64()
}
