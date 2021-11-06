package xunhupay

import (
	"net/url"
	"sort"
	"strings"

	"github.com/webx-top/com"
	"github.com/webx-top/echo"
	"github.com/webx-top/payment/config"
)

func GenerateHash(data url.Values, secret string) string {
	data.Del(`hash`)
	keys := make([]string, 0, len(data))
	for key, val := range data {
		if len(val) == 0 || (len(val) == 1 && len(val[0]) == 0) {
			continue
		}
		keys = append(keys, key)
	}
	sort.Strings(keys)
	kv := make([]string, len(keys))
	for idx, key := range keys {
		kv[idx] = key + `=` + data.Get(key)
	}
	return com.Md5(strings.Join(kv, `&`) + secret)
}

func (a *XunHuPay) VerifySign(ctx echo.Context) error {
	return config.ErrUnsupported
}
