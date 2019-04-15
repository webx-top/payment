package config

import "github.com/webx-top/echo"

type Refund struct {
	Platform     string
	TradeNo      string  //商户订单号
	RefundNo     string  //商户退单号（aliapy可不传）
	TotalAmount  float64 //订单总金额（alipay可不传）
	RefundAmount float64 //退款金额
	RefundReason string  //退款原因（选填）
	Options      echo.H  //其它选项
}
