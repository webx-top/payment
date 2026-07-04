package unionpay
import (
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
// 银联开放平台 API 文档: https://open.unionpay.com/tjweb/api/dictionary?index=1&catId=10&apiSvcId=448
func init() {
	payment.Register(Name, echo.T(`UnionPay`), New)
}
func New() payment.Driver {
	return &UnionPay{}
}
type UnionPay struct {
	account        *config.Account
	notifyCallback payment.NotifyCallback
	gatewayHost    string
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
	a.gatewayHost = account.Options.Extra.String("gatewayHost")
	if len(a.gatewayHost) == 0 {
		if a.account.Debug {
			a.gatewayHost = "https://gateway.test.95516.com"
		} else {
			a.gatewayHost = "https://gateway.95516.com"
		}
	}
	return a
}
func (a *UnionPay) client() *resty.Request {
	return restyclient.Retryable().
		SetHeader("Content-Type", "application/x-www-form-urlencoded;charset=UTF-8").
		SetHeader("Accept", "application/json")
}
// buildParams 构造银联请求公共参数
// payURL 返回对应交易类型的网关地址
func (a *UnionPay) payURL(tradeType string) string {
	switch tradeType {
	case "02": // 手机控件支付
		return a.gatewayHost + "/gateway/api/appTransReq.do"
	default: // 网关支付
		return a.gatewayHost + "/gateway/api/frontTransReq.do"
	}
}
// Pay 创建银联支付订单
// 文档:
//   网关支付: https://open.unionpay.com/tjweb/acproduct/APIList?apiservId=448&acpAPIId=754&bussType=0
//   手机控件支付: https://open.unionpay.com/tjweb/acproduct/APIList?apiservId=448&acpAPIId=961&bussType=0
// 支持模式: Subtype=01 网关支付, Subtype=02 手机控件支付
func (a *UnionPay) Pay(ctx echo.Context, cfg *config.Pay) (*config.PayResponse, error) {
	if a.account == nil || a.account.MerchantID == "" || a.account.AppSecret == "" {
		return nil, fmt.Errorf("unionpay: missing merchant config")
	}
	tradeType := defaultString(cfg.Subtype, "01")
	body := map[string]string{
		"version":      "5.1.0",
		"encoding":     "UTF-8",
		"certId":       a.account.PublicKey,
		"signMethod":   "01",
		"txnType":      "01",
		"txnSubType":   "01",
		"bizType":      "000201",
		"channelType":  "07",
		"accessType":   "0",
		"merId":        a.account.MerchantID,
		"orderId":      cfg.OutTradeNo,
		"txnTime":      time.Now().Format("20060102150405"),
		"txnAmt":       AmountToYi(cfg.Amount),
		"currencyCode": "156",
		"backUrl":      defaultString(cfg.NotifyURL, ""),
		"frontUrl":     defaultString(cfg.ReturnURL, ""),
		"orderDesc":    defaultString(cfg.Subject, ""),
		"reqReserved":  cfg.PassbackParams,
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
	result := &config.PayResponse{
		TradeNo: cfg.OutTradeNo,
		Raw:     body,
	}
	if tradeType == "02" {
		// 手机控件支付: 服务端 POST 到 appTransReq.do 获取 tn
		resp, err := a.client().
			SetFormData(body).
			Post(a.payURL(tradeType))
		if err != nil {
			return nil, err
		}
		if resp.StatusCode() != http.StatusOK {
			return nil, fmt.Errorf("unionpay: app pay request failed %d", resp.StatusCode())
		}
		// 解析响应中的 tn（银联返回 URL-encoded 表单数据）
		respValues, err := parseQuery(resp.String())
		if err != nil {
			return nil, fmt.Errorf("unionpay: parse app pay response failed: %v", err)
		}
		if tn := respValues.Get("tn"); len(tn) > 0 {
			result.TradeNo = tn
			result.Params = echo.H{"tn": tn, "trade_type": tradeType}
		} else {
			return nil, fmt.Errorf("unionpay: app pay response missing tn: %s", resp.String())
		}
	} else {
		// 网关支付: 返回 action URL + 参数，由前端渲染 HTML 表单自动提交
		result.RedirectURL = a.payURL(tradeType)
		params := make(echo.H, len(body))
		for k, v := range body {
			params[k] = v
		}
		result.Params = params
	}
	return result, nil
}
// PayNotify 处理银联支付回调通知
// 文档: https://open.unionpay.com/tjweb/acproduct/APIList?apiservId=448&acpAPIId=754&bussType=0 (后台通知接口说明)
func (a *UnionPay) PayNotify(ctx echo.Context) error {
	payload := parseFormData(ctx)
	if err := a.VerifyPayloadSign(payload); err != nil {
		return ctx.String("fail")
	}
	respCode := payload["respCode"]
	orderId := payload["orderId"]
	txnAmt := payload["txnAmt"]
	if a.notifyCallback != nil && respCode == "00" {
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
// 文档: https://open.unionpay.com/tjweb/acproduct/APIList?apiservId=448&acpAPIId=757&bussType=0
func (a *UnionPay) PayQuery(ctx echo.Context, cfg *config.Query) (*config.Result, error) {
	body := map[string]string{
		"version":     "5.1.0",
		"encoding":    "UTF-8",
		"certId":      a.account.PublicKey,
		"signMethod":  "01",
		"txnType":     "00",
		"txnSubType":  "00",
		"bizType":     "000000",
		"accessType":  "0",
		"merId":       a.account.MerchantID,
		"orderId":     cfg.OutTradeNo,
		"txnTime":     "",
		"reqReserved": "",
	}
	sign := a.GenerateSign(body)
	body["signature"] = sign
	resp, err := a.client().
		SetFormData(body).
		Post(a.gatewayHost + "/gateway/api/queryTrans.do")
	if err != nil {
		return nil, err
	}
	if resp.StatusCode() != http.StatusOK {
		return nil, fmt.Errorf("unionpay: query failed %d", resp.StatusCode())
	}
	rValues, err := parseQuery(resp.String())
	if err != nil {
		return nil, fmt.Errorf("unionpay: parse query response failed: %v", err)
	}
	// origRespCode 表原始交易状态，respCode 仅表查询动作是否成功
	origRespCode := rValues.Get("origRespCode")
	if len(origRespCode) == 0 {
		origRespCode = rValues.Get("respCode")
	}
	status := MappingRespCode(origRespCode)
	return &config.Result{
		Operation:   config.OperationPayment,
		Status:      status,
		TradeNo:     rValues.Get("queryId"),
		OutTradeNo:  rValues.Get("orderId"),
		Currency:    "CNY",
		TotalAmount: YiToAmount(rValues.Get("txnAmt")),
		Raw:         rValues,
	}, nil
}
// Refund 发起退款
// 文档: https://open.unionpay.com/tjweb/acproduct/APIList?apiservId=448&acpAPIId=756&bussType=0
func (a *UnionPay) Refund(ctx echo.Context, cfg *config.Refund) (*config.Result, error) {
	body := map[string]string{
		"version":      "5.1.0",
		"encoding":     "UTF-8",
		"certId":       a.account.PublicKey,
		"signMethod":   "01",
		"txnType":      "04",
		"txnSubType":   "00",
		"bizType":      "000201",
		"channelType":  "07",
		"accessType":   "0",
		"merId":        a.account.MerchantID,
		"orderId":      cfg.OutRefundNo,
		"origQryId":    cfg.TradeNo,
		"txnTime":      time.Now().Format("20060102150405"),
		"txnAmt":       AmountToYi(cfg.RefundAmount),
		"currencyCode": "156",
		"backUrl":      defaultString(cfg.NotifyURL, ""),
	}
	sign := a.GenerateSign(body)
	body["signature"] = sign
	resp, err := a.client().
		SetFormData(body).
		Post(a.gatewayHost + "/gateway/api/backTransReq.do")
	if err != nil {
		return nil, err
	}
	if resp.StatusCode() != http.StatusOK {
		return nil, fmt.Errorf("unionpay: refund failed %d", resp.StatusCode())
	}
	rValues, err := parseQuery(resp.String())
	if err != nil {
		return nil, fmt.Errorf("unionpay: parse refund response failed: %v", err)
	}
	status := MapRefundRespCode(rValues.Get("respCode"))
	return &config.Result{
		Operation:   config.OperationRefund,
		Status:      status,
		TradeNo:     rValues.Get("origQryId"),
		OutTradeNo:  cfg.OutTradeNo,
		Currency:    "CNY",
		RefundFee:   YiToAmount(rValues.Get("txnAmt")),
		RefundNo:    rValues.Get("queryId"),
		OutRefundNo: rValues.Get("orderId"),
		Raw:         rValues,
	}, nil
}
// RefundNotify 处理银联退款回调通知
// 文档: https://open.unionpay.com/tjweb/acproduct/APIList?apiservId=448&acpAPIId=756&bussType=0 (后台通知接口说明)
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
	if respCode == "00" {
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
// 文档: https://open.unionpay.com/tjweb/acproduct/APIList?apiservId=448&acpAPIId=757&bussType=0 (查询交易)
func (a *UnionPay) RefundQuery(ctx echo.Context, cfg *config.Query) (*config.Result, error) {
	body := map[string]string{
		"version":     "5.1.0",
		"encoding":    "UTF-8",
		"certId":      a.account.PublicKey,
		"signMethod":  "01",
		"txnType":     "00",
		"txnSubType":  "00",
		"bizType":     "000000",
		"accessType":  "0",
		"merId":       a.account.MerchantID,
		"orderId":     cfg.OutRefundNo,
		"txnTime":     "",
		"reqReserved": "",
	}
	sign := a.GenerateSign(body)
	body["signature"] = sign
	resp, err := a.client().
		SetFormData(body).
		Post(a.gatewayHost + "/gateway/api/queryTrans.do")
	if err != nil {
		return nil, err
	}
	if resp.StatusCode() != http.StatusOK {
		return nil, fmt.Errorf("unionpay: refund query failed %d", resp.StatusCode())
	}
	rValues, err := parseQuery(resp.String())
	if err != nil {
		return nil, fmt.Errorf("unionpay: parse refund query response failed: %v", err)
	}
	// 退款查询也用 origRespCode 判断实际退款状态
	origRespCode := rValues.Get("origRespCode")
	if len(origRespCode) == 0 {
		origRespCode = rValues.Get("respCode")
	}
	status := MapRefundRespCode(origRespCode)
	return &config.Result{
		Operation:   config.OperationRefund,
		Status:      status,
		TradeNo:     rValues.Get("origQryId"),
		OutTradeNo:  cfg.OutTradeNo,
		Currency:    "CNY",
		RefundFee:   YiToAmount(rValues.Get("txnAmt")),
		RefundNo:    rValues.Get("queryId"),
		OutRefundNo: cfg.OutRefundNo,
		Raw:         rValues,
	}, nil
}
