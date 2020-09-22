package config

import "strconv"

type (
	// Device 设备类型
	Device string
	// GoodsType 商品类型
	GoodsType int
	// Currency 币种
	Currency string
)

func (a Device) String() string {
	return string(a)
}

func (a Device) IsSupported() bool {
	for _, v := range devices {
		if a == v {
			return true
		}
	}
	return false
}

func (a GoodsType) String() string {
	return strconv.FormatInt(int64(a), 10)
}

func (a GoodsType) Name() string {
	switch a {
	case VirtualGoods:
		return "VirtualGoods"
	case PhysicalGoods:
		return "PhysicalGoods"
	default:
		return ""
	}
}

func (c Currency) String() string {
	if len(c) == 0 {
		return `CNY`
	}
	return string(c)
}

const (
	// App 在App支付
	App Device = `app`
	// Web 在电脑端网页上支付
	Web Device = `web`
	// Wap 在手机端网页上支付
	Wap Device = `wap`
)

var (
	devices = []Device{App, Web, Wap}
)

func DeviceList() []Device {
	return devices
}

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
