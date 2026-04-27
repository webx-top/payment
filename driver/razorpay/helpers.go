package razorpay

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"strings"

	"github.com/webx-top/echo"
	"github.com/webx-top/payment"
	"github.com/webx-top/payment/config"
)

// AmountToPaise 金额转最小单位(Razorpay要求整数，如100=INR 1.00)
// 对于 INR 等货币，单位是 paisa (100 paisa = 1 rupee)
// 注意：Razorpay 对所有货币都使用最小单位（分/派沙）
func AmountToPaise(amount float64) int64 {
	return int64(payment.Round(amount*100, 0))
}

// PaiseToAmount 最小单位转金额
func PaiseToAmount(paise int64) float64 {
	return float64(paise) / 100
}

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

// MappingStatus 映射 Razorpay 支付状态
// 状态文档: https://razorpay.com/docs/api/payments/#payment-entity
func MappingStatus(status string) string {
	switch status {
	case "created":
		return config.TradeStatusWaitBuyerPay
	case "authorized":
		return config.TradeStatusProcessing
	case "captured":
		return config.TradeStatusSuccess
	case "refunded":
		return config.TradeStatusFinished
	case "failed":
		return config.TradeStatusException
	default:
		return config.TradeStatusWaitBuyerPay
	}
}

// MapRefundStatus 映射 Razorpay 退款状态
// 状态文档: https://razorpay.com/docs/api/refunds/#refund-entity
func MapRefundStatus(status string) string {
	switch status {
	case "created":
		return config.TradeStatusProcessing
	case "issued":
		return config.TradeStatusProcessing
	case "processed":
		return config.TradeStatusSuccess
	case "failed":
		return config.TradeStatusException
	default:
		return config.TradeStatusProcessing
	}
}

// VerifySign 验证 Razorpay Webhook 签名(HMAC-SHA256)
// 文档: https://razorpay.com/docs/signature-verification/
// Razorpay 发送: X-Razorpay-Signature = HMAC-SHA256(raw_body, api_secret)
func (a *Razorpay) VerifySign(ctx echo.Context) error {
	signature := ctx.Request().Header().Get("X-Razorpay-Signature")
	if len(signature) == 0 {
		return fmt.Errorf("missing X-Razorpay-Signature header")
	}

	body, err := io.ReadAll(ctx.Request().Body())
	if err != nil {
		return fmt.Errorf("failed to read request body: %v", err)
	}
	// 重置 body，以便后续 ctx.MustBind 可再次读取
	ctx.Request().SetBody(strings.NewReader(string(body)))

	expected := generateSignature(string(body), a.apiSecret)
	if !hmac.Equal([]byte(expected), []byte(signature)) {
		return fmt.Errorf("invalid signature")
	}
	return nil
}

// generateSignature 生成 Razorpay HMAC-SHA256 签名
func generateSignature(payload, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(payload))
	return hex.EncodeToString(mac.Sum(nil))
}
