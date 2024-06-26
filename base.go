package payment

import (
	"github.com/webx-top/echo"
	"github.com/webx-top/payment/config"
)

var _ Driver = New()

func New() *Base {
	return &Base{}
}

type Base struct {
	Account        *config.Account
	NotifyCallback func(echo.Context) error
}

func (a *Base) IsSupported(s config.Support) bool {
	return false
}

func (a *Base) SetNotifyCallback(callback func(echo.Context) error) Driver {
	a.NotifyCallback = callback
	return a
}

func (a *Base) SetAccount(account *config.Account) Driver {
	a.Account = account
	return a
}

func (a *Base) Pay(ctx echo.Context, cfg *config.Pay) (*config.PayResponse, error) {
	return nil, config.ErrUnsupported
}

// PayNotify 付款回调处理
// ! *务必在内部验证签名*
func (a *Base) PayNotify(ctx echo.Context) error {
	return config.ErrUnsupported
}

func (a *Base) PayQuery(ctx echo.Context, cfg *config.Query) (*config.Result, error) {
	return nil, config.ErrUnsupported
}

func (a *Base) Refund(ctx echo.Context, cfg *config.Refund) (*config.Result, error) {
	return nil, config.ErrUnsupported
}

// RefundNotify 退款回调处理
// ! *务必在内部验证签名*
func (a *Base) RefundNotify(ctx echo.Context) error {
	return config.ErrUnsupported
}

// RefundQuery 退款查询
func (a *Base) RefundQuery(ctx echo.Context, cfg *config.Query) (*config.Result, error) {
	return nil, config.ErrUnsupported
}

func (a *Base) VerifySign(ctx echo.Context) error {
	return config.ErrUnsupported
}
