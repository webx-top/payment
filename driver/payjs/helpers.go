package payjs

import (
	"net/url"
	"sort"
	"strings"

	"github.com/webx-top/com"
	"github.com/webx-top/echo"
	"github.com/webx-top/payment"
	"github.com/webx-top/payment/config"
)

// MoneyFeeToString 微信金额浮点转字符串
func MoneyFeeToString(moneyFee float64) string {
	return payment.MulFloat(moneyFee, 100, 0)
}

func (a *PayJS) VerifySign(ctx echo.Context) error {
	formData := url.Values(ctx.Forms())
	sign := formData.Get("sign")
	formData.Del("sign")

	var keys []string
	for k := range formData {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var pList []string
	for _, key := range keys {
		value := formData.Get(key)
		if len(value) > 0 {
			pList = append(pList, key+"="+value)
		}
	}
	src := strings.Join(pList, "&")
	src += "&key=" + a.account.AppSecret
	genSign := strings.ToUpper(com.Md5(src))
	if sign != genSign {
		return config.ErrSignature
	}
	return nil
}
