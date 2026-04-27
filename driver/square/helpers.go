package square

import (
	"crypto/hmac"
	"crypto/sha256"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/admpub/log"
	"github.com/webx-top/echo"
	"github.com/webx-top/payment"
	"github.com/webx-top/payment/config"
)

// AmountToCentInt 金额转分为整数(Square要求整数分单位)
func AmountToCentInt(amount float64) int64 {
	return int64(payment.Round(amount*100, 0))
}

// CentToAmount 分转金额
func CentToAmount(cent int64) float64 {
	return float64(cent) / 100
}

// MoneyFeeToString 金额转字符串
func MoneyFeeToString(moneyFee float64) string {
	return payment.RoundFloat(moneyFee, 2)
}

// MappingStatus 映射 Square 支付状态
// 状态文档: https://developer.square.com/reference/square/enums/PaymentStatus
func MappingStatus(status string) string {
	switch status {
	case "APPROVED":
		return config.TradeStatusSuccess
	case "COMPLETED":
		return config.TradeStatusSuccess
	case "CANCELED":
		return config.TradeStatusClosed
	case "FAILED":
		return config.TradeStatusException
	default:
		return config.TradeStatusWaitBuyerPay
	}
}

// MapRefundStatus 映射 Square 退款状态
// 状态文档: https://developer.square.com/reference/square/enums/RefundStatus
func MapRefundStatus(status string) string {
	switch status {
	case "COMPLETED":
		return config.TradeStatusSuccess
	case "PENDING":
		return config.TradeStatusProcessing
	case "REJECTED":
		return config.TradeStatusException
	case "FAILED":
		return config.TradeStatusException
	default:
		return config.TradeStatusProcessing
	}
}

// refundReasonString 将退款原因转为 Square 格式
func refundReasonString(reason string) string {
	switch reason {
	case "duplicate":
		return "DUPLICATE"
	case "fraudulent":
		return "FRAUDULENT"
	default:
		return "CUSTOMER_REQUEST"
	}
}

// VerifySign 验证 Square Webhook 签名(HMAC-SHA256)
// 文档: https://developer.squareup.com/docs/webhooks/overview#step-2-verify-the-webhook-event-signature
// Square 发送: X-Square-Signature = HMAC-SHA256(notification_url + body, webhook_signature_key)
func (a *Square) VerifySign(ctx echo.Context) error {
	signature := ctx.Request().Header().Get("X-Square-Signature")
	if len(signature) == 0 {
		return fmt.Errorf("missing X-Square-Signature header")
	}

	timestamp := ctx.Request().Header().Get("X-Square-Timestamp")
	if len(timestamp) == 0 {
		return fmt.Errorf("missing X-Square-Timestamp header")
	}

	ts, err := strconv.ParseInt(timestamp, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid timestamp: %v", err)
	}
	now := time.Now().Unix()
	if now-ts > 300 || ts-now > 300 {
		return fmt.Errorf("timestamp out of range (diff > 5min)")
	}

	signatureKey := a.account.WebhookID
	if len(signatureKey) == 0 {
		log.Warn("Square webhook: WebhookID (signature key) not configured, skipping verification")
		return nil
	}

	body, err := io.ReadAll(ctx.Request().Body())
	if err != nil {
		return fmt.Errorf("failed to read request body: %v", err)
	}
	// 重置 body，以便后续 ctx.MustBind 可再次读取
	ctx.Request().SetBody(strings.NewReader(string(body)))

	// signed_payload = notification_url + raw_body
	notificationURL := ctx.FullRequestURI()
	signedPayload := notificationURL + string(body)
	mac := hmac.New(sha256.New, []byte(signatureKey))
	mac.Write([]byte(signedPayload))
	expectedHash := fmt.Sprintf("%x", mac.Sum(nil))

	if !hmac.Equal([]byte(signature), []byte(expectedHash)) {
		return fmt.Errorf("signature mismatch")
	}
	return nil
}
