package unionpay

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/admpub/resty/v2"
	"github.com/webx-top/echo"
	"github.com/webx-top/payment"
	"github.com/webx-top/payment/config"
	"github.com/webx-top/restyclient"
)

const Name = `unionpay`

var supports = config.Supports{
	config.SupportPayNotify,
	config.SupportPayQuery,
	config.SupportRefund,
	config.SupportRefundQuery,
}

var SupportedCurrencies = []string{
	"CNY",
}

// 银联开放平台 API 文档: https://open.unionpay.com/tjweb/api/detail?id=2

func init() {
	payment.Register(Name, echo.T(`UnionPay`), New)
}

func New() payment.Driver {
	return &UnionPay{}
}

type UnionPay struct {
	account        *config.Account
	notifyCallback payment.NotifyCallback
	apiURL         string
	frontURL       string // 前台交易地址
}

func (a *UnionPay) IsSupported(s config.Support) bool {
	return supports.IsSupported(s)
}

func (a *UnionPay) SetNotifyCallback(callback payment.NotifyCallback) payment.Driver {
	a.notifyCallback = callback
	return a
}

func (a *UnionPay) SetAccount(account *config.Account) payment.Driver {
	a.account = account
	a.apiURL = account.Options.Extra.String("apiURL")
	if len(a.apiURL) == 0 {
		if a.account.Debug {
			a.apiURL = "https://gateway.test.95516.com/gateway/api"
			a.frontURL = "https://gateway.test.95516.com/frontTransReq"
		} else {
			a.apiURL = "https://gateway.95516.com/gateway/api"
			a.frontURL = "https://gateway.95516.com/frontTransReq"
		}
	}
	a.frontURL = account.Options.Extra.String("frontURL")
	if len(a.frontURL) == 0 {
		if a.account.Debug {
			a.frontURL = "https://gateway.test.95516.com/frontTransReq"
		} else {
			a.frontURL = "https://gateway.95516.com/frontTransReq"
		}
	}
	return a
}

func (a *UnionPay) client() *resty.Request {
	return restyclient.Retryable().
		SetHeader("Content-Type", "application/x-www-form-urlencoded;charset=UTF-8").
		SetHeader("Accept", "application/json")
}

// Pay 创建银联支付订单(网关支付/手机控件支付)
// 文档: https://open.unionpay.com/tjweb/api/detail?id=2 (消费类交易)
// 支持模式: 01-网关支付, 02-手机控件支付
func (a *UnionPay) Pay(ctx echo.Context, cfg *config.Pay) (*config.PayResponse, error) {
	if a.account == nil || a.account.MerchantID == "" || a.account.AppSecret == "" {
		return nil, fmt.Errorf("unionpay: missing merchant config")
	}

	bizType := "000201" // 消费
	tradeType := defaultString(cfg.Subtype, "01") // 默认网关支付

	body := map[string]string{
		"version":     "5.1.0",
		"encoding":    "UTF-8",
		"certId":      a.account.PublicKey,
		"signMethod":  "01",     // RSA签名
		"txnType":     "01",     // 消费交易
		"txnSubType":  "01",     // 自助消费
		"bizType":     bizType,
		"channelType": "07",     // PC网页
		"backUrl":     defaultString(cfg.NotifyURL, ""),
		"frontUrl":    defaultString(cfg.ReturnURL, ""),
		"merId":       a.account.MerchantID,
		"orderId":     cfg.OutTradeNo,
		"txnTime":     time.Now().Format("20060102150405"),
		"txnAmt":      AmountToYi(cfg.Amount),
		"currencyCode": "156", // CNY
		"orderDesc":   defaultString(cfg.Subject, ""),
		"reqReserved": cfg.PassbackParams,
	}

	switch tradeType {
	case "02":
		body["channelType"] = "08" // 手机
	default:
		body["channelType"] = "07" // PC网页
	}

	// 签名
	sign := a.GenerateSign(body)
	body["signature"] = sign

	resp, err := a.client().
		SetBody(body).
		Post(a.frontURL)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode() != http.StatusOK {
		return nil, fmt.Errorf("unionpay: pay request failed %d", resp.StatusCode())
	}

	result := &config.PayResponse{
		TradeNo:     cfg.OutTradeNo,
		RedirectURL: resp.Header().Get("Location"),
		Raw:         body,
	}

	// 如果响应体包含 tn(交易流水号)，返回给前端用于手机控件
	var payResp map[string]string
	json.Unmarshal(resp.Body(), &payResp)
	if tn, ok := payResp["tn"]; ok {
		result.Params = echo.H{"tn": tn, "trade_type": tradeType}
		result.TradeNo = cfg.OutTradeNo
	}

	return result, nil
}

// PayNotify 处理银联回调通知
// 文档: https://open.unionpay.com/tjweb/api/detail?id=175 (后台通知接口说明)
func (a *UnionPay) PayNotify(ctx echo.Context) error {
	payload := parseFormData(ctx)

	if err := a.VerifyPayloadSign(payload); err != nil {
		return ctx.String("fail")
	}

	// 判断交易状态
	respCode := payload["respCode"]
	orderId := payload["orderId"]
	txnAmt := payload["txnAmt"]

	if a.notifyCallback != nil && respCode == "00" { // 成功
		result := &config.Result{
			Operation:   config.OperationPayment,
			Status:      config.TradeStatusSuccess,
			TradeNo:     payload["queryId"],
			OutTradeNo:  orderId,
			Currency:    "CNY",
			TotalAmount: YiToAmount(txnAmt),
			Raw:         payload,
		}
		if err := a.notifyCallback(ctx, result); err != nil {
			return err
		}
	} else if respCode != "00" && a.notifyCallback != nil {
		status := MappingRespCode(respCode)
		if status == config.TradeStatusClosed || status == config.TradeStatusException {
			result := &config.Result{
				Operation:  config.OperationPayment,
				Status:     status,
				TradeNo:    payload["queryId"],
				OutTradeNo: orderId,
				Currency:   "CNY",
				Raw:        payload,
			}
			a.notifyCallback(ctx, result)
		}
	}

	return ctx.String("ok")
}

// PayQuery 查询交易状态
// 文档: https://open.unionpay.com/tjweb/api/detail?id=3 (查询交易)
func (a *UnionPay) PayQuery(ctx echo.Context, cfg *config.Query) (*config.Result, error) {
	body := map[string]string{
		"version":    "5.1.0",
		"encoding":   "UTF-8",
		"certId":     a.account.PublicKey,
		"signMethod": "01",
		"txnType":    "00",     // 查询交易
		"txnSubType": "00",
		"bizType":    "000000",
		"merId":      a.account.MerchantID,
		"orderId":    cfg.TradeNo,
		"txnTime":    "", // 查询时可不填
		"reqReserved": "",
	}
	sign := a.GenerateSign(body)
	body["signature"] = sign

	var response struct {
		RespCode string `json:"respCode"`
		RespMsg  string `json:"respMsg"`
		QueryId  string `json:"queryId"`
		OrderId  string `json:"orderId"`
		TxnAmt   string `json:"txnAmt"`
		TxnTime  string `json:"txnTime"`
		Status   string `json:"status"` // 原始交易状态码
	}

	resp, err := a.client().
		SetResult(&response).
		SetBody(body).
		Post(a.apiURL + "/query.trans")
	if err != nil {
		return nil, err
	}
	if resp.StatusCode() != http.StatusOK {
		return nil, fmt.Errorf("unionpay: query failed %d", resp.StatusCode())
	}

	status := MappingRespCode(response.RespCode)

	return &config.Result{
		Operation:   config.OperationPayment,
		Status:      status,
		TradeNo:     response.QueryId,
		OutTradeNo:  response.OrderId,
		Currency:    "CNY",
		TotalAmount: YiToAmount(response.TxnAmt),
		Raw:         &response,
	}, nil
}

// Refund 发起退款
// 文档: https://open.unionpay.com/tjweb/api/detail?id=4 (退款交易)
func (a *UnionPay) Refund(ctx echo.Context, cfg *config.Refund) (*config.Result, error) {
	body := map[string]string{
		"version":     "5.1.0",
		"encoding":    "UTF-8",
		"certId":      a.account.PublicKey,
		"signMethod":  "01",
		"txnType":     "04",     // 退款
		"txnSubType":  "00",     // 原交易退款
		"bizType":     "000402", // 退款业务
		"channelType": "07",
		"backUrl":     defaultString(cfg.NotifyURL, ""),
		"merId":       a.account.MerchantID,
		"orderId":     cfg.OutRefundNo, // 退款订单号
		"origQryId":   cfg.TradeNo,     // 原交易查询号
		"txnTime":     time.Now().Format("20060102150405"),
		"txnAmt":      AmountToYi(cfg.RefundAmount), // 退款金额(分)
		"currencyCode": "156",
	}

	sign := a.GenerateSign(body)
	body["signature"] = sign

	var response struct {
		RespCode  string `json:"respCode"`
		RespMsg   string `json:"respMsg"`
		QueryId   string `json:"queryId"`
		OrderId   string `json:"orderId"`
		OrigQryId string `json:"origQryId"`
		TxnAmt    string `json:"txnAmt"`
	}

	resp, err := a.client().
		SetResult(&response).
		SetBody(body).
		Post(a.apiURL + "/refund.trans")
	if err != nil {
		return nil, err
	}
	if resp.StatusCode() != http.StatusOK {
		return nil, fmt.Errorf("unionpay: refund failed %d", resp.StatusCode())
	}

	status := MapRefundRespCode(response.RespCode)

	return &config.Result{
		Operation:   config.OperationRefund,
		Status:      status,
		TradeNo:     response.OrigQryId,
		OutTradeNo:  cfg.OutTradeNo,
		Currency:    "CNY",
		RefundFee:   YiToAmount(response.TxnAmt),
		RefundNo:    response.QueryId,
		OutRefundNo: cfg.OutRefundNo,
		Raw:         &response,
	}, nil
}

// RefundNotify 处理银联退款回调通知
// 文档: https://open.unionpay.com/tjweb/api/detail?id=175 (后台通知接口说明)
// 退款通知与支付通知共用同一接口，通过 txnType 区分: 04=退款
func (a *UnionPay) RefundNotify(ctx echo.Context) error {
	payload := parseFormData(ctx)

	if err := a.VerifyPayloadSign(payload); err != nil {
		return ctx.String("fail")
	}

	txnType := payload["txnType"]
	respCode := payload["respCode"]

	if txnType != "04" {
		return ctx.String("ok")
	}

	if a.notifyCallback == nil {
		return ctx.String("ok")
	}

	orderId := payload["orderId"]
	queryId := payload["queryId"]
	txnAmt := payload["txnAmt"]
	origQryId := payload["origQryId"]

	if respCode == "00" { // 退款成功
		result := &config.Result{
			Operation:   config.OperationRefund,
			Status:      config.TradeStatusSuccess,
			TradeNo:     origQryId,
			OutTradeNo:  orderId,
			Currency:    "CNY",
			TotalAmount: YiToAmount(txnAmt),
			RefundFee:   YiToAmount(txnAmt),
			RefundNo:    queryId,
			OutRefundNo: orderId,
			Raw:         payload,
		}
		if err := a.notifyCallback(ctx, result); err != nil {
			return err
		}
	} else {
		status := MapRefundRespCode(respCode)
		result := &config.Result{
			Operation:  config.OperationRefund,
			Status:     status,
			TradeNo:    origQryId,
			OutTradeNo: orderId,
			Currency:   "CNY",
			Raw:        payload,
		}
		a.notifyCallback(ctx, result)
	}

	return ctx.String("ok")
}

// RefundQuery 查询退款结果
// 文档: https://open.unionpay.com/tjweb/api/detail?id=5 (查询退款)
func (a *UnionPay) RefundQuery(ctx echo.Context, cfg *config.Query) (*config.Result, error) {
	body := map[string]string{
		"version":    "5.1.0",
		"encoding":   "UTF-8",
		"certId":     a.account.PublicKey,
		"signMethod": "01",
		"txnType":    "00",
		"txnSubType": "00",
		"bizType":    "000000",
		"merId":      a.account.MerchantID,
		"orderId":    cfg.OutRefundNo, // 用退款单号查询
		"reqReserved": "",
	}
	sign := a.GenerateSign(body)
	body["signature"] = sign

	var response struct {
		RespCode  string `json:"respCode"`
		RespMsg   string `json:"respMsg"`
		QueryId   string `json:"queryId"`
		OrderId   string `json:"orderId"`
		TxnAmt    string `json:"txnAmt"`
		OrigQryId string `json:"origQryId"`
	}

	resp, err := a.client().
		SetResult(&response).
		SetBody(body).
		Post(a.apiURL + "/query.trans")
	if err != nil {
		return nil, err
	}
	if resp.StatusCode() != http.StatusOK {
		return nil, fmt.Errorf("unionpay: refund query failed %d", resp.StatusCode())
	}

	status := MapRefundRespCode(response.RespCode)

	return &config.Result{
		Operation:   config.OperationRefund,
		Status:      status,
		TradeNo:     response.OrigQryId,
		OutTradeNo:  cfg.OutTradeNo,
		Currency:    "CNY",
		RefundFee:   YiToAmount(response.TxnAmt),
		RefundNo:    response.QueryId,
		OutRefundNo: cfg.OutRefundNo,
		Raw:         &response,
	}, nil
}
