package payment

import (
	"github.com/webx-top/echo"
	"github.com/webx-top/payment/config"
)

// Hook 付款驱动接口
type Hook interface {
	SetNotifyCallback(callback func(echo.Context) error) Hook
	SetAccount(*config.Account) Hook
	Pay(echo.Context, *config.Pay) (*config.PayResponse, error)
	PayQuery(echo.Context, *config.Query) (*config.Result, error)
	PayNotify(echo.Context) error
	Refund(echo.Context, *config.Refund) (*config.Result, error)
	RefundQuery(echo.Context, *config.Query) (*config.Result, error)
	RefundNotify(echo.Context) error
	VerifySign(echo.Context) error
}

var (
	platforms = map[string]string{} //platform=>name: alipay=>支付宝
	payments  = map[string]func() Hook{}
)

func Platforms() map[string]string {
	return platforms
}

func Name(platform string) string {
	name, _ := platforms[platform]
	return name
}

func Register(platform string, name string, hook func() Hook, setDefaults ...func(*config.Account)) {
	payments[platform] = hook
	platforms[platform] = name
	if len(setDefaults) > 0 {
		config.RegisterAccountSetDefaults(platform, setDefaults[0])
	}
}

func Get(platform string) (hook func() Hook) {
	hook, _ = payments[platform]
	return hook
}
