package paddle

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"strconv"
	"strings"
	"time"

	"fmt"

	"github.com/admpub/log"
	"github.com/webx-top/echo"
	"github.com/webx-top/payment"
	"github.com/webx-top/payment/config"
)

// MoneyFeeToString 金额转字符串
func MoneyFeeToString(moneyFee float64) string {
	return payment.RoundFloat(moneyFee, 2)
}

func defaultString(val, def string) string {
	if len(val) == 0 { return def }
	return val
}

// MappingStatus 映射 Paddle 交易状态
// 状态文档: https://developer.paddle.com/reference/transaction-statuses
func MappingStatus(status string) string {
	switch strings.ToLower(status) {
	case "completed", "billed":
		return config.TradeStatusSuccess
	case "ready", "past_due", "due":
		return config.TradeStatusProcessing
	case "canceled", "archived":
		return config.TradeStatusClosed
	case "declined", "disputed":
		return config.TradeStatusException
	default:
		return config.TradeStatusWaitBuyerPay
	}
}

// MapRefundStatus 映射 Paddle 退款状态
func MapRefundStatus(status string) string {
	switch strings.ToLower(status) {
	case "completed", "approved":
		return config.TradeStatusSuccess
	case "pending", "processing":
		return config.TradeStatusProcessing
	case "declined", "rejected":
		return config.TradeStatusException
	default:
		return config.TradeStatusProcessing
	}
}

// VerifySign 验证 Paddle Webhook 签名
// 文档: https://developer.paddle.com/webhooks/verify-webhooks
// Paddle v2 使用 HMAC-SHA256 签名，格式: ts={timestamp};h1={hash}
// secret key 存储在 account.WebhookID 中
func (a *Paddle) VerifySign(ctx echo.Context) error {
	signature := ctx.Request().Header().Get("Paddle-Signature")
	if len(signature) == 0 {
		return fmt.Errorf("missing Paddle-Signature header")
	}

	timestamp := ctx.Request().Header().Get("Paddle-Timestamp")
	if len(timestamp) == 0 {
		return fmt.Errorf("missing Paddle-Timestamp header")
	}

	ts, err := strconv.ParseInt(timestamp, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid timestamp: %v", err)
	}

	now := time.Now().Unix()
	if now-ts > 300 || ts-now > 300 {
		return fmt.Errorf("timestamp out of range (diff > 5min)")
	}

	// 解析签名: ts={ts};h1={hash}
	var h1 string
	for _, part := range strings.Split(signature, ";") {
		part = strings.TrimSpace(part)
		if strings.HasPrefix(part, "h1=") {
			h1 = strings.TrimPrefix(part, "h1=")
		}
	}
	if len(h1) == 0 {
		return fmt.Errorf("invalid signature format, missing h1")
	}

	secretKey := a.account.WebhookID
	if len(secretKey) == 0 {
		log.Warn("Paddle webhook: WebhookID (signing secret) not configured, skipping verification")
		return nil
	}

	body, err := io.ReadAll(ctx.Request().Body())
	if err != nil {
		return fmt.Errorf("failed to read request body: %v", err)
	}
	// 重置 body，以便后续 ctx.MustBind 可再次读取
	ctx.Request().SetBody(strings.NewReader(string(body)))

	// signed_payload = "ts:" + timestamp + ":" + raw_body
	signedPayload := "ts:" + timestamp + ":" + string(body)
	mac := hmac.New(sha256.New, []byte(secretKey))
	mac.Write([]byte(signedPayload))
	expectedHash := hex.EncodeToString(mac.Sum(nil))

	if !hmac.Equal([]byte(h1), []byte(expectedHash)) {
		return fmt.Errorf("signature mismatch")
	}

	return nil
}
