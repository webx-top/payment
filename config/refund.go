package config

import "github.com/webx-top/echo"

// Refund 退款参数
type Refund struct {
	Platform     string   //付款平台
	TradeNo      string   //付款平台的交易号
	OutTradeNo   string   //业务方的交易号（我们的订单号）
	RefundNo     string   //付款平台退单号
	OutRefundNo  string   //业务方退单号
	TotalAmount  float64  //订单总金额（alipay可不传）
	RefundAmount float64  //退款金额
	RefundReason string   //退款原因（选填）
	Currency     Currency //币种
	NotifyURL    string   //接收退款结果通知的网址
	Options      echo.H   //其它选项
}
