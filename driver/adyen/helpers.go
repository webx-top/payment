package adyen

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"strings"

	"github.com/webx-top/echo"
	"github.com/webx-top/payment"
	"github.com/webx-top/payment/config"
)

// AmountToMinor 将金额转为最小单位(Adyen要求整数，如100=1.00 USD)
func AmountToMinor(amount float64, currency string) int64 {
	if isZeroDecimal(currency) {
		return int64(payment.Round(amount, 0))
	}
	return int64(payment.Round(amount*100, 0))
}

// MinorToAmount 最小单位转金额
func MinorToAmount(minor int64) float64 {
	return float64(minor) / 100
}

// MoneyFeeToString 金额转字符串
func MoneyFeeToString(moneyFee float64) string {
	return payment.RoundFloat(moneyFee, 2)
}

// isZeroDecimal 检查零小数位货币
func isZeroDecimal(currency string) bool {
	zeroDecimals := map[string]bool{
		"BIF": true, "CLP": true, "DJF": true, "GNF": true,
		"JPY": true, "KMF": true, "KRW": true, "MGA": true,
		"PYG": true, "RWF": true, "UGX": true, "VND": true,
		"VUV": true, "XAF": true, "XOF": true, "XPF": true,
	}
	return zeroDecimals[strings.ToUpper(currency)]
}

// MappingStatus 映射 Adyen ResultCode 到项目状态
// 文档: https://docs.adyen.com/online-payments/result-codes/
func MappingStatus(resultCode string) string {
	switch resultCode {
	case "Authorised", "Received":
		return config.TradeStatusSuccess
	case "Pending":
		return config.TradeStatusProcessing
	case "Cancelled", "Refused":
		return config.TradeStatusClosed
	case "Error":
		return config.TradeStatusException
	default:
		return config.TradeStatusWaitBuyerPay
	}
}

// MapRefundStatus 映射 Adyen Refund 结果状态
func MapRefundStatus(resultCode string) string {
	switch resultCode {
	case "Received", "[refund-received]":
		return config.TradeStatusProcessing
	case "Refunded", "[refund-successfully-processed]":
		return config.TradeStatusSuccess
	case "Rejected", "Error":
		return config.TradeStatusException
	default:
		return config.TradeStatusProcessing
	}
}

func defaultString(val, def string) string {
	if len(val) == 0 {
		return def
	}
	return val
}

// adyenNotification Adyen webhook 通知数据结构（VerifySign 和 PayNotify 共用）
type adyenNotification struct {
	EventCode         string            `json:"eventCode"`
	Success           bool              `json:"success"`
	PspReference      string            `json:"pspReference"`
	MerchantReference string            `json:"merchantReference"`
	Value             int64             `json:"value"`
	Currency          string            `json:"currency"`
	Reason            string            `json:"reason"`
	AdditionalData    map[string]string `json:"additionalData"`
}

// parseNotifications 解析 Adyen 通知列表（内部共用，避免重复 MustBind）
func (a *Adyen) parseNotifications(ctx echo.Context) ([]adyenNotification, error) {
	var notifications []adyenNotification
	if err := ctx.MustBind(&notifications); err != nil {
		return nil, fmt.Errorf("failed to parse notifications: %v", err)
	}
	return notifications, nil
}

// VerifySign 验证 Adyen Webhook 签名(HMAC-SHA256)
// 文档: https://docs.adyen.com/development-resources/webhooks/signature-generation-and-verification
func (a *Adyen) VerifySign(ctx echo.Context) error {
	if len(a.hmacKey) == 0 {
		return nil
	}
	notifications, err := a.parseNotifications(ctx)
	if err != nil {
		return err
	}
	return a.verifySign(notifications)
}

func (a *Adyen) verifySign(notifications []adyenNotification) error {
	if len(a.hmacKey) == 0 {
		return nil
	}

	for i, n := range notifications {
		receivedSig := n.AdditionalData["hmacSignature"]
		if len(receivedSig) == 0 {
			return fmt.Errorf("notification[%d]: missing hmacSignature", i)
		}

		expected := a.generateNotificationHMAC(n.EventCode, n.Success, n.PspReference,
			n.MerchantReference, n.Value, n.Currency)
		if !hmac.Equal([]byte(expected), []byte(receivedSig)) {
			return fmt.Errorf("notification[%d]: signature mismatch", i)
		}
	}
	return nil
}

// generateNotificationHMAC 计算单个通知的 HMAC 签名
func (a *Adyen) generateNotificationHMAC(eventCode string, success bool, pspReference string,
	merchantReference string, value int64, currency string) string {
	data := pspReference + ":" +
		":" + // originalReference (空)
		a.account.MerchantID + ":" +
		merchantReference + ":" +
		fmt.Sprintf("%d", value) + ":" +
		currency + ":" +
		eventCode + ":" +
		fmt.Sprintf("%t", success)

	keyBytes, _ := base64.StdEncoding.DecodeString(a.hmacKey)
	mac := hmac.New(sha256.New, keyBytes)
	mac.Write([]byte(data))
	return base64.StdEncoding.EncodeToString(mac.Sum(nil))
}
