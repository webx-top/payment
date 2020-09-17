package config

import "github.com/webx-top/echo"

// Query 查询参数
type Query struct {
	Platform   string //付款平台
	TradeNo    string //付款平台的交易号
	OutTradeNo string //业务方的交易号（我们的订单号）
	Options    echo.H //其它选项
}
