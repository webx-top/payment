package mugglepay

import (
	"errors"
	"fmt"

	"github.com/admpub/mugglepay"
	"github.com/admpub/mugglepay/structs"
	"github.com/webx-top/echo"
	"github.com/webx-top/payment"
	"github.com/webx-top/payment/config"
)

const Name = `mugglepay`

func init() {
	payment.Register(Name, `麻瓜宝`, New)
}

func New() payment.Hook {
	return &Mugglepay{}
}

type Mugglepay struct {
	account        *config.Account
	client         *mugglepay.Mugglepay
	notifyCallback func(echo.Context) error
}

func (a *Mugglepay) SetNotifyCallback(callback func(echo.Context) error) payment.Hook {
	a.notifyCallback = callback
	return a
}

func (a *Mugglepay) SetAccount(account *config.Account) payment.Hook {
	a.account = account
	return a
}

func (a *Mugglepay) Client() *mugglepay.Mugglepay {
	if a.client != nil {
		return a.client
	}
	a.client = mugglepay.New(a.account.AppSecret)
	return a.client
}

// Pay documentation: https://github.com/MugglePay/MugglePay/blob/master/API/order/CreateOrder.md
func (a *Mugglepay) Pay(ctx echo.Context, cfg *config.Pay) (*config.PayResponse, error) {
	order := &structs.Order{
		MerchantOrderID: cfg.OutTradeNo,
		PriceAmount:     cfg.Amount,
		PriceCurrency:   cfg.Currency.String(), // "USD",
		PayCurrency:     "",                    // ALIPAY | WECHAT
		Title:           cfg.Subject,
		Description:     "",
		CallbackURL:     cfg.NotifyURL,
		SuccessURL:      cfg.ReturnURL,
		CancelURL:       cfg.CancelURL,
		Mobile:          cfg.Device.IsMobile(),
	}
	if cfg.Options != nil {
		order.PayCurrency = cfg.Options.String(`payCurrency`)
		order.Description = cfg.Options.String(`description`)
		order.Fast = cfg.Options.Bool(`fast`)
	}
	serverOrder, err := a.Client().CreateOrder(order)
	if err != nil {
		return nil, err
	}
	serverOrder.Parse()
	if serverOrder.Status != 200 && len(serverOrder.PaymentURL) == 0 {
		return nil, errors.New(serverOrder.ErrorCode + `: ` + serverOrder.Error)
	}
	result := &config.PayResponse{
		TradeNo:        serverOrder.Order.OrderID,
		RedirectURL:    serverOrder.PaymentURL,
		QRCodeImageURL: ``,
		QRCodeContent:  serverOrder.Invoice.Qrcode,
		Params:         echo.H{},
		Raw:            serverOrder,
	}
	return result, nil
}

// PayNotify 付款回调处理
// documentation https://github.com/MugglePay/MugglePay/blob/master/API/order/PaymentCallback.md
func (a *Mugglepay) PayNotify(ctx echo.Context) error {
	callback := &structs.Callback{}
	err := ctx.MustBind(callback)
	if err != nil {
		return err
	}
	if !a.Client().VerifyOrder(callback) {
		return ctx.JSON(echo.H{`status`: 400})
	}
	//这里处理支付成功回调，一般是修改数据库订单信息等等
	//msg即为支付成功异步通知过来的内容
	if a.notifyCallback != nil && callback.Status == `PAID` {
		result := &config.Result{
			Operation:   config.OperationPayment,
			Status:      config.TradeStatusSuccess,
			TradeNo:     callback.OrderID,
			OutTradeNo:  callback.MerchantOrderID,
			Currency:    callback.PayCurrency,
			TotalAmount: callback.PayAmount,
			PayCurrency: callback.PriceCurrency,
			PayAmount:   callback.PriceAmount,
			Reason:      ``,
			Raw:         callback,
		}
		ctx.Set(`notify`, result)
		a.notifyCallback(ctx)
	}
	return ctx.JSON(echo.H{`status`: 200})
}

// PayQuery documentation: https://github.com/MugglePay/MugglePay/blob/master/API/order/GetOrder.md
func (a *Mugglepay) PayQuery(ctx echo.Context, cfg *config.Query) (*config.Result, error) {
	serverOrder, err := a.Client().GetOrder(cfg.TradeNo)
	if err != nil {
		return nil, err
	}
	result := &config.Result{
		Operation:   config.OperationPayment,
		TradeNo:     serverOrder.Order.OrderID,
		OutTradeNo:  serverOrder.Order.MerchantOrderID,
		Currency:    serverOrder.Order.PayCurrency,
		TotalAmount: serverOrder.Order.PayAmount,
		PayCurrency: serverOrder.Order.PriceCurrency,
		PayAmount:   serverOrder.Order.PriceAmount,
		Reason:      ``,
		Raw:         serverOrder,
	}
	MappingStatus(serverOrder.Order.Status, result)
	return result, err
}

// Refund documentation https://github.com/MugglePay/MugglePay/blob/master/API/order/Refund.md
func (a *Mugglepay) Refund(ctx echo.Context, cfg *config.Refund) (*config.Result, error) {
	queryResult, err := a.PayQuery(ctx, config.NewQuery().CopyFromRefund(cfg))
	if err != nil {
		return nil, err
	}
	if queryResult.PayAmount != cfg.RefundAmount {
		return nil, fmt.Errorf("MugglePay只支持全额退款")
	}
	serverRefund, err := a.Client().Refund(cfg.TradeNo)
	if err != nil {
		return nil, err
	}
	if len(serverRefund.ErrorCode) > 0 {
		return nil, errors.New(serverRefund.ErrorCode + `: ` + serverRefund.Error)
	}
	result := &config.Result{
		Operation:   config.OperationRefund,
		TradeNo:     serverRefund.Order.OrderID,
		OutTradeNo:  serverRefund.Order.MerchantOrderID,
		Currency:    queryResult.Currency,
		TotalAmount: queryResult.TotalAmount,
		PayCurrency: serverRefund.Order.PriceCurrency,
		PayAmount:   serverRefund.Order.PriceAmount,
		Reason:      ``,
		RefundFee:   serverRefund.Order.PriceAmount,
		RefundNo:    ``,
		OutRefundNo: cfg.OutRefundNo,
		Raw:         serverRefund,
	}
	MappingStatus(serverRefund.Order.Status, result)
	return result, err
}

// RefundNotify 退款回调处理
func (a *Mugglepay) RefundNotify(ctx echo.Context) error {
	callback := &structs.Callback{}
	err := ctx.MustBind(callback)
	if err != nil {
		return err
	}
	if !a.Client().VerifyOrder(callback) {
		return ctx.JSON(echo.H{`status`: 400})
	}
	//这里处理支付成功回调，一般是修改数据库订单信息等等
	//msg即为支付成功异步通知过来的内容
	if a.notifyCallback != nil && callback.Status == `REFUNDED` {
		result := &config.Result{
			Operation:   config.OperationRefund,
			Status:      config.TradeStatusSuccess,
			TradeNo:     callback.OrderID,
			OutTradeNo:  callback.MerchantOrderID,
			Currency:    callback.PayCurrency,
			TotalAmount: callback.PayAmount,
			PayCurrency: callback.PriceCurrency,
			PayAmount:   callback.PriceAmount,
			Reason:      ``,
			Raw:         callback,
		}
		ctx.Set(`notify`, result)
		a.notifyCallback(ctx)
	}
	return ctx.JSON(echo.H{`status`: 200})
}

// RefundQuery 退款查询
func (a *Mugglepay) RefundQuery(ctx echo.Context, cfg *config.Query) (*config.Result, error) {
	return nil, config.ErrUnsupported
}
