package config

import (
	"strconv"

	"github.com/webx-top/echo"
)

type (
	// Device 设备类型
	Device int
	// GoodsType 商品类型
	GoodsType int
	// Currency 币种
	Currency string
)

func (a GoodsType) String() string {
	return strconv.FormatInt(int64(a), 10)
}

func (c Currency) String() string {
	if len(c) == 0 {
		return `CNY`
	}
	return string(c)
}

const (
	// App 在App支付
	App = iota + 1
	// Web 在网页上支付
	Web
)

const (
	// VirtualGoods 虚拟商品
	VirtualGoods GoodsType = iota
	// PhysicalGoods 实物类商品
	PhysicalGoods
)

const (
	// USD 美元
	USD Currency = `USD`
	// CNY 人民币
	CNY Currency = `CNY`
	// RUB 俄罗斯卢布
	RUB Currency = `RUB`
	// EUR 欧元
	EUR Currency = `EUR`
	// GBP 英镑
	GBP Currency = `GBP`
	// HKD 港元
	HKD Currency = `HKD`
	// JPY 日元
	JPY Currency = `JPY`
	// KRW 韩元
	KRW Currency = `KRW`
	// AUD 澳元
	AUD Currency = `AUD`
	// CAD 加元
	CAD Currency = `CAD`
)

// Pay 付款参数
type Pay struct {
	Platform       string    //付款平台（alipay/wechat/paypal）
	Device         Device    //付款时的设备
	NotifyURL      string    //接收付款结果通知的网址
	ReturnURL      string    //支付操作后返回的网址
	Subject        string    //主题描述
	OutTradeNo     string    //业务方的交易号（我们的订单号）
	Amount         float64   //支付金额
	Currency       Currency  //币种
	GoodsType      GoodsType //商品类型
	PassbackParams string    //回传参数
	Options        echo.H    //其它选项
}

func (pay *Pay) DeviceType() string {
	switch pay.Device {
	case App:
		return "APP"
	case Web:
		return "WEB"
	default:
		return ""
	}
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
