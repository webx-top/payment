package mugglepay

import (
	"github.com/webx-top/echo"
	"github.com/webx-top/payment/config"
)

// MappingStatus Order Status https://github.com/MugglePay/MugglePay/blob/master/API/basic/OrderStatus.md
func MappingStatus(status string, result *config.Result) {
	switch status {
	case `PAID`: // 付款已确认并已计入商家账户
		result.Status = config.TradeStatusSuccess

	case `NEW`: //新创造的。尚未选择付款货币
		result.Status = config.TradeStatusWaitBuyerPay

	case `PENDING`: // 该事务已被检测到，并等待区块链确认。确认时间根据不同的币而有差异;在STAB1网络上，比特币10分钟，ETH 1分钟和EOS 3秒。
		result.Status = config.TradeStatusProcessing

	case `UNRESOLVED`: // 该交易已得到确认，但付款与预期的渠道分歧。它可能是过度付出的，欠款或延迟。
		result.Status = config.TradeStatusProcessing

	case `RESOLVED`: // 商家标记为已付款
		result.Status = config.TradeStatusSuccess

	case `EXPIRED`: // 客户在规定时间内没有支付，支付信息过期，需要重新提交新的支付订单
		result.Status = config.TradeStatusClosed

	case `CANCELED`: // 买家取消交易
		result.Status = config.TradeStatusClosed

	// 退款
	case `REFUND_PENDING`: // 已提交退款，待确认
		result.Status = config.TradeStatusProcessing
		result.Operation = config.OperationRefund
	case `REFUNDED`: // 已退款
		result.Status = config.TradeStatusSuccess
		result.Operation = config.OperationRefund
	}
}

func (a *Mugglepay) VerifySign(ctx echo.Context) error {
	return config.ErrUnsupported
}
