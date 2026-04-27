package authorize

import (
	"fmt"

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

// MappingStatus 映射 Authorize.Net Response Code
// 状态文档: https://developer.authorize.com/api/reference/response-codes.html
func MappingStatus(responseCode string) string {
	switch responseCode {
	case "1":
		return config.TradeStatusSuccess
	case "2":
		return config.TradeStatusClosed
	case "3":
		return config.TradeStatusException
	case "4":
		return config.TradeStatusProcessing
	default:
		return config.TradeStatusWaitBuyerPay
	}
}

// MapRefundStatus 映射退款响应码
func MapRefundStatus(responseCode string) string {
	switch responseCode {
	case "1":
		return config.TradeStatusSuccess
	case "2", "3":
		return config.TradeStatusException
	case "4":
		return config.TradeStatusProcessing
	default:
		return config.TradeStatusProcessing
	}
}

// VerifySign 验证签名(MD5 Hash方式已在PayNotify中处理)
func (a *Authorize) VerifySign(ctx echo.Context) error {
	// Authorize.Net 使用 MD5 Hash 验证，在 PayNotify 中已处理
	// 这里做基本检查
	transID := ctx.Form("x_trans_id")
	if len(transID) == 0 {
		return fmt.Errorf("missing transaction id")
	}
	return nil
}
