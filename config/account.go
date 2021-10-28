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
	Disabled bool   `json:"disabled,omitempty"`
	Value    string `json:"value,omitempty"`
	Text     string `json:"text,omitempty"`
	Image    string `json:"image,omitempty"`
	Checked  bool   `json:"label,omitempty"`
}

func NewSubtype(name string, label string, options ...*SubtypeOption) *Subtype {
	return &Subtype{
		Name:    name,
		Label:   label,
		Options: options,
	}
}

type Subtype struct {
	Disabled bool             `json:"disabled,omitempty"`
	Name     string           `json:"name,omitempty"`
	Label    string           `json:"label,omitempty"`
	Options  []*SubtypeOption `json:"options,omitempty"`
}

func (s *Subtype) Add(o ...*SubtypeOption) *Subtype {
	s.Options = append(s.Options, o...)
	return s
}

// Account 付款平台账号参数
type Account struct {
	Debug      bool       `json:"debug"`
	AppID      string     `json:"appID,omitempty"`      //即AppID
	AppSecret  string     `json:"appSecret,omitempty"`  //即AppKey
	MerchantID string     `json:"merchantID,omitempty"` //商家ID
	PublicKey  string     `json:"publicKey,omitempty"`  //公钥
	PrivateKey string     `json:"privateKey,omitempty"` //私钥
	CertPath   string     `json:"certPath,omitempty"`   //证书路径
	WebhookID  string     `json:"webhookID,omitempty"`  //Paypal使用的webhook id
	Currencies []string   `json:"currencies,omitempty"` //支持的币种
	Subtypes   []*Subtype `json:"subtypes,omitempty"`   //子类型（用于选择第四方平台内支持的支付方式）
	Options    Options    `json:"options,omitempty"`    //其它选项
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

func (c *Account) AddSubtype(subtypes ...*Subtype) *Account {
	c.Subtypes = append(c.Subtypes, subtypes...)
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
	subtypes := v.Get(`subtypes`)
	switch rv := subtypes.(type) {
	case []*Subtype:
		c.Subtypes = rv
	case []interface{}:
		c.Subtypes = make([]*Subtype, len(rv))
		if len(rv) > 0 {
			if _, ok := rv[0].(*Subtype); !ok {
				b, err := json.Marshal(rv)
				if err == nil {
					err = json.Unmarshal(b, &c.Subtypes)
					if err != nil {
						log.Error(err)
					}
				}
			} else {
				for _k, _v := range rv {
					c.Subtypes[_k] = _v.(*Subtype)
				}
			}
		}
	case string:
		if len(rv) > 0 {
			err := json.Unmarshal(com.Str2bytes(rv), &c.Subtypes)
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
