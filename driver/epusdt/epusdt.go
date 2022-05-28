package epusdt

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/admpub/resty/v2"
	"github.com/webx-top/com"
	"github.com/webx-top/echo"
	"github.com/webx-top/payment"
	"github.com/webx-top/payment/config"
	"github.com/webx-top/restyclient"
)

const Name = `epusdt`

var URLCreateOrder = `/api/v1/order/create-transaction`

var supports = config.Supports{
	config.SupportPayNotify,
}

func init() {
	payment.Register(Name, `USDT`, New)
}

func New() payment.Hook {
	return &EPUSDT{}
}

type EPUSDT struct {
	account        *config.Account
	notifyCallback func(echo.Context) error
	apiURL         string
}

func (a *EPUSDT) IsSupported(s config.Support) bool {
	return supports.IsSupported(s)
}

func (a *EPUSDT) SetNotifyCallback(callback func(echo.Context) error) payment.Hook {
	a.notifyCallback = callback
	return a
}

func (a *EPUSDT) SetAccount(account *config.Account) payment.Hook {
	a.account = account
	a.apiURL = strings.TrimSuffix(account.Options.Extra.String(`apiURL`), `/`)
	return a
}

func (a *EPUSDT) Client() *resty.Request {
	return restyclient.Retryable()
}

// Pay documentation
func (a *EPUSDT) Pay(ctx echo.Context, cfg *config.Pay) (*config.PayResponse, error) {
	order := CreateTransactionRequest{
		OrderId:     cfg.OutTradeNo,
		Amount:      cfg.Amount,
		NotifyUrl:   cfg.NotifyURL,
		RedirectUrl: cfg.ReturnURL,
	}
	data := order.URLValues()
	order.Signature = GenerateSign(data, a.account.AppSecret)
	recv := &Response{}
	resp, err := a.Client().SetResult(recv).SetBody(order).Post(a.apiURL + URLCreateOrder)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode() != http.StatusOK {
		return nil, fmt.Errorf("%s: %s", resp.Status(), com.StripTags(resp.String()))
	}
	if recv.StatusCode != http.StatusOK {
		if recv.StatusCode == 10002 {
			return nil, config.ErrTradeAlreadyExists
		}
		return nil, errors.New(recv.Message)
	}
	result := &config.PayResponse{
		TradeNo:        recv.Data.TradeId,
		RedirectURL:    recv.Data.PaymentUrl,
		QRCodeImageURL: ``,
		//QRCodeContent:  recv.Data.Token,
		Params: echo.H{},
		Raw:    recv,
	}
	return result, nil
}

// PayNotify 付款回调处理
func (a *EPUSDT) PayNotify(ctx echo.Context) error {
	callback := &OrderNotifyResponse{}
	err := ctx.MustBind(callback)
	if err != nil {
		return err
	}
	if !callback.Verify(a.account.AppSecret) {
		return ctx.String(`error-signature`)
	}
	//这里处理支付成功回调，一般是修改数据库订单信息等等
	if a.notifyCallback != nil && callback.Status == StatusPaid {
		result := &config.Result{
			Operation:   config.OperationPayment,
			Status:      config.TradeStatusSuccess,
			TradeNo:     callback.TradeId,
			OutTradeNo:  callback.OrderId,
			Currency:    `CNY`,
			TotalAmount: callback.Amount,
			PayCurrency: `USDT`,
			PayAmount:   callback.ActualAmount,
			Reason:      ``,
			Raw:         callback,
		}
		ctx.Set(`notify`, result)
		a.notifyCallback(ctx)
	}
	return ctx.String(`ok`)
}

// PayQuery documentation
func (a *EPUSDT) PayQuery(ctx echo.Context, cfg *config.Query) (*config.Result, error) {
	return nil, config.ErrUnsupported
}

// Refund 退款
func (a *EPUSDT) Refund(ctx echo.Context, cfg *config.Refund) (*config.Result, error) {
	return nil, config.ErrUnsupported
}

// RefundNotify 退款回调处理
func (a *EPUSDT) RefundNotify(ctx echo.Context) error {
	return config.ErrUnsupported
}

// RefundQuery 退款查询
func (a *EPUSDT) RefundQuery(ctx echo.Context, cfg *config.Query) (*config.Result, error) {
	return nil, config.ErrUnsupported
}
