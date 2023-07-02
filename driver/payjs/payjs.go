package payjs

import (
	"strings"

	"github.com/qingwg/payjs"
	"github.com/qingwg/payjs/notify"
	"github.com/webx-top/echo"
	"github.com/webx-top/echo/param"
	"github.com/webx-top/payment"
	"github.com/webx-top/payment/config"
)

const Name = `payjs`

var supports = config.Supports{
	config.SupportPayNotify,
	config.SupportPayQuery,
	config.SupportRefund,
}

func init() {
	payment.Register(Name, `PayJS支付`, New)
}

func New() payment.Driver {
	return &PayJS{}
}

type PayJS struct {
	account        *config.Account
	client         *payjs.PayJS
	notifyCallback func(echo.Context) error
}

func (a *PayJS) IsSupported(s config.Support) bool {
	return supports.IsSupported(s)
}

func (a *PayJS) SetNotifyCallback(callback func(echo.Context) error) payment.Driver {
	a.notifyCallback = callback
	return a
}

func (a *PayJS) SetAccount(account *config.Account) payment.Driver {
	a.account = account
	return a
}

func (a *PayJS) Client() *payjs.PayJS {
	if a.client != nil {
		return a.client
	}
	payjsConfig := &payjs.Config{
		Key:       a.account.AppSecret,
		MchID:     a.account.MerchantID,
		NotifyUrl: ``,
	}
	a.client = payjs.New(payjsConfig)
	return a.client
}

func (a *PayJS) Pay(ctx echo.Context, cfg *config.Pay) (*config.PayResponse, error) {
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
	var result *config.PayResponse
	a.Client().Context.NotifyUrl = cfg.NotifyURL
	switch tradeType {
	case `JSAPI`:
		openID := cfg.Options.String(`openid`)
		if len(openID) == 0 && a.account.Options.Extra != nil {
			payConfig := a.account.Options.Extra.GetStore(`payConfig`)
			openID = payConfig.String(`openId`)
		}
		// documentation https://help.payjs.cn/api-lie-biao/jsapiyuan-sheng-zhi-fu.html
		jsapi := a.Client().GetJs()
		resp, err := jsapi.Create(param.AsInt64(MoneyFeeToString(cfg.Amount)), cfg.Subject, cfg.OutTradeNo, cfg.PassbackParams, openID)
		if err != nil {
			return nil, err
		}
		result = &config.PayResponse{
			Params: echo.H{},
			Raw:    resp,
		}
		result.Params[`appId`] = resp.JsApi.AppID
		result.Params[`timeStamp`] = resp.JsApi.TimeStamp
		result.Params[`nonceStr`] = resp.JsApi.NonceStr
		result.Params[`package`] = resp.JsApi.Package
		result.Params[`signType`] = resp.JsApi.SignType
		result.Params[`paySign`] = resp.JsApi.PaySign
	case `APP`, `MWEB`, `NATIVE`:
		fallthrough
	default:
		// documentation https://help.payjs.cn/api-lie-biao/shou-yin-tai-zhi-fu.html
		cashier := a.Client().GetCashier()
		resp, err := cashier.GetRequestUrl(param.AsInt64(MoneyFeeToString(cfg.Amount)), cfg.Subject, cfg.OutTradeNo, cfg.PassbackParams, cfg.ReturnURL, 1, 1)
		if err != nil {
			return nil, err
		}
		result = &config.PayResponse{
			RedirectURL: resp,
			Raw:         resp,
		}
	}
	return result, nil
}

// PayNotify 付款回调处理
// documentation https://help.payjs.cn/api-lie-biao/jiao-yi-xin-xi-tui-song.html
// TODO: 验证签名
func (a *PayJS) PayNotify(ctx echo.Context) error {
	payNotify := a.Client().GetNotify(ctx.Request().StdRequest(), ctx.Response().StdResponseWriter())

	var notifyCallbackErr error
	//设置接收消息的处理方法
	payNotify.SetMessageHandler(func(msg notify.Message) {
		//这里处理支付成功回调，一般是修改数据库订单信息等等
		//msg即为支付成功异步通知过来的内容
		if a.notifyCallback != nil {
			result := &config.Result{
				Operation:      config.OperationPayment,
				Status:         config.TradeStatusSuccess,
				TradeNo:        msg.PayJSOrderID + `|` + msg.TransactionID,
				OutTradeNo:     msg.OutTradeNo,
				Currency:       ``,
				PassbackParams: msg.Attach,
				TotalAmount:    param.AsFloat64(payment.CutFloat(float64(msg.TotalFee)/100, 2)),
				Reason:         ``,
				Raw:            msg,
			}
			ctx.Set(`notify`, result)
			notifyCallbackErr = a.notifyCallback(ctx)
		}
	})

	//处理消息接收以及回复
	err := payNotify.Serve()
	if err != nil {
		return err
	}
	if notifyCallbackErr != nil {
		return notifyCallbackErr
	}

	//发送回复的消息
	return payNotify.SendResponseMsg()
}

func (a *PayJS) PayQuery(ctx echo.Context, cfg *config.Query) (*config.Result, error) {
	// documentation https://help.payjs.cn/api-lie-biao/ding-dan-cha-xun.html
	order := a.Client().GetOrder()
	resp, err := order.Check(strings.SplitN(cfg.TradeNo, `|`, 2)[0])
	if err != nil {
		return nil, err
	}
	var tradeStatus string
	switch resp.Status {
	case 1:
		tradeStatus = config.TradeStatusSuccess
	case 0:
		tradeStatus = config.TradeStatusWaitBuyerPay
	}
	return &config.Result{
		Operation:      config.OperationPayment,
		Status:         tradeStatus,
		TradeNo:        resp.PayJSOrderID + `|` + resp.TransactionID,
		OutTradeNo:     resp.OutTradeNo,
		Currency:       ``,
		PassbackParams: resp.Attach,
		TotalAmount:    param.AsFloat64(payment.CutFloat(float64(resp.TotalFee)/100, 2)),
		Reason:         resp.ReturnMsg,
		Raw:            resp,
	}, err
}

func (a *PayJS) Refund(ctx echo.Context, cfg *config.Refund) (*config.Result, error) {
	// documentation https://help.payjs.cn/api-lie-biao/tui-kuan.html
	order := a.Client().GetOrder()
	resp, err := order.Refund(strings.SplitN(cfg.TradeNo, `|`, 2)[0])
	if err != nil {
		return nil, err
	}
	return &config.Result{
		Operation:   config.OperationRefund,
		Status:      config.TradeStatusSuccess,
		TradeNo:     resp.PayJSOrderID + `|` + resp.TransactionID,
		OutTradeNo:  resp.OutTradeNo,
		Currency:    ``,
		TotalAmount: cfg.TotalAmount,
		Reason:      resp.ReturnMsg,
		RefundFee:   cfg.RefundAmount,
		RefundNo:    resp.PayJSOrderID,
		OutRefundNo: cfg.OutRefundNo,
		Raw:         resp,
	}, err
}

// RefundNotify 退款回调处理
// documentation 不支持
func (a *PayJS) RefundNotify(ctx echo.Context) error {
	return config.ErrUnsupported
}

// RefundQuery 退款查询
func (a *PayJS) RefundQuery(ctx echo.Context, cfg *config.Query) (*config.Result, error) {
	return nil, config.ErrUnsupported
}
