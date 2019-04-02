package payment

import (
	"sync"

	"github.com/webx-top/echo"
	"github.com/webx-top/echo/param"
	"github.com/webx-top/payment/config"
)

type Hook interface {
	SetAccount(*config.Account) Hook
	Pay(*config.Pay) (param.StringMap, error)
	Notify(echo.Context) (param.StringMap, error)
	Refund(*config.Refund) (param.StringMap, error)
}

var (
	mutex    = &sync.RWMutex{}
	payments = map[config.Platform]Hook{}
)

func Register(name config.Platform, hook Hook) {
	mutex.Lock()
	defer mutex.Unlock()
	payments[name] = hook
}

func Get(name config.Platform) (hook Hook) {
	mutex.RLock()
	defer mutex.RUnlock()
	hook, _ = payments[name]
	return hook
}
