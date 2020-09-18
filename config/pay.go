package config

import (
	"time"

	"github.com/webx-top/echo"
)

// Pay 付款参数
type Pay struct {
	Platform       string    //付款平台（alipay/wechat/paypal）
	Device         Device    //付款时的设备
	NotifyURL      string    //接收付款结果通知的网址
	ReturnURL      string    //支付操作后返回的网址
	CancelURL      string    //取消付款后返回的网址
	Subject        string    //主题描述
	OutTradeNo     string    //业务方的交易号（我们的订单号）
	Amount         float64   //支付金额
	Currency       Currency  //币种
	GoodsType      GoodsType //商品类型
	PassbackParams string    //回传参数
	ExpiredAt      time.Time //支付过期时间
	Options        echo.H    //其它选项
}

func (pay *Pay) GoodsTypeName() string {
	switch pay.GoodsType {
	case VirtualGoods:
		return "VirtualGoods"
	case PhysicalGoods:
		return "PhysicalGoods"
	default:
		return ""
	}
}
