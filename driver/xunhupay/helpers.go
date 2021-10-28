package xunhupay

import (
	"net/url"
	"sort"
	"strings"

	"github.com/webx-top/com"
	"github.com/webx-top/echo"
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
	formData := url.Values(ctx.Forms())
	status := formData.Get(`status`)
	if status != `OD` {
		return ctx.String(status)
	}
	formHash := formData.Get(`hash`)
	hashString := GenerateHash(formData, a.account.AppSecret)
	if formHash != hashString {
		return ctx.String(`invalid signature`)
	}

	return ctx.String(`success`)
}
