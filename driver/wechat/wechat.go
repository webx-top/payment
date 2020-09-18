package wechat

import (
	"encoding/base64"
	"errors"
	"fmt"
	"io/ioutil"
	"strconv"
	"strings"

	"github.com/admpub/log"
	"github.com/objcoding/wxpay"
	"github.com/webx-top/codec"
	"github.com/webx-top/com"
	"github.com/webx-top/echo"
	"github.com/webx-top/echo/param"
	"github.com/webx-top/payment"
	"github.com/webx-top/payment/config"
)

const Name = `wechat`

func init() {
	payment.Register(Name, `微信支付`, New)
}

func New() payment.Hook {
	return &Wechat{}
}

type Wechat struct {
	account        *config.Account
	client         *wxpay.Client
	notifyCallback func(echo.Context) error
}

func (a *Wechat) SetNotifyCallback(callback func(echo.Context) error) payment.Hook {
	a.notifyCallback = callback
	return a
}

func (a *Wechat) SetAccount(account *config.Account) payment.Hook {
	a.account = account
	return a
}

func (a *Wechat) Client() *wxpay.Client {
	if a.client != nil {
		return a.client
	}
	wechatAccount := wxpay.NewAccount(
		a.account.AppID,
		a.account.MerchantID,
		a.account.AppSecret,
		a.account.Debug,
	)
	if len(a.account.CertPath) > 0 {
		wechatAccount.SetCertData(a.account.CertPath)
	}
	a.client = wxpay.NewClient(wechatAccount)
	return a.client
}

func (a *Wechat) Pay(ctx echo.Context, cfg *config.Pay) (param.StringMap, error) {
	var tradeType string
	switch cfg.Device {
	case config.Web:
		tradeType = `NATIVE`
	case config.Wap:
		tradeType = `MWEB`
	case config.App:
		tradeType = `APP`
	default:
		tradeType = `MWEB`
	}
	if strings.Contains(ctx.Request().UserAgent(), `MicroMessenger`) {
		tradeType = `JSAPI`
	}
	wxParams := wxpay.Params{
		"notify_url":   cfg.NotifyURL,
		"trade_type":   tradeType, //JSAPI:JSAPI支付（或小程序支付）、NATIVE:Native支付、APP:app支付，MWEB:H5支付
		"total_fee":    MoneyFeeToString(cfg.Amount),
		"out_trade_no": cfg.OutTradeNo,
		"body":         cfg.Subject,
		"scene_info":   ``,
	}
	if !cfg.ExpiredAt.IsZero() {
		wxParams[`time_expire`] = cfg.ExpiredAt.Format(`20060102150405`)
	}
	if cfg.Options != nil {
		params := cfg.Options.Store(`params`)
		for k := range params {
			wxParams[k] = params.String(k)
		}
	}
	// documentation https://pay.weixin.qq.com/wiki/doc/api/jsapi.php?chapter=9_1
	params, err := a.Client().UnifiedOrder(wxParams)
	if err != nil {
		return nil, err
	}
	return param.ToStringMap(a.translateWxpayAppResult(cfg, params)), nil
}

// PayNotify 付款回调处理
// documentation https://pay.weixin.qq.com/wiki/doc/api/jsapi.php?chapter=9_7&index=8
func (a *Wechat) PayNotify(ctx echo.Context) error {
	result := param.StringMap{}
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
	if params["return_code"] != "SUCCESS" {
		return config.ErrPaymentFailed
	}
	result = param.ToStringMap(params)
	totalFee := result.String(`total_fee`)
	cents, err := strconv.ParseInt(totalFee, 10, 64)
	if err != nil {
		return fmt.Errorf(`total_fee(%v): %v`, totalFee, err)
	}
	result[`total_amount`] = param.String(payment.CutFloat(float64(cents)/100, 2))
	result[`trade_no`], _ = result[`transaction_id`]
	result[`operation`] = `payment`
	var isSuccess = true
	var xmlString string
	noti := wxpay.Notifies{}
	if a.notifyCallback != nil {
		ctx.Set(`notify`, result)
		if err := a.notifyCallback(ctx); err != nil {
			log.Error(err)
			isSuccess = false
		}
	}
	if !isSuccess {
		xmlString = noti.NotOK("faild")
	} else {
		xmlString = noti.OK()
	}

	return ctx.XMLBlob([]byte(xmlString))
}

func (a *Wechat) PayQuery(ctx echo.Context, cfg *config.Query) (config.TradeStatus, error) {
	params := make(wxpay.Params)
	if len(cfg.TradeNo) > 0 {
		params.SetString("transaction_id", cfg.TradeNo)
	} else {
		params.SetString("out_trade_no", cfg.OutTradeNo)
	}
	// documentation https://pay.weixin.qq.com/wiki/doc/api/jsapi.php?chapter=9_2
	resp, err := a.Client().OrderQuery(params)
	if err != nil {
		return config.EmptyTradeStatus, err
	}
	if resp.GetString(`return_code`) != wxpay.Success {
		return config.EmptyTradeStatus, errors.New(resp.GetString(`return_msg`))
	}
	tradeStatus := resp.GetString(`trade_state`)
	/*
		SUCCESS—支付成功
		REFUND—转入退款
		NOTPAY—未支付
		CLOSED—已关闭
		REVOKED—已撤销（付款码支付）
		USERPAYING--用户支付中（付款码支付）
		PAYERROR--支付失败(其他原因，如银行返回失败)
	*/
	switch tradeStatus {
	case `SUCCESS`:
		tradeStatus = config.TradeStatusSuccess
	case `REFUND`, `CLOSED`:
		tradeStatus = config.TradeStatusClosed
	case `NOTPAY`, `REVOKED`, `USERPAYING`, `PAYERROR`:
		tradeStatus = config.TradeStatusWaitBuyerPay
	}
	return config.NewTradeStatus(tradeStatus, echo.H{
		`trade_no`:     resp.GetString(`transaction_id`),
		`out_trade_no`: resp.GetString(`out_trade_no`),
		`currency`:     resp.GetString(`fee_type`),
		`total_amount`: payment.CutFloat(float64(resp.GetInt64(`total_fee`))/100, 2),
	}), err
}

func (a *Wechat) Refund(ctx echo.Context, cfg *config.Refund) (param.StringMap, error) {
	refundConfig := wxpay.Params{
		"out_trade_no":  cfg.OutTradeNo,
		"out_refund_no": cfg.OutRefundNo,
		"total_fee":     MoneyFeeToString(cfg.TotalAmount),
		"refund_fee":    MoneyFeeToString(cfg.RefundAmount),
	}
	if len(cfg.NotifyURL) > 0 {
		refundConfig[`notify_url`] = cfg.NotifyURL
	}
	// documentation https://pay.weixin.qq.com/wiki/doc/api/jsapi.php?chapter=9_4
	resp, err := a.Client().Refund(refundConfig)
	if err != nil {
		return nil, err
	}
	returnCode := resp.GetString("return_code")
	resp[`success`] = ``
	if returnCode == wxpay.Fail {
		resp[`success`] = `0`
	} else if returnCode == wxpay.Success {
		resp[`success`] = `1`
	}
	resp[`refund_no`], _ = resp[`refund_id`]
	return param.ToStringMap(resp), err
}

// RefundNotify 退款回调处理
// documentation https://pay.weixin.qq.com/wiki/doc/api/jsapi.php?chapter=9_16&index=10
func (a *Wechat) RefundNotify(ctx echo.Context) error {
	result := param.StringMap{}
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
	if params["return_code"] != "SUCCESS" {
		return config.ErrRefundFailed
	}
	result = param.ToStringMap(params)
	totalFee := result.String(`total_fee`)
	cents, err := strconv.ParseInt(totalFee, 10, 64)
	if err != nil {
		return fmt.Errorf(`total_fee(%v): %v`, totalFee, err)
	}
	result[`total_amount`] = param.String(payment.CutFloat(float64(cents)/100, 2))
	result[`trade_no`], _ = result[`transaction_id`]
	refundFee := result.String(`refund_fee`)
	refundFeeCents, err := strconv.ParseInt(refundFee, 10, 64)
	if err != nil {
		return fmt.Errorf(`refund_fee(%v): %v`, refundFee, err)
	}
	result[`refund_fee`] = param.String(payment.CutFloat(float64(refundFeeCents)/100, 2))
	result[`operation`] = `refund`
	reqInfo := result.String(`req_info`)
	b, err = base64.StdEncoding.DecodeString(reqInfo)
	if err != nil {
		return fmt.Errorf(`base64decode(%v): %v`, reqInfo, err)
	}
	key := strings.ToLower(com.Md5(a.account.AppSecret))
	crypto := codec.NewAesECBCrypto(`AES-256`)
	b = crypto.DecodeBytes(b, []byte(key))
	for k, v := range XmlToMap(string(b)) {
		result[k] = param.String(v)
	}
	var isSuccess = true
	var xmlString string
	noti := wxpay.Notifies{}
	if a.notifyCallback != nil {
		ctx.Set(`notify`, result)
		if err := a.notifyCallback(ctx); err != nil {
			isSuccess = false
		}
	}
	if !isSuccess {
		xmlString = noti.NotOK("faild")
	} else {
		xmlString = noti.OK()
	}

	return ctx.XMLBlob([]byte(xmlString))
}

// RefundQuery 退款查询
func (a *Wechat) RefundQuery(ctx echo.Context, cfg *config.Query) (config.TradeStatus, error) {
	params := make(wxpay.Params)
	if len(cfg.OutRefundNo) > 0 {
		params.SetString("out_refund_no", cfg.OutRefundNo)
	} else if len(cfg.TradeNo) > 0 {
		params.SetString("transaction_id", cfg.TradeNo)
	} else {
		params.SetString("out_trade_no", cfg.OutTradeNo)
	}
	// documentation https://pay.weixin.qq.com/wiki/doc/api/jsapi.php?chapter=9_5
	resp, err := a.Client().RefundQuery(params)
	if err != nil {
		return config.EmptyTradeStatus, err
	}
	if resp.GetString(`return_code`) != wxpay.Success {
		return config.EmptyTradeStatus, errors.New(resp.GetString(`return_msg`))
	}
	status := config.TradeStatusProcessing
	/*
		SUCCESS—退款成功
		REFUNDCLOSE—退款关闭。
		PROCESSING—退款处理中
		CHANGE—退款异常，退款到银行发现用户的卡作废或者冻结了，导致原路退款银行卡失败，可前往商户平台（pay.weixin.qq.com）-交易中心，手动处理此笔退款。。
	*/
	refundCount := resp.GetInt64(`refund_count`)
	refundList := []echo.H{}
	if refundCount > 0 {
		status = config.TradeStatusSuccess
	}
	var refundTotalFee int64
	for i := int64(0); i < refundCount; i++ {
		refundStatus := resp.GetString(fmt.Sprintf(`refund_status_%d`, i))
		refundFeeInt := resp.GetInt64(fmt.Sprintf(`refund_fee_%d`, i))
		outRefundNo := resp.GetInt64(fmt.Sprintf(`out_refund_no_%d`, i))
		switch refundStatus {
		case `SUCCESS`:
			refundStatus = config.TradeStatusSuccess
		case `REFUNDCLOSE`:
			refundStatus = config.TradeStatusClosed
		case `PROCESSING`:
			refundStatus = config.TradeStatusProcessing
		case `CHANGE`:
			refundStatus = config.TradeStatusException
		}
		refundList = append(refundList, echo.H{
			`refundList`:  refundStatus,
			`refundFee`:   payment.CutFloat(float64(refundFeeInt)/100, 2),
			`outRefundNo`: outRefundNo,
		})
		if status == config.TradeStatusSuccess && refundStatus != config.TradeStatusSuccess {
			status = config.TradeStatusProcessing
		}
		refundTotalFee += refundFeeInt
	}
	return config.NewTradeStatus(status, echo.H{
		`trade_no`:     resp.GetString(`transaction_id`),
		`out_trade_no`: resp.GetString(`out_trade_no`),
		`currency`:     resp.GetString(`fee_type`),
		`refund_fee`:   payment.CutFloat(float64(refundTotalFee)/100, 2),
		`total_amount`: payment.CutFloat(float64(resp.GetInt64(`total_fee`))/100, 2),
		`refundList`:   refundList,
	}), err
}
