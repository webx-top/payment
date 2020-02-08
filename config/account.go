package config

import (
	"github.com/webx-top/echo"
)

func NewAccount() *Account {
	return &Account{}
}

type Options struct {
	IconClass string `json:"iconClass"`
	IconImage string `json:"iconImage"`
	Title     string `json:"title"`
	Name      string `json:"name"`
}

type Account struct {
	Debug      bool    `json:"debug"`
	AppID      string  `json:"appID"`      //即AppID
	AppSecret  string  `json:"appSecret"`  //即AppKey
	MerchantID string  `json:"merchantID"` //商家ID
	PublicKey  string  `json:"publicKey"`  //公钥
	PrivateKey string  `json:"privateKey"` //私钥
	CertPath   string  `json:"certPath"`   //证书路径
	Options    Options `json:"options"`    //其它选项
}

func (c *Account) FromStore(v echo.Store) *Account {
	c.Debug = v.Bool(`debug`)
	c.AppID = v.String(`appID`)
	c.AppSecret = v.String(`appSecret`)
	c.MerchantID = v.String(`merchantID`)
	c.PublicKey = v.String(`publicKey`)
	c.PrivateKey = v.String(`privateKey`)
	c.CertPath = v.String(`certPath`)
	options := v.Store(`options`)
	c.Options.IconClass = options.String(`iconClass`)
	c.Options.IconImage = options.String(`iconImage`)
	c.Options.Title = options.String(`title`)
	c.Options.Name = options.String(`name`)
	return c
}
