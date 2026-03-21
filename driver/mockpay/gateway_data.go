package mockpay

import (
	"runtime"
	"sync"
	"time"

	"github.com/admpub/go-ttlmap"
	"github.com/webx-top/payment/config"
)

type GatewayPayData struct {
	TradeNo     string
	Currency    string
	TotalAmount float64
	Config      config.Pay
}

type GatewayRefundData struct {
	RefundNo    string
	TotalAmount float64
	RefundFee   float64
	Currency    string
	Config      config.Refund
}

var defaultMaxAge = time.Hour * 24
var cackedKeys = sync.Map{}
var cachedData = ttlmap.New(&ttlmap.Options{
	InitialCapacity: 100,
	OnWillExpire:    nil,
	OnWillEvict: func(key string, item ttlmap.Item) {
		cackedKeys.Delete(key)
	},
})

func init() {
	runtime.SetFinalizer(cachedData, func(t *ttlmap.Map) error {
		cachedData.Drain()
		return nil
	})
}

func getCachedPayData(key string) (data GatewayPayData, err error) {
	item, err := cachedData.Get(key)
	if err != nil {
		return
	}
	data = item.Value().(GatewayPayData)
	return
}

func getCachedRefundData(key string) (data GatewayRefundData, err error) {
	item, err := cachedData.Get(key)
	if err != nil {
		return
	}
	data = item.Value().(GatewayRefundData)
	return
}

func setCachedData(key string, val interface{}) error {
	cackedKeys.Store(key, struct{}{})
	return cachedData.Set(key, ttlmap.NewItem(val, ttlmap.WithTTL(defaultMaxAge)), nil)
}

func DeleteCachedKey(key string) {
	cachedData.Delete(key)
	cackedKeys.Delete(key)
}

func GetAllCachedKeys() []string {
	var keys []string
	cackedKeys.Range(func(key, value interface{}) bool {
		keys = append(keys, key.(string))
		return true
	})
	return keys
}
