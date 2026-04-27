package stripe

import (
	"fmt"
	"io"

	"github.com/stripe/stripe-go/v81"
	"github.com/stripe/stripe-go/v81/webhook"
	"github.com/webx-top/echo"
	"github.com/webx-top/payment"
	"github.com/webx-top/payment/config"
)

var zeroCurrencies = map[string]struct{}{
	"BIF": {}, "CLP": {}, "DJF": {}, "GNF": {},
	"JPY": {}, "KMF": {}, "KRW": {}, "MGA": {},
	"PYG": {}, "RWF": {}, "UGX": {}, "VND": {},
	"VUV": {}, "XAF": {}, "XOF": {}, "XPF": {},
}

// isZeroDecimalCurrency 检查是否为零小数位货币
func isZeroDecimalCurrency(currency string) bool {
	_, ok := zeroCurrencies[currency]
	return ok
}

// AmountToCent 将金额转换为Stripe最小单位(分)
// 对于零小数位货币(JPY等)，不乘以100
func AmountToCent(amount float64, currency string) int64 {
	if isZeroDecimalCurrency(currency) {
		return int64(payment.Round(amount, 0))
	}
	return int64(payment.Round(amount*100, 0))
}

// CentToAmount 将Stripe最小单位(分)转换为金额
func CentToAmount(cent int64, currency string) float64 {
	if isZeroDecimalCurrency(currency) {
		return float64(cent)
	}
	return float64(cent) / 100
}

// CurrencyToLower 将货币代码转为小写(Stripe API使用小写)
func CurrencyToLower(currency string) string {
	for _, c := range currency {
		if c >= 'A' && c <= 'Z' {
			return toLower(currency)
		}
		break
	}
	return currency
}

func toLower(s string) string {
	r := []rune(s)
	for i := range r {
		if r[i] >= 'A' && r[i] <= 'Z' {
			r[i] += 32
		}
	}
	return string(r)
}

// MoneyFeeToString 金额转字符串
func MoneyFeeToString(moneyFee float64) string {
	return payment.RoundFloat(moneyFee, 2)
}

// MappingStatus 映射Stripe PaymentIntent状态到项目标准状态
// 状态文档: https://stripe.com/docs/payments/intents#intent-statuses
func MappingStatus(status string) string {
	switch status {
	case string(stripe.PaymentIntentStatusSucceeded):
		return config.TradeStatusSuccess
	case string(stripe.PaymentIntentStatusCanceled):
		return config.TradeStatusClosed
	case string(stripe.PaymentIntentStatusProcessing):
		return config.TradeStatusProcessing
	case string(stripe.PaymentIntentStatusRequiresAction),
		string(stripe.PaymentIntentStatusRequiresConfirmation),
		string(stripe.PaymentIntentStatusRequiresCapture):
		return config.TradeStatusWaitBuyerPay
	default:
		return config.TradeStatusWaitBuyerPay
	}
}

// MapRefundStatus 映射Stripe Refund状态到项目标准状态
// 状态文档: https://stripe.com/docs/api/refunds/object#refund_object-status
func MapRefundStatus(status string) string {
	switch status {
	case string(stripe.RefundStatusSucceeded):
		return config.TradeStatusSuccess
	case string(stripe.RefundStatusCanceled):
		return config.TradeStatusClosed
	case string(stripe.RefundStatusFailed):
		return config.TradeStatusException
	case string(stripe.RefundStatusPending):
		return config.TradeStatusProcessing
	default:
		return config.TradeStatusProcessing
	}
}

// VerifySign 验证签名
func (a *Stripe) VerifySign(ctx echo.Context) error {
	if a.account == nil || a.account.WebhookID == `` {
		return fmt.Errorf("stripe: account or WebhookID not set")
	}
	body, err := io.ReadAll(ctx.Request().Body())
	if err != nil {
		return err
	}

	_, err = a.verifySignAndGetEvent(ctx, body)
	return err
}

// verifySignAndGetEvent 验证Stripe Webhook签名并解析事件
// 从请求头获取 Stripe-Signature，使用 Webhook Secret 验证签名并构造 stripe.Event 对象
// 文档: https://stripe.com/docs/webhooks/signatures
func (a *Stripe) verifySignAndGetEvent(ctx echo.Context, body []byte) (stripe.Event, error) {
	signature := ctx.Request().Header().Get("Stripe-Signature")
	return webhook.ConstructEvent(body, signature, a.account.WebhookID)
}
