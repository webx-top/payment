package mollie

import (
	"strings"

	"github.com/webx-top/echo"
	"github.com/webx-top/payment"
	"github.com/webx-top/payment/config"
)

// MoneyFeeToString 金额转字符串
func MoneyFeeToString(moneyFee float64) string {
	return payment.RoundFloat(moneyFee, 2)
}

func defaultString(val, def string) string {
	if len(val) == 0 {
		return def
	}
	return val
}

// MappingStatus 映射 Mollie 支付状态
// 状态文档: https://docs.mollie.com/status/mollie-payment-statuses
func MappingStatus(status string, result *config.Result) {
	switch status {
	case "open":
		result.Status = config.TradeStatusWaitBuyerPay
	case "pending":
		result.Status = config.TradeStatusProcessing
	case "paid", "authorized":
		result.Status = config.TradeStatusSuccess
	case "expired":
		result.Status = config.TradeStatusClosed
	case "canceled":
		result.Status = config.TradeStatusClosed
	case "failed":
		result.Status = config.TradeStatusException
	default:
		result.Status = config.TradeStatusWaitBuyerPay
	}
}

// MapRefundStatus 映射 Mollie 退款状态
// 状态文档: https://docs.mollie.com/status/mollie-refund-statuses
func MapRefundStatus(status string) string {
	switch status {
	case "queued":
		return config.TradeStatusProcessing
	case "pending":
		return config.TradeStatusProcessing
	case "processing":
		return config.TradeStatusProcessing
	case "refunded":
		return config.TradeStatusSuccess
	case "failed":
		return config.TradeStatusException
	default:
		return config.TradeStatusProcessing
	}
}

// VerifySign 验证签名(Mollie使用IP白名单+Bearer Token验证)
func (a *Mollie) VerifySign(ctx echo.Context) error {
	// Mollie webhook 不使用签名验证，而是依赖 IP 白名单和 Bearer Token
	// 这里做基本检查
	auth := ctx.Request().Header().Get("Authorization")
	if !strings.HasPrefix(auth, "Bearer ") {
		return echo.NewError("missing authorization header")
	}
	return nil
}
