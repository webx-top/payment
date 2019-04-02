package config

import (
	"github.com/webx-top/echo"
)

type Account struct {
	Debug        bool
	ClientID     string //即AppID
	ClientSecret string //即AppKey
	MerchantID   string //商家ID
	PublicKey    string //公钥
	PrivateKey   string //私钥
	CertPath     string //证书路径
	Options      echo.H //其它选项
}
