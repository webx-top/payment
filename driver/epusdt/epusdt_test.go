package epusdt

import (
	"os"
	"testing"
	"time"

	"github.com/webx-top/com"
	"github.com/webx-top/echo/defaults"
	"github.com/webx-top/echo/testing/test"
	"github.com/webx-top/payment/config"
)

var (
	orderID = time.Now().Format(`20060102150405.000000`)
	tradeID string
)

func TestPay(t *testing.T) {
	h := New()
	c := config.NewAccount()
	c.AppSecret = os.Getenv(`EPUSDT_API_TOKEN`)
	apiURL := os.Getenv(`EPUSDT_API_URL`)
	c.Options.Extra.Set(`apiURL`, apiURL)
	h.SetAccount(c)
	ctx := defaults.NewMockContext()
	resp, err := h.Pay(ctx, &config.Pay{
		Platform:   Name,
		OutTradeNo: orderID,
		Amount:     100,
		NotifyURL:  apiURL + `/test/notify`,
		ReturnURL:  apiURL + `/test/return`,
	})
	test.Eq(t, nil, err)
	com.Dump(resp)
	tradeID = resp.TradeNo
	actual := ``
	expected := ``
	test.Eq(t, expected, actual)
}

func TestQueryPay(t *testing.T) {
	h := New()
	c := config.NewAccount()
	c.AppSecret = os.Getenv(`EPUSDT_API_TOKEN`)
	apiURL := os.Getenv(`EPUSDT_API_URL`)
	c.Options.Extra.Set(`apiURL`, apiURL)
	h.SetAccount(c)
	ctx := defaults.NewMockContext()
	resp, err := h.PayQuery(ctx, &config.Query{
		Platform:   Name,
		OutTradeNo: orderID,
		TradeNo:    tradeID,
	})
	test.Eq(t, nil, err)
	com.Dump(resp)
	actual := ``
	expected := ``
	test.Eq(t, expected, actual)
}
