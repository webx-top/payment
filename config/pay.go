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
)

func (a GoodsType) String() string {
	return strconv.FormatInt(int64(a), 10)
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

type Pay struct {
	Platform       string
	Device         Device
	NotifyURL      string
	ReturnURL      string
	Subject        string
	TradeNo        string
	Amount         float64
	GoodsType      GoodsType
	PassbackParams string // 回传参数
	Options        echo.H //其它选项
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
