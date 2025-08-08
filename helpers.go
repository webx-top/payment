package payment

import (
	"github.com/webx-top/com"
	"github.com/webx-top/echo/param"
)

// NotifyIsPay 是付款通知
func NotifyIsPay(result param.StringMap) bool {
	return result.String(`operation`) == `payment`
}

// NotifyIsRefund 是退款通知
func NotifyIsRefund(result param.StringMap) bool {
	return result.String(`operation`) == `refund`
}

// GenerateNonce 生成一次性随机字符串
func GenerateNonce() string {
	return com.RandomAlphanumeric(16)
}
