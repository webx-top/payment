package epusdt

import (
	"net/url"
	"sort"
	"strings"

	"github.com/webx-top/com"
	"github.com/webx-top/echo"
	"github.com/webx-top/payment/config"
)

// MappingStatus Order Status
func MappingStatus(status int, result *config.Result) {
	switch status {
	case StatusPaid: // 付款已确认并已计入商家账户
		result.Status = config.TradeStatusSuccess

	case StatusWaitPay: //新创造的。尚未选择付款货币
		result.Status = config.TradeStatusWaitBuyerPay

	case StatusExpired: // 客户在规定时间内没有支付，支付信息过期，需要重新提交新的支付订单
		result.Status = config.TradeStatusClosed
	}
}

func (a *EPUSDT) VerifySign(ctx echo.Context) error {
	return config.ErrUnsupported
}

func GenerateSign(data url.Values, token string) string {
	names := make([]string, len(data))
	var i int
	for name, values := range data {
		if name != `signature` && len(values) > 0 && len(values[0]) > 0 {
			names[i] = name
			i++
		}
	}
	sort.Strings(names)
	sortedData := make([]string, len(names))
	for i, v := range names {
		sortedData[i] = v + `=` + data.Get(v)
	}
	return com.Md5(strings.Join(sortedData, `&`) + token)
}
