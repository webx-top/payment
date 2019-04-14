package payment

import (
	"sync"
	"sort"

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
	mutex    = &sync.RWMutex{}
	payments = map[config.Platform]func() Hook{}
)

func Platforms()[]string{
	r:=[]string{}
	for platform:=range payments{
		r=append(r,string(platform))
	}
	sort.Strings(r) 
	return r
}

func Register(name config.Platform, hook func() Hook) {
	mutex.Lock()
	defer mutex.Unlock()
	payments[name] = hook
}

func Get(name config.Platform) (hook func() Hook) {
	mutex.RLock()
	defer mutex.RUnlock()
	hook, _ = payments[name]
	return hook
}
