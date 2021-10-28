package config

import (
	"encoding/json"
	"strings"

	"github.com/admpub/log"
	"github.com/webx-top/com"
	"github.com/webx-top/echo"
)

func NewAccount() *Account {
	return &Account{
		Options: Options{
			Extra: echo.H{},
		},
	}
}

type Options struct {
	IconClass string `json:"iconClass,omitempty"`
	IconImage string `json:"iconImage,omitempty"`
	Title     string `json:"title,omitempty"`
	Name      string `json:"name,omitempty"`
	Extra     echo.H `json:"extra,omitempty"`
}

type SubtypeOption struct {
	Disabled bool   `json:"disabled"`
	Value    string `json:"value"`
	Text     string `json:"text"`
	Image    string `json:"image"`
	Checked  bool   `json:"label"`
}

func NewSubtype(label string, options ...*SubtypeOption) *Subtype {
	return &Subtype{
		Label:   label,
		Options: options,
	}
}

type Subtype struct {
	Label   string           `json:"label"`
	Options []*SubtypeOption `json:"options"`
}

func (s *Subtype) Add(o ...*SubtypeOption) *Subtype {
	s.Options = append(s.Options, o...)
	return s
}

func (s *Subtype) Exists(value string) bool {
	for _, o := range s.Options {
		if o.Value == value {
			return true
		}
	}
	return false
}

func (s *Subtype) GetOption(value string) *SubtypeOption {
	for _, o := range s.Options {
		if o.Value == value {
			return o
		}
	}
	return nil
}

// Account 付款平台账号参数
type Account struct {
	Debug      bool     `json:"debug"`
	AppID      string   `json:"appID,omitempty"`      //即AppID
	AppSecret  string   `json:"appSecret,omitempty"`  //即AppKey
	MerchantID string   `json:"merchantID,omitempty"` //商家ID
	PublicKey  string   `json:"publicKey,omitempty"`  //公钥
	PrivateKey string   `json:"privateKey,omitempty"` //私钥
	CertPath   string   `json:"certPath,omitempty"`   //证书路径
	WebhookID  string   `json:"webhookID,omitempty"`  //Paypal使用的webhook id
	Currencies []string `json:"currencies,omitempty"` //支持的币种
	Subtype    *Subtype `json:"subtype,omitempty"`    //子类型（用于选择第四方平台内支持的支付方式）
	Options    Options  `json:"options,omitempty"`    //其它选项
}

var accountSetDefaults = map[string]func(a *Account){}

func RegisterAccountSetDefaults(platform string, fn func(a *Account)) {
	accountSetDefaults[platform] = fn
}

func UnregisterAccountSetDefaults(platform string) {
	delete(accountSetDefaults, platform)
}

func (c *Account) SetDefaults(platform string) *Account {
	fn, ok := accountSetDefaults[platform]
	if ok {
		fn(c)
	}
	return c
}

func (c *Account) FromStore(v echo.Store) *Account {
	c.Debug = v.Bool(`debug`)
	c.AppID = v.String(`appID`)
	c.AppSecret = v.String(`appSecret`)
	c.MerchantID = v.String(`merchantID`)
	c.PublicKey = v.String(`publicKey`)
	c.PrivateKey = v.String(`privateKey`)
	c.CertPath = v.String(`certPath`)
	c.WebhookID = v.String(`webhookID`)
	if currencies := v.String(`currencies`); len(currencies) > 0 {
		tmp := map[string]struct{}{}
		for _, currency := range strings.Split(currencies, ",") {
			currency = strings.TrimSpace(currency)
			if len(currency) == 0 {
				continue
			}
			if _, ok := tmp[currency]; ok {
				continue
			}
			c.Currencies = append(c.Currencies, currency)
			tmp[currency] = struct{}{}
		}
	}
	subtype := v.Get(`subtype`)
	switch rv := subtype.(type) {
	case *Subtype:
		c.Subtype = rv
	case Subtype:
		c.Subtype = &rv
	case string:
		if len(rv) > 0 {
			if c.Subtype == nil {
				c.Subtype = &Subtype{}
			}
			err := json.Unmarshal(com.Str2bytes(rv), &c.Subtype)
			if err != nil {
				log.Error(err)
			}
		}
	}
	options := v.GetStore(`options`)
	c.Options.IconClass = options.String(`iconClass`)
	c.Options.IconImage = options.String(`iconImage`)
	c.Options.Title = options.String(`title`)
	c.Options.Name = options.String(`name`)
	c.Options.Extra = options.GetStore(`extra`)
	return c
}
