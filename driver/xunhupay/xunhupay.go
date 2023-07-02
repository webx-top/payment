package xunhupay

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/admpub/resty/v2"
	"github.com/webx-top/com"
	"github.com/webx-top/com/encoding/json"
	"github.com/webx-top/echo"
	"github.com/webx-top/echo/param"
	"github.com/webx-top/payment"
	"github.com/webx-top/payment/config"
	"github.com/webx-top/restyclient"
)

const Name = `xunhupay`

var (
	SandboxURL    = `https://api.diypc.com.cn` // 测试环境接口
	ProductionURL = `https://api.xunhupay.com` // 生产环境接口
	APIPay        = `/payment/do.html`
	APIQuery      = `/payment/query.html`
)

var supports = config.Supports{
	config.SupportPayNotify,
	config.SupportPayQuery,
}

func init() {
	payment.Register(Name, `虎皮椒支付`, New, SetDefaults)
}

func SetDefaults(a *config.Account) {
	if a.Subtype == nil {
		a.Subtype = config.NewSubtype(
			`支付类型`,
		)
	}
	if len(a.Subtype.Options) == 0 {
		a.Subtype.Add(
			&config.SubtypeOption{
				Value: `alipay`, Text: `支付宝付款`, Checked: true,
			},
			&config.SubtypeOption{
				Value: `wechat`, Text: `微信付款`,
			},
		)
	}
}

func New() payment.Driver {
	return &XunHuPay{}
}

type XunHuPay struct {
	account        *config.Account
	notifyCallback func(echo.Context) error
}

func (a *XunHuPay) IsSupported(s config.Support) bool {
	return supports.IsSupported(s)
}

func (a *XunHuPay) SetNotifyCallback(callback func(echo.Context) error) payment.Driver {
	a.notifyCallback = callback
	return a
}

func (a *XunHuPay) SetAccount(account *config.Account) payment.Driver {
	a.account = account
	return a
}

func (a *XunHuPay) Client() *resty.Request {
	return restyclient.Retryable()
}

func (a *XunHuPay) generateURL(endpoint string) string {
	if a.account.Debug {
		return SandboxURL + endpoint
	}
	return ProductionURL + endpoint
}

func (a *XunHuPay) parseAppID(subtype string) (appID string) {
	// alipay=appID;wechat=appID
	return a.account.ParseAppID(subtype)
}

func (a *XunHuPay) parseAppSecret(subtype string) (appSecret string) {
	// alipay=appSecret;wechat=appSecret
	return a.account.ParseAppSecret(subtype)
}

func (a *XunHuPay) Pay(ctx echo.Context, cfg *config.Pay) (*config.PayResponse, error) {
	ts := time.Now().Unix()
	tss := fmt.Sprint(ts)
	title := cfg.Subject
	if len(cfg.Subject) == 0 {
		title = `NO.` + cfg.OutTradeNo
	}
	appID := a.parseAppID(cfg.Subtype)
	if len(appID) == 0 {
		return nil, config.ErrAppIDRequired
	}
	appSecret := a.parseAppSecret(cfg.Subtype)
	if len(appSecret) == 0 {
		return nil, config.ErrAppSecretRequired
	}
	if len(cfg.Subtype) == 0 {
		return nil, config.ErrSubtypeRequired
	}

	// documentation https://www.xunhupay.com/doc/api/pay.html
	data := url.Values{
		`version`:        []string{`1.1`},
		`appid`:          []string{appID},
		`trade_order_id`: []string{cfg.OutTradeNo},         //订单编号
		`total_fee`:      []string{fmt.Sprint(cfg.Amount)}, // 订单金额(元)，单位为人民币，精确到分
		`title`:          []string{title},
		`time`:           []string{tss},
		`notify_url`:     []string{cfg.NotifyURL}, //异步回调地址，用户支付成功后，我们服务器会主动发送一个post消息到这个网址(注意：当前接口内，SESSION内容无效)
		`return_url`:     []string{cfg.ReturnURL}, //用户支付成功后，我们会让用户浏览器自动跳转到这个网址
		`callback_url`:   []string{cfg.CancelURL}, //用户取消支付后，我们可能引导用户跳转到这个网址上重新进行支付
		`nonce_str`:      []string{com.Md5(tss)},
		`plugins`:        []string{cfg.Subtype},        // 名称，用于识别对接程序或作者
		`attach`:         []string{cfg.PassbackParams}, //备注字段，可以传入一些备注数据，回调时原样返回
		//`type`:           []string{`WAP`},            //支付通道类型，H5支付固定值"WAP"，小程序支付固定值"JSAPI" （支付宝不需要此参数）
	}
	if cfg.Subtype == `wechat` {
		switch cfg.Device {
		case config.Wap:
			data.Set(`type`, `WAP`)
		//case config.Web:
		//case config.App:
		default:
			if strings.Contains(ctx.Request().UserAgent(), `MicroMessenger`) {
				data.Set(`type`, `JSAPI`)
			}
		}
	}
	if len(cfg.PassbackParams) > 0 {
		data.Set(`attach`, cfg.PassbackParams)
	}
	if a.account.Options.Extra != nil {
		payConfig := a.account.Options.Extra.GetStore(`payConfig`)
		for k, v := range payConfig {
			data.Set(k, param.AsString(v))
		}
	}
	if cfg.Options != nil {
		typ := cfg.Options.String(`type`)
		if len(typ) > 0 {
			data.Set(`type`, typ)
		}
	}
	data.Set(`hash`, GenerateHash(data, appSecret))
	apiURL := APIPay
	recv := echo.H{}
	apiURL = a.generateURL(apiURL)
	resp, err := a.Client().SetResult(&recv).SetFormDataFromValues(data).Post(apiURL)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode() != http.StatusOK {
		return nil, fmt.Errorf("%s: %s", resp.Status(), com.StripTags(resp.String()))
	}
	errcode := recv.Int(`errcode`)
	if errcode != 0 {
		return nil, fmt.Errorf("[%d] %s", errcode, recv.String(`errmsg`))
	}
	recvHash := recv.String(`hash`)
	formData := url.Values{}
	for k, v := range recv {
		formData.Set(k, param.AsString(v))
	}
	hashString := GenerateHash(formData, appSecret)
	if recvHash != hashString {
		return nil, config.ErrSignature
	}
	result := &config.PayResponse{
		TradeNo:        recv.String(`oderid`),
		QRCodeImageURL: recv.String(`url_qrcode`),
		RedirectURL:    recv.String(`url`),
		Params:         echo.H{},
		Raw:            recv,
	}
	jsapi := recv.String(`jsapi`)
	if len(jsapi) > 0 {
		err = json.Unmarshal(com.Str2bytes(jsapi), &result.Params)
		if err != nil {
			return result, err
		}
	}
	return result, nil
}

// PayNotify 付款回调处理
// documentation https://www.xunhupay.com/doc/api/pay.html
func (a *XunHuPay) PayNotify(ctx echo.Context) error {
	formData := url.Values(ctx.Forms())
	status := formData.Get(`status`)
	formHash := formData.Get(`hash`)
	subtype := formData.Get(`plugins`)
	appSecret := a.parseAppSecret(subtype)
	hashString := GenerateHash(formData, appSecret)
	if formHash != hashString {
		return ctx.String(config.ErrSignature.Error())
	}

	var tradeStatus string
	switch status {
	case `OD`: // 支付成功
		tradeStatus = config.TradeStatusSuccess
	case `WP`: // 待支付
		tradeStatus = config.TradeStatusWaitBuyerPay
	case `CD`: // 已取消
		tradeStatus = config.TradeStatusClosed
	}
	if a.notifyCallback != nil {
		result := &config.Result{
			Operation:      config.OperationPayment,
			Status:         tradeStatus,
			TradeNo:        formData.Get(`open_order_id`) + `|` + formData.Get(`transaction_id`),
			OutTradeNo:     formData.Get(`trade_order_id`),
			Currency:       ``,
			PassbackParams: formData.Get(`attach`),
			TotalAmount:    param.AsFloat64(formData.Get(`total_fee`)),
			Reason:         ``,
			Raw:            formData,
		}
		ctx.Set(`notify`, result)
		if err := a.notifyCallback(ctx); err != nil {
			return ctx.String(err.Error())
		}
	}

	return ctx.String(`success`)
}

func (a *XunHuPay) PayQuery(ctx echo.Context, cfg *config.Query) (*config.Result, error) {
	// documentation https://www.xunhupay.com/doc/api/search.html
	ts := time.Now().Unix()
	tss := fmt.Sprint(ts)
	formData := url.Values{
		`appid`:           []string{a.account.AppID},
		`out_trade_order`: []string{cfg.OutTradeNo}, //商户网站订单号. out_trade_order，open_order_id 二选一。请确保在您的网站内是唯一订单号
		//`open_order_id`: []string{strings.SplitN(cfg.TradeNo, `|`, 2)[0]}, //虎皮椒内部订单号. out_trade_order，open_order_id 二选一。在支付时，或支付成功时会返回此数据给商户网站y
		`time`:      []string{tss},
		`nonce_str`: []string{com.Md5(tss)},
	}
	if len(cfg.Subtype) == 0 {
		return nil, config.ErrSubtypeRequired
	}
	appSecret := a.parseAppSecret(cfg.Subtype)
	formData.Set(`hash`, GenerateHash(formData, appSecret))
	recv := echo.H{}
	apiURL := a.generateURL(APIQuery)
	resp, err := a.Client().SetResult(&recv).SetFormDataFromValues(formData).Post(apiURL)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode() != http.StatusOK {
		return nil, fmt.Errorf("%d %s:\n%s", resp.StatusCode(), resp.Status(), com.StripTags(resp.String()))
	}
	errcode := recv.Int(`errcode`)
	if errcode != 0 {
		return nil, fmt.Errorf("[%d] %s", errcode, recv.String(`errmsg`))
	}
	// com.Dump(recv)
	/* recv = {
	  "data": {
	    "open_order_id": "20201558372",
	    "out_trade_order": "RECHARGE168066492347187200",
	    "paid_date": "2021-11-03 18:39:04",
	    "pay_url": "weixin://wxpay/bizpayurl?pr=QpyXFy2zz",
	    "payment_method": "two-wechat",
	    "status": "OD",
	    "title": "账号充值",
	    "total_amount": "0.01",
	    "transaction_id": "4200001232202111037855405793"
	  },
	  "errcode": 0,
	  "errmsg": ""
	}
	*/
	data := recv.GetStore(`data`)
	var tradeStatus string
	switch data.String(`status`) {
	case `OD`: // 支付成功
		tradeStatus = config.TradeStatusSuccess
	case `WP`: // 待支付
		tradeStatus = config.TradeStatusWaitBuyerPay
	case `CD`: // 已取消
		tradeStatus = config.TradeStatusClosed
	}
	r := &config.Result{
		Operation: config.OperationPayment,
		Status:    tradeStatus,
		// TradeNo:        cfg.TradeNo,
		// OutTradeNo:     cfg.OutTradeNo,
		TradeNo:        data.String(`open_order_id`) + `|` + data.String(`transaction_id`),
		OutTradeNo:     data.String(`out_trade_order`),
		Currency:       ``,
		PassbackParams: ``,
		TotalAmount:    data.Float64(`total_amount`),
		Reason:         ``,
		Raw:            recv,
	}
	if len(r.TradeNo) == 0 {
		r.TradeNo = cfg.TradeNo
	}
	if len(r.OutTradeNo) == 0 {
		r.OutTradeNo = cfg.OutTradeNo
	}
	return r, err
}

func (a *XunHuPay) Refund(ctx echo.Context, cfg *config.Refund) (*config.Result, error) {
	return nil, config.ErrUnsupported
}

// RefundNotify 退款回调处理
// documentation 不支持
func (a *XunHuPay) RefundNotify(ctx echo.Context) error {
	return config.ErrUnsupported
}

// RefundQuery 退款查询
func (a *XunHuPay) RefundQuery(ctx echo.Context, cfg *config.Query) (*config.Result, error) {
	return nil, config.ErrUnsupported
}
