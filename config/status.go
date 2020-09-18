package config

import "github.com/webx-top/echo"

var EmptyTradeStatus = TradeStatus{}

func NewTradeStatus(status string, extras ...echo.H) TradeStatus {
	var extra echo.H
	if len(extras) > 0 {
		extra = extras[0]
	}
	return TradeStatus{
		Status: status,
		Extra:  extra,
	}
}

// TradeStatus 交易状态
type TradeStatus struct {
	Status string
	Extra  echo.H
}

func (t TradeStatus) IsSuccess() bool {
	return t.Status == TradeStatusSuccess
}

func (t TradeStatus) IsWaitPay() bool {
	return t.Status == TradeStatusWaitBuyerPay
}

func (t TradeStatus) IsClosed() bool {
	return t.Status == TradeStatusClosed
}

func (t TradeStatus) IsFinished() bool {
	return t.Status == TradeStatusFinished
}

// IsProcessing 是否退款中
func (t TradeStatus) IsProcessing() bool {
	return t.Status == TradeStatusProcessing
}

const (
	// TradeStatusWaitBuyerPay 交易创建，等待买家付款
	TradeStatusWaitBuyerPay = "WAIT_BUYER_PAY"
	// TradeStatusClosed 未付款交易超时关闭，或支付完成后全额退款
	TradeStatusClosed = "TRADE_CLOSED"
	// TradeStatusSuccess 交易支付成功
	TradeStatusSuccess = "TRADE_SUCCESS"
	// TradeStatusFinished 交易结束，不可退款
	TradeStatusFinished = "TRADE_FINISHED"
	// TradeStatusException 交易异常(用于退款)
	TradeStatusException = "TRADE_EXCEPTION"
	// TradeStatusProcessing 交易中(用于退款)
	TradeStatusProcessing = "TRADE_PROCESSING"
)
