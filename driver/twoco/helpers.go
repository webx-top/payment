package twoco

import (
	"fmt"
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

// MappingStatus 映射 2Checkout 销售状态
func MappingStatus(status string) string {
	switch strings.ToLower(status) {
	case "pending", "pending-review":
		return config.TradeStatusProcessing
	case "complete", "recurring", "active":
		return config.TradeStatusSuccess
	case "cancelled", "expired":
		return config.TradeStatusClosed
	case "rejected", "fraud", "chargeback", "refunded":
		return config.TradeStatusException
	default:
		return config.TradeStatusWaitBuyerPay
	}
}

// MapRefundStatus 映射 2Checkout 退款状态
func MapRefundStatus(status string) string {
	switch strings.ToLower(status) {
	case "pending":
		return config.TradeStatusProcessing
	case "completed", "approved":
		return config.TradeStatusSuccess
	case "rejected", "failed":
		return config.TradeStatusException
	case "cancelled":
		return config.TradeStatusClosed
	default:
		return config.TradeStatusProcessing
	}
}

// VerifySign 验证签名(IPN签名在PayNotify中已处理)
func (a *Twoco) VerifySign(ctx echo.Context) error {
	// 2Checkout IPN 签名在 PayNotify 中已处理
	// 这里做基本检查
	saleID := ctx.Form("sale_id")
	if len(saleID) == 0 {
		return fmt.Errorf("missing sale_id")
	}
	// MD5 签名验证
	if !a.verifyIPNSignature(ctx) {
		return ctx.String("Invalid signature")
	}
	return nil
}
