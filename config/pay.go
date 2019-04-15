package config

import "github.com/webx-top/echo"

type Device int

const (
	_ Device = iota
	App
	Web
)

type Pay struct {
	Platform  string
	Device    Device
	NotifyURL string
	Subject   string
	TradeNo   string
	Amount    float64
	Options   echo.H //其它选项
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
