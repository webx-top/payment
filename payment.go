package payment

import (
	"github.com/webx-top/echo"
	"github.com/webx-top/payment/config"
)

// Driver 付款驱动接口
type Driver interface {
	IsSupported(config.Support) bool
	SetNotifyCallback(callback func(echo.Context) error) Driver
	SetAccount(*config.Account) Driver
	Pay(echo.Context, *config.Pay) (*config.PayResponse, error)
	PayQuery(echo.Context, *config.Query) (*config.Result, error)
	PayNotify(echo.Context) error //! *务必在内部验证签名*
	Refund(echo.Context, *config.Refund) (*config.Result, error)
	RefundQuery(echo.Context, *config.Query) (*config.Result, error)
	RefundNotify(echo.Context) error //! *务必在内部验证签名*
	VerifySign(echo.Context) error
}

var (
	platforms = map[string]string{} //platform=>name: alipay=>支付宝
	payments  = map[string]func() Driver{}
)

func Platforms() map[string]string {
	return platforms
}

func Name(platform string) string {
	name, _ := platforms[platform]
	return name
}

func Register(platform string, name string, hook func() Driver, setDefaults ...func(*config.Account)) {
	payments[platform] = hook
	platforms[platform] = name
	if len(setDefaults) > 0 {
		config.RegisterAccountSetDefaults(platform, setDefaults[0])
	}
}

func Unregister(platform string) {
	delete(payments, platform)
	delete(platforms, platform)
	config.UnregisterAccountSetDefaults(platform)
}

func Get(platform string) (driver func() Driver) {
	driver, _ = payments[platform]
	return driver
}
