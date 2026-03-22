package mockpay

import (
	"github.com/webx-top/echo"
	"github.com/webx-top/payment/config"
)

var supports = config.Supports{
	config.SupportPayNotify,
	config.SupportPayQuery,
	config.SupportRefund,
	config.SupportRefundNotify,
	config.SupportRefundQuery,
}

var DisableableFeatures = []map[string]string{
	{`id`: config.SupportPayQuery.String(), `text`: echo.T(`查询付款结果`)},
	{`id`: config.SupportPayNotify.String(), `text`: echo.T(`付款结果通知`)},
	{`id`: config.SupportRefund.String(), `text`: echo.T(`退款功能`)},
	{`id`: config.SupportRefundQuery.String(), `text`: echo.T(`查询退款结果`)},
	{`id`: config.SupportRefundNotify.String(), `text`: echo.T(`退款结果通知`)},
}

func GetDisableableFeatures(ctx echo.Context) []map[string]string {
	smaps := make([]map[string]string, len(DisableableFeatures))
	for index, smap := range DisableableFeatures {
		smaps[index] = map[string]string{
			`id`:   smap[`id`],
			`text`: ctx.T(smap[`text`]),
		}
	}
	return smaps
}
