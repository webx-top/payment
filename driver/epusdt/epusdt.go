package epusdt

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/admpub/resty/v2"
	"github.com/webx-top/com"
	"github.com/webx-top/echo"
	"github.com/webx-top/payment"
	"github.com/webx-top/payment/config"
	"github.com/webx-top/restyclient"
)

const Name = `epusdt`

var (
	URLCreateOrder = `/api/v1/order/create-transaction`
	URLQueryOrder  = `/api/v1/order/query-transaction`
	URLQueryNet    = `/api/v1/order/query-networks`
)

var supports = config.Supports{
	config.SupportPayNotify,
	config.SupportPayQuery,
}

func init() {
	payment.Register(Name, `USDT`, New, SetDefaults)
}

func New() payment.Driver {
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

func (a *EPUSDT) SetNotifyCallback(callback func(echo.Context) error) payment.Driver {
	a.notifyCallback = callback
	return a
}

func (a *EPUSDT) SetAccount(account *config.Account) payment.Driver {
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
		Timestamp:   time.Now().Unix(),
		Nonce:       payment.GenerateNonce(),
	}
	if len(cfg.Subtype) > 0 {
		order.TradeType = cfg.Subtype
	}
	data := order.URLValues()
	order.Signature = GenerateSign(data, a.account.AppSecret)
	trade := &CreateTransactionResponse{}
	recv := &Response{
		Data: trade,
	}
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
		TradeNo:        trade.TradeId,
		RedirectURL:    trade.PaymentUrl,
		QRCodeImageURL: ``,
		//QRCodeContent:  trade.Token,
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
	query := QueryTransactionRequest{
		TradeId:   cfg.TradeNo,
		Timestamp: time.Now().Unix(),
		Nonce:     payment.GenerateNonce(),
	}
	data := query.URLValues()
	query.Signature = GenerateSign(data, a.account.AppSecret)
	queryResult := &QueryTransactionResponse{}
	recv := &Response{
		Data: queryResult,
	}
	resp, err := a.Client().SetResult(recv).SetBody(query).Post(a.apiURL + URLQueryOrder)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode() != http.StatusOK {
		return nil, fmt.Errorf("%s: %s", resp.Status(), com.StripTags(resp.String()))
	}
	if recv.StatusCode != http.StatusOK {
		return nil, errors.New(recv.Message)
	}
	result := &config.Result{
		Operation:   config.OperationPayment,
		TradeNo:     cfg.TradeNo,
		OutTradeNo:  cfg.OutTradeNo,
		Currency:    queryResult.Currency,
		TotalAmount: queryResult.Amount,
		PayCurrency: queryResult.ActualCurrency,
		PayAmount:   queryResult.ActualAmount,
		Reason:      ``,
		Raw:         recv,
	}
	MappingStatus(queryResult.Status, result)
	return result, err
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
