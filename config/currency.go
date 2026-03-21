package config

// Currency 币种
type Currency string

func (c Currency) String() string {
	if len(c) == 0 {
		return `CNY`
	}
	return string(c)
}

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
