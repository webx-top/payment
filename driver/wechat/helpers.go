package wechat

import (
	"encoding/xml"
	"io/ioutil"
	"strconv"
	"strings"
	"time"

	"github.com/objcoding/wxpay"
	"github.com/webx-top/echo"
	"github.com/webx-top/payment"
	"github.com/webx-top/payment/config"
)

// MoneyFeeToString 微信金额浮点转字符串
func MoneyFeeToString(moneyFee float64) string {
	return payment.MulFloat(moneyFee, 100, 0)
}

func XmlToMap(xmlStr string) wxpay.Params {

	params := make(wxpay.Params)
	decoder := xml.NewDecoder(strings.NewReader(xmlStr))

	var (
		key   string
		value string
	)

	for t, err := decoder.Token(); err == nil; t, err = decoder.Token() {
		switch token := t.(type) {
		case xml.StartElement: // 开始标签
			key = token.Name.Local
		case xml.CharData: // 标签内容
			content := string([]byte(token))
			value = content
		}
		if key != "xml" && key != "root" {
			if value != "\n" {
				params.SetString(key, value)
			}
		}
	}

	return params
}

func (a *Wechat) VerifySign(ctx echo.Context) error {
	body := ctx.Request().Body()
	defer body.Close()
	b, err := ioutil.ReadAll(body)
	if err != nil {
		return err
	}
	params := wxpay.XmlToMap(string(b))
	if !a.Client().ValidSign(params) {
		return config.ErrSignature
	}
	return nil
}

func (a *Wechat) translateWxpayAppResult(tradePay *config.Pay, params wxpay.Params) map[string]string {
	params[`reason`], _ = params[`return_msg`]
	if tradePay.Device == config.App {
		p := make(wxpay.Params)
		p["appid"] = params["appid"]
		p["partnerid"] = params["mch_id"]
		p["noncestr"] = params["nonce_str"]
		p["prepayid"] = params["prepay_id"]
		p["timestamp"] = strconv.FormatInt(time.Now().Unix(), 10)
		p["package"] = "Sign=WXPay"
		p["sign"] = a.Client().Sign(p)
		return map[string]string(p)
	}
	return map[string]string(params)
}
