package wechat

import (
	"encoding/base64"
	"errors"
	"fmt"
	"io/ioutil"
	"strings"

	"github.com/objcoding/wxpay"
	"github.com/webx-top/codec"
	"github.com/webx-top/com"
	"github.com/webx-top/echo"
	"github.com/webx-top/echo/param"
	"github.com/webx-top/payment"
	"github.com/webx-top/payment/config"
)

const Name = `wechat`

var supports = config.Supports{
	config.SupportPayNotify,
	config.SupportPayQuery,
	config.SupportRefund,
	config.SupportRefundNotify,
	config.SupportRefundQuery,
}

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

func (a *Wechat) IsSupported(s config.Support) bool {
	return supports.IsSupported(s)
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

func (a *Wechat) Pay(ctx echo.Context, cfg *config.Pay) (*config.PayResponse, error) {
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
	if a.account.Options.Extra != nil {
		payConfig := a.account.Options.Extra.GetStore(`payConfig`)
		for k, v := range payConfig {
			wxParams[k] = param.AsString(v)
		}
	}
	if cfg.Options != nil {
		params := cfg.Options.GetStore(`params`)
		for k := range params {
			wxParams[k] = params.String(k)
		}
	}
	// documentation https://pay.weixin.qq.com/wiki/doc/api/jsapi.php?chapter=9_1
	resp, err := a.Client().UnifiedOrder(wxParams)
	if err != nil {
		return nil, err
	}
	resp = a.translateWxpayAppResult(cfg, resp)
	params := wxpay.Params(resp)
	result := &config.PayResponse{
		QRCodeContent: params.GetString(`code_url`),
		Raw:           resp,
	}
	if cfg.Device == config.App {
		result.Params = param.AsStore(resp)
	}
	return result, nil
}

// PayNotify 付款回调处理
// documentation https://pay.weixin.qq.com/wiki/doc/api/jsapi.php?chapter=9_7&index=8
func (a *Wechat) PayNotify(ctx echo.Context) error {
	body := ctx.Request().Body()
	defer body.Close()
	b, err := ioutil.ReadAll(body)
	if err != nil {
		return err
	}
	resp := wxpay.XmlToMap(string(b))
	if !a.Client().ValidSign(resp) {
		return config.ErrSignature
	}
	var status string
	switch resp.GetString(`return_code`) {
	case wxpay.Success:
		status = config.TradeStatusSuccess
	case wxpay.Fail:
		status = config.TradeStatusException
	}
	var isSuccess = true
	var xmlString string
	noti := wxpay.Notifies{}
	if a.notifyCallback != nil {
		result := &config.Result{
			Operation:      config.OperationPayment,
			Status:         status,
			TradeNo:        resp.GetString(`transaction_id`),
			OutTradeNo:     resp.GetString(`out_trade_no`),
			Currency:       resp.GetString(`fee_type`),
			PassbackParams: resp.GetString(`attach`),
			TotalAmount:    param.AsFloat64(payment.CutFloat(float64(resp.GetInt64(`total_fee`))/100, 2)),
			Reason:         resp.GetString(`return_msg`),
			Raw:            resp,
		}
		ctx.Set(`notify`, result)
		if err := a.notifyCallback(ctx); err != nil {
			isSuccess = false
		}
	}
	if !isSuccess {
		xmlString = noti.NotOK("failed")
	} else {
		xmlString = noti.OK()
	}

	return ctx.XMLBlob([]byte(xmlString))
}

func (a *Wechat) PayQuery(ctx echo.Context, cfg *config.Query) (*config.Result, error) {
	params := make(wxpay.Params)
	if len(cfg.TradeNo) > 0 {
		params.SetString("transaction_id", cfg.TradeNo)
	} else {
		params.SetString("out_trade_no", cfg.OutTradeNo)
	}
	// documentation https://pay.weixin.qq.com/wiki/doc/api/jsapi.php?chapter=9_2
	resp, err := a.Client().OrderQuery(params)
	if err != nil {
		return nil, err
	}
	if resp.GetString(`return_code`) != wxpay.Success {
		return nil, errors.New(resp.GetString(`return_msg`))
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
	return &config.Result{
		Operation:      config.OperationPayment,
		Status:         tradeStatus,
		TradeNo:        resp.GetString(`transaction_id`),
		OutTradeNo:     resp.GetString(`out_trade_no`),
		Currency:       resp.GetString(`fee_type`),
		PassbackParams: resp.GetString(`attach`),
		TotalAmount:    param.AsFloat64(payment.CutFloat(float64(resp.GetInt64(`total_fee`))/100, 2)),
		Reason:         resp.GetString(`return_msg`),
		Raw:            resp,
	}, err
}

func (a *Wechat) Refund(ctx echo.Context, cfg *config.Refund) (*config.Result, error) {
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
	var status string
	if returnCode == wxpay.Fail {
		status = config.TradeStatusException
	} else if returnCode == wxpay.Success {
		status = config.TradeStatusSuccess
	}
	return &config.Result{
		Operation:   config.OperationRefund,
		Status:      status,
		TradeNo:     resp.GetString(`transaction_id`),
		OutTradeNo:  resp.GetString(`out_trade_no`),
		Currency:    resp.GetString(`fee_type`),
		TotalAmount: param.AsFloat64(payment.CutFloat(float64(resp.GetInt64(`total_fee`))/100, 2)),
		Reason:      resp.GetString(`return_msg`),
		RefundFee:   param.AsFloat64(payment.CutFloat(float64(resp.GetInt64(`refund_fee`))/100, 2)),
		RefundNo:    resp.GetString(`refund_id`),
		OutRefundNo: resp.GetString(`out_refund_no`),
		Raw:         resp,
	}, err
}

// RefundNotify 退款回调处理
// documentation https://pay.weixin.qq.com/wiki/doc/api/jsapi.php?chapter=9_16&index=10
func (a *Wechat) RefundNotify(ctx echo.Context) error {
	body := ctx.Request().Body()
	defer body.Close()
	b, err := ioutil.ReadAll(body)
	if err != nil {
		return err
	}
	resp := wxpay.XmlToMap(string(b))
	if !a.Client().ValidSign(resp) {
		return config.ErrSignature
	}
	var status string
	switch resp.GetString(`return_code`) {
	case wxpay.Success:
		status = config.TradeStatusSuccess
	case wxpay.Fail:
		status = config.TradeStatusException
	}
	reqInfo := resp.GetString(`req_info`)
	b, err = base64.StdEncoding.DecodeString(reqInfo)
	if err != nil {
		return fmt.Errorf(`base64decode(%v): %v`, reqInfo, err)
	}
	key := strings.ToLower(com.Md5(a.account.AppSecret))
	crypto := codec.NewAesECBCrypto(`AES-256`)
	b = crypto.DecodeBytes(b, []byte(key))
	for k, v := range XmlToMap(string(b)) {
		resp[k] = v
	}
	var isSuccess = true
	var xmlString string
	noti := wxpay.Notifies{}
	if a.notifyCallback != nil {
		result := &config.Result{
			Operation:   config.OperationRefund,
			Status:      status,
			TradeNo:     resp.GetString(`transaction_id`),
			OutTradeNo:  resp.GetString(`out_trade_no`),
			Currency:    resp.GetString(`fee_type`),
			TotalAmount: param.AsFloat64(payment.CutFloat(float64(resp.GetInt64(`total_fee`))/100, 2)),
			Reason:      resp.GetString(`return_msg`),
			RefundFee:   param.AsFloat64(payment.CutFloat(float64(resp.GetInt64(`refund_fee`))/100, 2)),
			RefundNo:    resp.GetString(`refund_id`),
			OutRefundNo: resp.GetString(`out_refund_no`),
			Raw:         resp,
		}
		ctx.Set(`notify`, result)
		if err := a.notifyCallback(ctx); err != nil {
			isSuccess = false
		}
	}
	if !isSuccess {
		xmlString = noti.NotOK("failed")
	} else {
		xmlString = noti.OK()
	}

	return ctx.XMLBlob([]byte(xmlString))
}

// RefundQuery 退款查询
func (a *Wechat) RefundQuery(ctx echo.Context, cfg *config.Query) (*config.Result, error) {
	params := make(wxpay.Params)
	if len(cfg.RefundNo) > 0 {
		params.SetString("refund_id", cfg.RefundNo)
	} else if len(cfg.OutRefundNo) > 0 {
		params.SetString("out_refund_no", cfg.OutRefundNo)
	} else if len(cfg.TradeNo) > 0 {
		params.SetString("transaction_id", cfg.TradeNo)
	} else {
		params.SetString("out_trade_no", cfg.OutTradeNo)
	}
	// documentation https://pay.weixin.qq.com/wiki/doc/api/jsapi.php?chapter=9_5
	resp, err := a.Client().RefundQuery(params)
	if err != nil {
		return nil, err
	}
	if resp.GetString(`return_code`) != wxpay.Success {
		return nil, errors.New(resp.GetString(`return_msg`))
	}
	status := config.TradeStatusProcessing
	/*
		SUCCESS—退款成功
		REFUNDCLOSE—退款关闭。
		PROCESSING—退款处理中
		CHANGE—退款异常，退款到银行发现用户的卡作废或者冻结了，导致原路退款银行卡失败，可前往商户平台（pay.weixin.qq.com）-交易中心，手动处理此笔退款。。
	*/
	refundCount := resp.GetInt64(`refund_count`)
	if refundCount > 0 {
		status = config.TradeStatusSuccess
	}
	var refundTotalFee int64
	refundItems := []*config.RefundItem{}
	for i := int64(0); i < refundCount; i++ {
		refundStatus := resp.GetString(fmt.Sprintf(`refund_status_%d`, i))
		refundFeeInt := resp.GetInt64(fmt.Sprintf(`refund_fee_%d`, i))
		outRefundNo := resp.GetString(fmt.Sprintf(`out_refund_no_%d`, i))
		refundNo := resp.GetString(fmt.Sprintf(`out_id_%d`, i))
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
		refundItems = append(refundItems, &config.RefundItem{
			Status:      refundStatus,
			RefundFee:   param.AsFloat64(payment.CutFloat(float64(refundFeeInt)/100, 2)),
			OutRefundNo: outRefundNo,
			RefundNo:    refundNo,
		})
		if status == config.TradeStatusSuccess && refundStatus != config.TradeStatusSuccess {
			status = config.TradeStatusProcessing
		}
		refundTotalFee += refundFeeInt
	}
	return &config.Result{
		Operation:   config.OperationRefund,
		Status:      status,
		TradeNo:     resp.GetString(`transaction_id`),
		OutTradeNo:  resp.GetString(`out_trade_no`),
		Currency:    resp.GetString(`fee_type`),
		TotalAmount: param.AsFloat64(payment.CutFloat(float64(resp.GetInt64(`total_fee`))/100, 2)),
		Reason:      resp.GetString(`return_msg`),
		RefundFee:   param.AsFloat64(payment.CutFloat(float64(refundTotalFee)/100, 2)),
		RefundNo:    ``,
		OutRefundNo: ``,
		RefundItems: refundItems,
		Raw:         resp,
	}, err
}
