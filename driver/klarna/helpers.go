package klarna

import (
	"fmt"
	"io"

	"github.com/webx-top/echo"
	"github.com/webx-top/payment"
	"github.com/webx-top/payment/config"
)

// AmountToMinorInt 金额转最小单位整数(Klarna要求整数，如10000=100.00 EUR)
func AmountToMinorInt(amount float64) int64 {
	return int64(payment.Round(amount*100, 0))
}

// MinorToAmount 最小单位整数转金额
func MinorToAmount(minor int64) float64 {
	return float64(minor) / 100
}

// MoneyFeeToString 金额转字符串
func MoneyFeeToString(moneyFee float64) string {
	return payment.RoundFloat(moneyFee, 2)
}

func defaultString(val, def string) string {
	if len(val) == 0 { return def }
	return val
}

func totalAmount(amount int64) float64 {
	return MinorToAmount(amount)
}

// countryFromCurrency 根据货币推断国家代码
func countryFromCurrency(currency string) string {
	countryMap := map[string]string{
		"EUR": "DE", "USD": "US", "GBP": "GB", "SEK": "SE",
		"NOK": "NO", "DKK": "DK", "PLN": "PL", "CHF": "CH",
		"CAD": "CA", "AUD": "AU", "NZD": "NZ",
	}
	code, ok := countryMap[currency]
	if ok { return code }
	return "US"
}

// MappingStatus 映射 Klarna 订单状态
// 状态文档: https://developers.klarna.com/documentation/klarna-checkout/additional-info/order-statuses/
func MappingStatus(status string, result *config.Result) {
	switch status {
	case "checkout_complete":
		result.Status = config.TradeStatusSuccess
	case "created":
		result.Status = config.TradeStatusWaitBuyerPay
	case "canceled":
		result.Status = config.TradeStatusClosed
	case "expired":
		result.Status = config.TradeStatusClosed
	default:
		result.Status = config.TradeStatusWaitBuyerPay
	}
}

// MapRefundStatus 映射 Klarna 退款状态
func MapRefundStatus(status string) string {
	switch status {
	case "SUCCESSFUL":
		return config.TradeStatusSuccess
	case "PENDING":
		return config.TradeStatusProcessing
	case "FAILED":
		return config.TradeStatusException
	default:
		return config.TradeStatusProcessing
	}
}

// VerifySign 验证签名(Klarna使用HMAC-SHA256验证回调)
// 文档: https://developers.klarna.com/documentation/klarna-checkout/integration-guide-js/#verification
func (a *Klarna) VerifySign(ctx echo.Context) error {
	signature := ctx.Request().Header().Get("X-Klarna-Signature")
	body, err := io.ReadAll(ctx.Request().Body())
	if err != nil {
		return err
	}

	if len(signature) == 0 || len(body) == 0 {
		return fmt.Errorf("missing signature or body")
	}

	// TODO: 完整的 Klarna 签名验证实现
	// Klarna 签名基于 shared secret + request body
	return nil
}
