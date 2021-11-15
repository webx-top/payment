package payment

import "github.com/admpub/decimal"

// ==== string ====

// CutFloat 非四舍五入的方式保留小数位数
// * money 金额
// * precision 小数位数
func CutFloat(money float64, precision int32) string {
	return decimal.NewFromFloat(money).Truncate(precision).String()
}

// MulFloat 小数相乘
// * money 金额
// * multiple 乘数
// * precision 小数位数
func MulFloat(money float64, multiple float64, precision int32) string {
	aDecimal := decimal.NewFromFloat(money)
	bDecimal := decimal.NewFromFloat(multiple)
	return aDecimal.Mul(bDecimal).Truncate(precision).String()
}

// AddFloat 小数相加
// * money 金额
// * money2 金额2
// * precision 小数位数
func AddFloat(money float64, money2 float64, precision int32) string {
	aDecimal := decimal.NewFromFloat(money)
	bDecimal := decimal.NewFromFloat(money2)
	return aDecimal.Add(bDecimal).Truncate(precision).String()
}

// SubFloat 小数相减
// * money 金额
// * money2 金额2
// * precision 小数位数
func SubFloat(money float64, money2 float64, precision int32) string {
	aDecimal := decimal.NewFromFloat(money)
	bDecimal := decimal.NewFromFloat(money2)
	return aDecimal.Sub(bDecimal).Truncate(precision).String()
}

// DivFloat 小数相除
// * money 金额
// * money2 金额2
// * precision 小数位数
func DivFloat(money float64, money2 float64, precision int32) string {
	aDecimal := decimal.NewFromFloat(money)
	bDecimal := decimal.NewFromFloat(money2)
	return aDecimal.Div(bDecimal).Truncate(precision).String()
}

// ==== float64 ====

// Cut 非四舍五入的方式保留小数位数
// * money 金额
// * precision 小数位数
func Cut(money float64, precision int32) float64 {
	v, _ := decimal.NewFromFloat(money).Truncate(precision).Float64()
	return v
}

// Mul 小数相乘
// * money 金额
// * multiple 乘数
// * precision 小数位数
func Mul(money float64, multiple float64, precision int32) float64 {
	aDecimal := decimal.NewFromFloat(money)
	bDecimal := decimal.NewFromFloat(multiple)
	v, _ := aDecimal.Mul(bDecimal).Truncate(precision).Float64()
	return v
}

// Add 小数相加
// * money 金额
// * money2 金额2
// * precision 小数位数
func Add(money float64, money2 float64, precision int32) float64 {
	aDecimal := decimal.NewFromFloat(money)
	bDecimal := decimal.NewFromFloat(money2)
	v, _ := aDecimal.Add(bDecimal).Truncate(precision).Float64()
	return v
}

// Sub 小数相减
// * money 金额
// * money2 金额2
// * precision 小数位数
func Sub(money float64, money2 float64, precision int32) float64 {
	aDecimal := decimal.NewFromFloat(money)
	bDecimal := decimal.NewFromFloat(money2)
	v, _ := aDecimal.Sub(bDecimal).Truncate(precision).Float64()
	return v
}

// Div 小数相除
// * money 金额
// * money2 金额2
// * precision 小数位数
func Div(money float64, money2 float64, precision int32) float64 {
	aDecimal := decimal.NewFromFloat(money)
	bDecimal := decimal.NewFromFloat(money2)
	v, _ := aDecimal.Div(bDecimal).Truncate(precision).Float64()
	return v
}
