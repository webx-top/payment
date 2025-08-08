package epusdt

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/admpub/log"
	"github.com/webx-top/com"
	"github.com/webx-top/echo"
	"github.com/webx-top/payment"
	"github.com/webx-top/payment/config"
	"github.com/webx-top/restyclient"
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

func SetDefaults(a *config.Account) {
	if a.Subtype == nil {
		a.Subtype = config.NewSubtype(
			`合约网络`,
		)
	}
	if len(a.Subtype.Options) == 0 {
		var currencies []string
		if len(a.Currencies) == 0 {
			currencies = []string{`USDT`}
		} else {
			currencies = a.Currencies
		}
		networks, err := queryNetworks(a)
		if err != nil {
			log.Error(err)
			return
		}
		length := len(currencies)
		needPrefix := length > 1 || a.Options.Title != currencies[0]
		for i, currency := range currencies {
			for j, network := range networks[currency] {
				stype := &config.SubtypeOption{
					Value:   network.Value,
					Text:    network.Label,
					Checked: i == 0 && j == 0,
				}
				if needPrefix {
					stype.Text = currency + ` • ` + network.Label
				}
				a.Subtype.Add(stype)
			}
		}
	}
}

func queryNetworks(account *config.Account) (map[string][]QueryNetworkResponse, error) {
	apiURL := strings.TrimSuffix(account.Options.Extra.String(`apiURL`), `/`)
	query := QueryNetworksRequest{
		Timestamp: time.Now().Unix(),
		Nonce:     payment.GenerateNonce(),
	}
	data := query.URLValues()
	query.Signature = GenerateSign(data, account.AppSecret)
	queryResult := map[string][]QueryNetworkResponse{}
	recv := &Response{
		Data: &queryResult,
	}
	resp, err := restyclient.Retryable().SetResult(recv).SetBody(query).Post(apiURL + URLQueryNet)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode() != http.StatusOK {
		return nil, fmt.Errorf("%s: %s", resp.Status(), com.StripTags(resp.String()))
	}
	if recv.StatusCode != http.StatusOK {
		return nil, errors.New(recv.Message)
	}
	return queryResult, err
}
