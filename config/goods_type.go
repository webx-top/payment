package config

import "strconv"

// GoodsType 商品类型
type GoodsType int

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

const (
	// VirtualGoods 虚拟商品
	VirtualGoods GoodsType = iota
	// PhysicalGoods 实物类商品
	PhysicalGoods
)
