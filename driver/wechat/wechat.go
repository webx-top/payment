package wechat

import (
	"errors"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/go-pay/gopay"
	wechatv3 "github.com/go-pay/gopay/wechat/v3"
	"github.com/webx-top/echo"
	"github.com/webx-top/payment"
	"github.com/webx-top/payment/config"
)

const Name = `wechat`

// cstZone 东八区时区（微信支付接口要求的时间格式基准）
var cstZone = time.FixedZone(`CST`, 8*3600)

var supports = config.Supports{
	config.SupportPayNotify,
	config.SupportPayQuery,
	config.SupportRefund,
	config.SupportRefundNotify,
	config.SupportRefundQuery,
}

func init() {
	payment.Register(Name, echo.T(`微信支付`), New)
}

func New() payment.Driver {
	return &Wechat{}
}

// Wechat 微信支付V3驱动
// 文档: https://pay.weixin.qq.com/docs/merchant/apis/index.html
//
// Account字段映射:
//   - MerchantID: 商户号(mchid)
//   - AppID: 应用ID(appid)
//   - AppSecret: APIv3密钥(商户平台「账户中心-API安全」中设置)
//   - PrivateKey: 商户API私钥(PEM内容)；也可通过 CertPath 指定私钥文件路径
//   - Options.Extra["serialNo"]: 商户API证书序列号
type Wechat struct {
	account        *config.Account
	client         *wechatv3.ClientV3
	clientErr      error
	clientMutex    sync.Mutex
	notifyCallback payment.NotifyCallback
}

func (a *Wechat) IsSupported(s config.Support) bool {
	return supports.IsSupported(s)
}

func (a *Wechat) SetNotifyCallback(callback payment.NotifyCallback) payment.Driver {
	a.notifyCallback = callback
	return a
}

func (a *Wechat) SetAccount(account *config.Account) payment.Driver {
	a.account = account
	return a
}

func (a *Wechat) getAPIv3Key() string {
	if a.account != nil {
		return a.account.AppSecret
	}
	return ``
}

// Client 获取微信支付V3客户端
func (a *Wechat) Client() (*wechatv3.ClientV3, error) {
	a.clientMutex.Lock()
	defer a.clientMutex.Unlock()
	if a.client != nil {
		return a.client, nil
	}
	if a.clientErr != nil {
		return nil, a.clientErr
	}
	privateKey := a.account.PrivateKey
	if len(privateKey) == 0 && len(a.account.CertPath) > 0 {
		b, err := os.ReadFile(a.account.CertPath)
		if err != nil {
			a.clientErr = err
			return nil, err
		}
		privateKey = string(b)
	}
	var serialNo string
	if a.account.Options.Extra != nil {
		serialNo = a.account.Options.Extra.String(`serialNo`)
	}
	client, err := wechatv3.NewClientV3(a.account.MerchantID, serialNo, a.getAPIv3Key(), privateKey)
	if err != nil {
		a.clientErr = err
		return nil, err
	}
	if a.account.Debug {
		client.DebugSwitch = gopay.DebugOn
	}
	// 自动下载并刷新微信平台证书，用于回调通知验签
	if err = client.AutoVerifySign(); err != nil {
		a.clientErr = err
		return nil, err
	}
	a.client = client
	return client, nil
}

func (a *Wechat) Pay(ctx echo.Context, cfg *config.Pay) (*config.PayResponse, error) {
	client, err := a.Client()
	if err != nil {
		return nil, err
	}
	bm := gopay.BodyMap{}
	bm.Set("appid", a.account.AppID)
	bm.Set("mchid", a.account.MerchantID)
	bm.Set("description", cfg.Subject)
	bm.Set("out_trade_no", cfg.OutTradeNo)
	if len(cfg.NotifyURL) > 0 {
		bm.Set("notify_url", cfg.NotifyURL)
	}
	if !cfg.ExpiredAt.IsZero() {
		// 微信要求RFC3339格式(带时区偏移)，强制转东八区避免输出"Z"后缀
		bm.Set("time_expire", cfg.ExpiredAt.In(cstZone).Format(time.RFC3339))
	}
	if cfg.Options != nil {
		if v := cfg.Options.String(`attach`); len(v) > 0 {
			bm.Set("attach", v)
		}
	}
	bm.SetBodyMap("amount", func(b gopay.BodyMap) {
		b.Set("total", MoneyFeeToInt64(cfg.Amount))
		b.Set("currency", `CNY`)
	})

	result := &config.PayResponse{
		TradeNo: cfg.OutTradeNo,
	}

	isJSAPI := strings.Contains(ctx.Request().UserAgent(), `MicroMessenger`)
	switch {
	case isJSAPI: // 公众号/小程序支付
		openid := ``
		if cfg.Options != nil {
			openid = cfg.Options.String(`openid`)
		}
		if len(openid) == 0 {
			return nil, errors.New(`openid required for jsapi payment`)
		}
		bm.SetBodyMap("payer", func(b gopay.BodyMap) {
			b.Set("openid", openid)
		})
		wxRsp, err := client.V3TransactionJsapi(ctx, bm)
		if err != nil {
			return nil, err
		}
		if err = checkV3Rsp(wxRsp.Code, &wxRsp.ErrResponse, wxRsp.Error); err != nil {
			return nil, err
		}
		sign, err := client.PaySignOfJSAPI(a.account.AppID, wxRsp.Response.PrepayId)
		if err != nil {
			return nil, err
		}
		result.Params = paramsToStore(sign)
		result.Raw = sign
	case cfg.Device == config.App: // APP支付
		wxRsp, err := client.V3TransactionApp(ctx, bm)
		if err != nil {
			return nil, err
		}
		if err = checkV3Rsp(wxRsp.Code, &wxRsp.ErrResponse, wxRsp.Error); err != nil {
			return nil, err
		}
		sign, err := client.PaySignOfApp(a.account.AppID, wxRsp.Response.PrepayId)
		if err != nil {
			return nil, err
		}
		result.Params = paramsToStore(sign)
		result.Raw = sign
	case cfg.Device == config.Web: // Native扫码支付
		wxRsp, err := client.V3TransactionNative(ctx, bm)
		if err != nil {
			return nil, err
		}
		if err = checkV3Rsp(wxRsp.Code, &wxRsp.ErrResponse, wxRsp.Error); err != nil {
			return nil, err
		}
		result.QRCodeContent = wxRsp.Response.CodeUrl
		result.Raw = wxRsp.Response
	default: // H5支付
		bm.SetBodyMap("scene_info", func(b gopay.BodyMap) {
			b.Set("payer_client_ip", ctx.RealIP())
			b.SetBodyMap("h5_info", func(h gopay.BodyMap) {
				h.Set("type", `Wap`)
			})
		})
		wxRsp, err := client.V3TransactionH5(ctx, bm)
		if err != nil {
			return nil, err
		}
		if err = checkV3Rsp(wxRsp.Code, &wxRsp.ErrResponse, wxRsp.Error); err != nil {
			return nil, err
		}
		result.RedirectURL = wxRsp.Response.H5Url
		result.Raw = wxRsp.Response
	}
	return result, nil
}

// PayNotify 付款回调处理
// 文档: https://pay.weixin.qq.com/docs/merchant/apis/jsapi-payment/payment-notify.html
func (a *Wechat) PayNotify(ctx echo.Context) error {
	notifyReq, err := a.parseNotify(ctx)
	if err != nil {
		return a.notifyFail(ctx, err.Error())
	}
	if notifyReq.EventType != `TRANSACTION.SUCCESS` { // 仅处理支付成功通知
		return a.notifyOK(ctx)
	}
	payResult, err := notifyReq.DecryptPayCipherText(a.getAPIv3Key())
	if err != nil {
		return a.notifyFail(ctx, err.Error())
	}
	if a.notifyCallback != nil {
		var amount int64
		var currency string
		if payResult.Amount != nil {
			amount = int64(payResult.Amount.Total)
			currency = payResult.Amount.Currency
		}
		result := &config.Result{
			Operation:      config.OperationPayment,
			Status:         config.TradeStatusSuccess,
			TradeNo:        payResult.TransactionId,
			OutTradeNo:     payResult.OutTradeNo,
			Currency:       currency,
			PassbackParams: payResult.Attach,
			TotalAmount:    payment.Round(float64(amount)/100, 2),
			Raw:            payResult,
		}
		if err := a.notifyCallback(ctx, result); err != nil {
			return a.notifyFail(ctx, err.Error())
		}
	}
	return a.notifyOK(ctx)
}

func (a *Wechat) PayQuery(ctx echo.Context, cfg *config.Query) (*config.Result, error) {
	client, err := a.Client()
	if err != nil {
		return nil, err
	}
	orderNo := cfg.OutTradeNo
	orderNoType := wechatv3.OutTradeNo
	if len(cfg.TradeNo) > 0 {
		orderNo = cfg.TradeNo
		orderNoType = wechatv3.TransactionId
	}
	// 文档: https://pay.weixin.qq.com/docs/merchant/apis/jsapi-payment/query-by-out-trade-no.html
	wxRsp, err := client.V3TransactionQueryOrder(ctx, orderNoType, orderNo)
	if err != nil {
		return nil, err
	}
	if err = checkV3Rsp(wxRsp.Code, &wxRsp.ErrResponse, wxRsp.Error); err != nil {
		return nil, err
	}
	resp := wxRsp.Response
	var amount int64
	var currency string
	if resp.Amount != nil {
		amount = int64(resp.Amount.Total)
		currency = resp.Amount.Currency
	}
	return &config.Result{
		Operation:      config.OperationPayment,
		Status:         MappingTradeState(resp.TradeState),
		TradeNo:        resp.TransactionId,
		OutTradeNo:     resp.OutTradeNo,
		Currency:       currency,
		PassbackParams: resp.Attach,
		TotalAmount:    payment.Round(float64(amount)/100, 2),
		Reason:         resp.TradeStateDesc,
		Raw:            resp,
	}, nil
}

func (a *Wechat) Refund(ctx echo.Context, cfg *config.Refund) (*config.Result, error) {
	client, err := a.Client()
	if err != nil {
		return nil, err
	}
	bm := gopay.BodyMap{}
	if len(cfg.TradeNo) > 0 {
		bm.Set("transaction_id", cfg.TradeNo)
	} else {
		bm.Set("out_trade_no", cfg.OutTradeNo)
	}
	if len(cfg.OutRefundNo) == 0 {
		return nil, errors.New(`outRefundNo required`)
	}
	bm.Set("out_refund_no", cfg.OutRefundNo)
	if len(cfg.RefundReason) > 0 {
		bm.Set("reason", cfg.RefundReason)
	}
	if len(cfg.NotifyURL) > 0 {
		bm.Set("notify_url", cfg.NotifyURL)
	}
	bm.SetBodyMap("amount", func(b gopay.BodyMap) {
		b.Set("refund", MoneyFeeToInt64(cfg.RefundAmount))
		b.Set("total", MoneyFeeToInt64(cfg.TotalAmount))
		b.Set("currency", `CNY`)
	})
	// 文档: https://pay.weixin.qq.com/docs/merchant/apis/jsapi-payment/refund.html
	wxRsp, err := client.V3Refund(ctx, bm)
	if err != nil {
		return nil, err
	}
	if err = checkV3Rsp(wxRsp.Code, &wxRsp.ErrResponse, wxRsp.Error); err != nil {
		return nil, err
	}
	resp := wxRsp.Response
	var refundFee, total int64
	if resp.Amount != nil {
		refundFee = int64(resp.Amount.Refund)
		total = int64(resp.Amount.Total)
	}
	return &config.Result{
		Operation:   config.OperationRefund,
		Status:      MappingRefundStatus(resp.Status),
		TradeNo:     resp.TransactionId,
		OutTradeNo:  resp.OutTradeNo,
		TotalAmount: payment.Round(float64(total)/100, 2),
		RefundFee:   payment.Round(float64(refundFee)/100, 2),
		RefundNo:    resp.RefundId,
		OutRefundNo: resp.OutRefundNo,
		Raw:         resp,
	}, nil
}

// RefundNotify 退款回调处理
// 文档: https://pay.weixin.qq.com/docs/merchant/apis/jsapi-payment/refund-notify.html
// 事件类型: REFUND.SUCCESS(退款成功) / REFUND.ABNORMAL(退款异常) / REFUND.CLOSED(退款关闭) / REFUND.PROCESSING(退款处理中)
func (a *Wechat) RefundNotify(ctx echo.Context) error {
	notifyReq, err := a.parseNotify(ctx)
	if err != nil {
		return a.notifyFail(ctx, err.Error())
	}
	var status string
	switch notifyReq.EventType {
	case `REFUND.SUCCESS`:
		status = config.TradeStatusSuccess
	case `REFUND.ABNORMAL`:
		status = config.TradeStatusException
	case `REFUND.CLOSED`:
		status = config.TradeStatusClosed
	default: // REFUND.PROCESSING 等
		status = config.TradeStatusProcessing
	}
	refundResult, err := notifyReq.DecryptRefundCipherText(a.getAPIv3Key())
	if err != nil {
		return a.notifyFail(ctx, err.Error())
	}
	if a.notifyCallback != nil {
		var refundFee, total int64
		if refundResult.Amount != nil {
			refundFee = int64(refundResult.Amount.Refund)
			total = int64(refundResult.Amount.Total)
		}
		result := &config.Result{
			Operation:   config.OperationRefund,
			Status:      status,
			TradeNo:     refundResult.TransactionId,
			OutTradeNo:  refundResult.OutTradeNo,
			TotalAmount: payment.Round(float64(total)/100, 2),
			RefundFee:   payment.Round(float64(refundFee)/100, 2),
			RefundNo:    refundResult.RefundId,
			OutRefundNo: refundResult.OutRefundNo,
			Raw:         refundResult,
		}
		if err := a.notifyCallback(ctx, result); err != nil {
			return a.notifyFail(ctx, err.Error())
		}
	}
	return a.notifyOK(ctx)
}

// RefundQuery 退款查询（V3仅支持通过商户退款单号查询）
// 文档: https://pay.weixin.qq.com/docs/merchant/apis/jsapi-payment/refund-query.html
func (a *Wechat) RefundQuery(ctx echo.Context, cfg *config.Query) (*config.Result, error) {
	client, err := a.Client()
	if err != nil {
		return nil, err
	}
	if len(cfg.OutRefundNo) == 0 {
		return nil, errors.New(`outRefundNo required`)
	}
	wxRsp, err := client.V3RefundQuery(ctx, cfg.OutRefundNo, nil)
	if err != nil {
		return nil, err
	}
	if err = checkV3Rsp(wxRsp.Code, &wxRsp.ErrResponse, wxRsp.Error); err != nil {
		return nil, err
	}
	resp := wxRsp.Response
	var refundFee, total int64
	var currency string
	if resp.Amount != nil {
		refundFee = int64(resp.Amount.Refund)
		total = int64(resp.Amount.Total)
		currency = resp.Amount.Currency
	}
	return &config.Result{
		Operation:   config.OperationRefund,
		Status:      MappingRefundStatus(resp.Status),
		TradeNo:     resp.TransactionId,
		OutTradeNo:  resp.OutTradeNo,
		Currency:    currency,
		TotalAmount: payment.Round(float64(total)/100, 2),
		RefundFee:   payment.Round(float64(refundFee)/100, 2),
		RefundNo:    resp.RefundId,
		OutRefundNo: resp.OutRefundNo,
		Raw:         resp,
	}, nil
}

// VerifySign 验证回调签名
func (a *Wechat) VerifySign(ctx echo.Context) error {
	_, err := a.parseNotify(ctx)
	return err
}

// parseNotify 解析并验签回调通知
func (a *Wechat) parseNotify(ctx echo.Context) (*wechatv3.V3NotifyReq, error) {
	client, err := a.Client()
	if err != nil {
		return nil, err
	}
	notifyReq, err := wechatv3.V3ParseNotify(ctx.Request().StdRequest())
	if err != nil {
		return nil, err
	}
	// 验签（使用自动下载的微信平台证书公钥）
	if err = notifyReq.VerifySignByPKMap(client.WxPublicKeyMap()); err != nil {
		return nil, config.ErrSignature
	}
	return notifyReq, nil
}

func (a *Wechat) notifyOK(ctx echo.Context) error {
	return ctx.JSON(echo.H{"code": `SUCCESS`, "message": `成功`})
}

func (a *Wechat) notifyFail(ctx echo.Context, msg string) error {
	return ctx.JSON(echo.H{"code": `FAIL`, "message": msg})
}
