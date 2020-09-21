package config

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
