package config

import "github.com/webx-top/com"

// AccountLite Account脱敏后的结构体
type AccountLite struct {
	Debug      bool     `json:"debug"`                //是否debug环境（如果支持沙箱环境则自动采用沙箱环境）
	Currencies []string `json:"currencies,omitempty"` //支持的币种
	Subtype    *Subtype `json:"subtype,omitempty"`    //子类型（用于选择第四方平台内支持的支付方式）
	Sort       int      `json:"sort"`                 //排序编号
	Options    Options  `json:"options,omitempty"`    //其它选项
}

func (c *AccountLite) HasCurrency(currencies ...string) bool {
	for _, currency := range currencies {
		if !com.InSlice(currency, c.Currencies) {
			return false
		}
	}
	return true
}
