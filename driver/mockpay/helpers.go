package mockpay

import (
	"net/url"
	"slices"
	"sort"
	"strings"

	"github.com/admpub/log"
	"github.com/admpub/resty/v2"
	"github.com/webx-top/com"
	"github.com/webx-top/echo"
	"github.com/webx-top/payment/config"
	"github.com/webx-top/restyclient"
)

func (a *Mockpay) callbackClient() *resty.Request {
	return restyclient.Retryable()
}

func (a *Mockpay) VerifySign(ctx echo.Context) error {
	log.Infof(`[Mockpay] VerifySign Form Data: %s`, com.Dump(ctx.Forms(), false))
	return config.ErrUnsupported
}

// name: queryStatus / noticeStatus / supportDevices / noticeDelay / disableFeatures
func (a *Mockpay) getOptionValue(name string, cfg *config.Pay) string {
	var optionValue string
	if a.account.Options.Extra != nil {
		optionValue = a.account.Options.Extra.String(name)
	}
	if len(optionValue) == 0 && cfg != nil && cfg.Options != nil {
		optionValue = cfg.Options.String(name)
	}
	return optionValue
}

func (a *Mockpay) getFeatures() config.Supports {
	if a.features != nil {
		return a.features
	}
	v := a.getOptionValue(`disableFeatures`, nil)
	if len(v) == 0 {
		return supports
	}
	disableFeatures := strings.Split(v, `,`)
	features := config.Supports{}
	for _, feature := range supports {
		if slices.Contains(disableFeatures, feature.String()) {
			continue
		}
		features = append(features, feature)
	}
	a.features = features
	return features
}

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

type ManualNotice interface {
	SubmitPayNotice(tradeNo string) error
	SubmitRefundNotice(refundNo string) error
}
