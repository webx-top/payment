package config

import "github.com/webx-top/echo"

// Query 查询参数
type Query struct {
	Platform    string //付款平台
	TradeNo     string //付款平台的交易号
	OutTradeNo  string //业务方的交易号（我们的订单号）
	RefundNo    string //付款平台的退款单号 (退款查询时有效)
	OutRefundNo string //业务方的退款单号 (退款查询时有效)
	Options     echo.H //其它选项
}

func NewQuery() *Query {
	return &Query{}
}

func (q *Query) CopyFromRefund(f *Refund) *Query {
	q.Platform = f.Platform
	q.TradeNo = f.TradeNo
	q.OutTradeNo = f.OutTradeNo
	q.RefundNo = f.RefundNo
	q.OutRefundNo = f.OutRefundNo
	q.Options = f.Options
	return q
}
