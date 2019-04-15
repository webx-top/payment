package payment

import (
	"sync"

	"github.com/webx-top/echo"
	"github.com/webx-top/echo/param"
	"github.com/webx-top/payment/config"
)

type Hook interface {
	SetNotifyCallback(callback func(echo.Context) error) Hook
	SetAccount(*config.Account) Hook
	Pay(*config.Pay) (param.StringMap, error)
	Notify(echo.Context) error
	Refund(*config.Refund) (param.StringMap, error)
}

var (
	mutex     = &sync.RWMutex{}
	platforms = []echo.KV{}
	payments  = map[config.Platform]func() Hook{}
)

func Platforms() []echo.KV {
	return platforms
}

func Register(platform config.Platform, name string, hook func() Hook) {
	mutex.Lock()
	defer mutex.Unlock()
	payments[platform] = hook
	var exists bool
	for i, v := range platforms {
		if v.K == platform.String() {
			exists = true
			platforms[i].K = platform.String()
			platforms[i].V = name
			break
		}
	}
	if !exists {
		platforms = append(platforms, echo.KV{K: platform.String(), V: name})
	}
}

func Get(name config.Platform) (hook func() Hook) {
	mutex.RLock()
	defer mutex.RUnlock()
	hook, _ = payments[name]
	return hook
}
