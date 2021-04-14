package config

import (
	"strings"

	"github.com/webx-top/echo"
)

func NewAccount() *Account {
	return &Account{}
}

type Options struct {
	IconClass string `json:"iconClass,omitempty"`
	IconImage string `json:"iconImage,omitempty"`
	Title     string `json:"title,omitempty"`
	Name      string `json:"name,omitempty"`
	Extra     echo.H `json:"extra,omitempty"`
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
	Options    Options  `json:"options,omitempty"`    //其它选项
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
	options := v.GetStore(`options`)
	c.Options.IconClass = options.String(`iconClass`)
	c.Options.IconImage = options.String(`iconImage`)
	c.Options.Title = options.String(`title`)
	c.Options.Name = options.String(`name`)
	return c
}
