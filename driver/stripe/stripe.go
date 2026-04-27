package stripe

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/stripe/stripe-go/v81"
	"github.com/stripe/stripe-go/v81/checkout/session"
	"github.com/stripe/stripe-go/v81/paymentintent"
	"github.com/stripe/stripe-go/v81/refund"
	"github.com/webx-top/echo"
	"github.com/webx-top/echo/param"
	"github.com/webx-top/payment"
	"github.com/webx-top/payment/config"
)

const Name = `stripe`

var supports = config.Supports{
	config.SupportPayNotify,
	config.SupportPayQuery,
	config.SupportRefund,
	config.SupportRefundNotify,
	config.SupportRefundQuery,
}

var SupportedCurrencies = []string{
	"USD", "EUR", "GBP", "AUD", "CAD", "CHF", "CNY",
	"DKK", "HKD", "JPY", "MXN", "NOK", "NZD", "SEK", "SGD",
}

func init() {
	payment.Register(Name, echo.T(`Stripe`), New)
}

func New() payment.Driver {
	return &Stripe{}
}

type Stripe struct {
	account        *config.Account
	notifyCallback payment.NotifyCallback
	paymentClient  paymentintent.Client
	refundClient   refund.Client
	sessionClient  session.Client
}

// initClients 初始化API客户端实例（避免使用全局变量 stripe.Key）
func (a *Stripe) initClients() {
	backend := stripe.GetBackend(stripe.APIBackend)
	a.paymentClient = paymentintent.Client{B: backend, Key: a.account.AppSecret}
	a.refundClient = refund.Client{B: backend, Key: a.account.AppSecret}
	a.sessionClient = session.Client{B: backend, Key: a.account.AppSecret}
}

func (a *Stripe) IsSupported(s config.Support) bool {
	return supports.IsSupported(s)
}

func (a *Stripe) SetNotifyCallback(callback payment.NotifyCallback) payment.Driver {
	a.notifyCallback = callback
	return a
}

func (a *Stripe) SetAccount(account *config.Account) payment.Driver {
	a.account = account
	return a
}

// Pay 创建Stripe Checkout Session，返回跳转URL
// 文档: https://stripe.com/docs/api/checkout/sessions/create
// 使用 Checkout Session 模式，用户会被重定向到 Stripe 托管的付款页面完成支付
func (a *Stripe) Pay(ctx echo.Context, cfg *config.Pay) (*config.PayResponse, error) {
	a.initClients()

	params := &stripe.CheckoutSessionParams{
		Mode: stripe.String(string(stripe.CheckoutSessionModePayment)),
		LineItems: []*stripe.CheckoutSessionLineItemParams{
			{
				PriceData: &stripe.CheckoutSessionLineItemPriceDataParams{
					Currency: stripe.String(CurrencyToLower(cfg.Currency.String())),
					ProductData: &stripe.CheckoutSessionLineItemPriceDataProductDataParams{
						Name: stripe.String(cfg.Subject),
					},
					UnitAmount: stripe.Int64(AmountToCent(cfg.Amount, cfg.Currency.String())),
				},
				Quantity: stripe.Int64(1),
			},
		},
		Metadata: map[string]string{
			"out_trade_no": cfg.OutTradeNo,
		},
	}

	// cfg.NotifyURL 需要在 Stripe 的 Dashboard 配置

	if len(cfg.ReturnURL) > 0 {
		params.SuccessURL = stripe.String(cfg.ReturnURL)
	}
	if len(cfg.CancelURL) > 0 {
		params.CancelURL = stripe.String(cfg.CancelURL)
	}
	if len(cfg.Subject) > 0 {
		params.LineItems[0].PriceData.ProductData.Description = stripe.String(cfg.Subject)
	}

	sess, err := a.sessionClient.New(params)
	if err != nil {
		return nil, err
	}

	result := &config.PayResponse{
		TradeNo:     sess.PaymentIntent.ID,
		RedirectURL: sess.URL,
		Raw:         sess,
	}
	return result, nil
}

// PayNotify 处理Stripe Webhook通知
// 文档: https://stripe.com/docs/webhooks
func (a *Stripe) PayNotify(ctx echo.Context) error {
	body, err := io.ReadAll(ctx.Request().Body())
	if err != nil {
		return err
	}
	event, err := a.verifySignAndGetEvent(ctx, body)
	if err != nil {
		return ctx.JSON(echo.H{"error": "invalid signature"})
	}

	switch event.Type {
	case stripe.EventTypePaymentIntentSucceeded:
		var pi stripe.PaymentIntent
		err := json.Unmarshal(event.Data.Raw, &pi)
		if err != nil {
			return err
		}
		if a.notifyCallback != nil && pi.Status == stripe.PaymentIntentStatusSucceeded {
			result := &config.Result{
				Operation:      config.OperationPayment,
				Status:         config.TradeStatusSuccess,
				TradeNo:        pi.ID,
				OutTradeNo:     pi.Metadata["out_trade_no"],
				Currency:       string(pi.Currency),
				TotalAmount:    CentToAmount(pi.Amount, string(pi.Currency)),
				PassbackParams: pi.Metadata["passback_params"],
				Raw:            &pi,
			}
			if err := a.notifyCallback(ctx, result); err != nil {
				return err
			}
		}
	case stripe.EventTypePaymentIntentPaymentFailed:
		var pi stripe.PaymentIntent
		err := json.Unmarshal(event.Data.Raw, &pi)
		if err != nil {
			return err
		}
		if a.notifyCallback != nil {
			result := &config.Result{
				Operation:      config.OperationPayment,
				Status:         config.TradeStatusClosed,
				TradeNo:        pi.ID,
				OutTradeNo:     pi.Metadata["out_trade_no"],
				Currency:       string(pi.Currency),
				TotalAmount:    CentToAmount(pi.Amount, string(pi.Currency)),
				Reason:         fmt.Sprintf("payment failed: %v", pi.LastPaymentError),
				PassbackParams: pi.Metadata["passback_params"],
				Raw:            &pi,
			}
			if err := a.notifyCallback(ctx, result); err != nil {
				return err
			}
		}

	}

	return ctx.JSON(echo.H{"status": "ok"})
}

// PayQuery 查询支付意图状态
// 文档: https://stripe.com/docs/api/payment_intents/retrieve
func (a *Stripe) PayQuery(ctx echo.Context, cfg *config.Query) (*config.Result, error) {
	a.initClients()

	pi, err := a.paymentClient.Get(cfg.TradeNo, nil)
	if err != nil {
		return nil, err
	}

	status := MappingStatus(string(pi.Status))

	return &config.Result{
		Operation:      config.OperationPayment,
		Status:         status,
		TradeNo:        pi.ID,
		OutTradeNo:     pi.Metadata["out_trade_no"],
		Currency:       string(pi.Currency),
		TotalAmount:    CentToAmount(pi.Amount, string(pi.Currency)),
		PassbackParams: pi.Metadata["passback_params"],
		Raw:            pi,
	}, nil
}

// Refund 发起退款
// 文档: https://stripe.com/docs/api/refunds/create
func (a *Stripe) Refund(ctx echo.Context, cfg *config.Refund) (*config.Result, error) {
	a.initClients()

	params := &stripe.RefundParams{
		PaymentIntent: stripe.String(cfg.TradeNo),
		Amount:        stripe.Int64(AmountToCent(cfg.RefundAmount, cfg.Currency.String())),
		Metadata: map[string]string{
			"out_refund_no": cfg.OutRefundNo,
			"out_trade_no":  cfg.OutTradeNo,
		},
	}
	if len(cfg.RefundReason) > 0 {
		params.Reason = stripe.String(string(stripe.RefundReasonRequestedByCustomer))
	}

	ref, err := a.refundClient.New(params)
	if err != nil {
		return nil, err
	}

	status := MapRefundStatus(string(ref.Status))

	feeValue := float64(0)
	feeCurrency := ""
	if ref.BalanceTransaction != nil {
		feeValue = param.AsFloat64(ref.BalanceTransaction.Fee)
		feeCurrency = string(ref.BalanceTransaction.Currency)
	}

	return &config.Result{
		Operation:              config.OperationRefund,
		Status:                 status,
		TradeNo:                ref.PaymentIntent.ID,
		OutTradeNo:             cfg.OutTradeNo,
		Currency:               cfg.Currency.String(),
		RefundFee:              CentToAmount(ref.Amount, cfg.Currency.String()),
		RefundNo:               ref.ID,
		OutRefundNo:            cfg.OutRefundNo,
		Reason:                 string(ref.Reason),
		TransactionFeeValue:    feeValue,
		TransactionFeeCurrency: feeCurrency,
		Raw:                    ref,
	}, nil
}

// RefundNotify 处理Stripe退款Webhook通知
// 文档: https://stripe.com/docs/webhooks
// 仅处理 charge.refunded 事件
func (a *Stripe) RefundNotify(ctx echo.Context) error {
	body, err := io.ReadAll(ctx.Request().Body())
	if err != nil {
		return err
	}
	event, err := a.verifySignAndGetEvent(ctx, body)
	if err != nil {
		return ctx.JSON(echo.H{"error": "invalid signature"})
	}

	switch event.Type {
	case stripe.EventTypeChargeRefunded:
		var ch stripe.Charge
		err := json.Unmarshal(event.Data.Raw, &ch)
		if err != nil {
			return err
		}
		if a.notifyCallback != nil && len(ch.Refunds.Data) > 0 {
			refundObj := ch.Refunds.Data[0]
			result := &config.Result{
				Operation:      config.OperationRefund,
				Status:         config.TradeStatusSuccess,
				TradeNo:        ch.PaymentIntent.ID,
				OutTradeNo:     ch.Metadata["out_trade_no"],
				Currency:       string(ch.Currency),
				RefundFee:      CentToAmount(refundObj.Amount, string(ch.Currency)),
				RefundNo:       refundObj.ID,
				OutRefundNo:    refundObj.Metadata["out_refund_no"],
				PassbackParams: ch.Metadata["passback_params"],
				Raw:            &ch,
			}
			if err := a.notifyCallback(ctx, result); err != nil {
				return err
			}
		}
	}

	return ctx.JSON(echo.H{"status": "ok"})
}

// RefundQuery 查询退款状态
// 文档: https://stripe.com/docs/api/refunds/retrieve
func (a *Stripe) RefundQuery(ctx echo.Context, cfg *config.Query) (*config.Result, error) {
	a.initClients()

	ref, err := a.refundClient.Get(cfg.RefundNo, nil)
	if err != nil {
		return nil, err
	}

	status := MapRefundStatus(string(ref.Status))

	feeValue := float64(0)
	feeCurrency := ""
	if ref.BalanceTransaction != nil {
		feeValue = param.AsFloat64(ref.BalanceTransaction.Fee)
		feeCurrency = string(ref.BalanceTransaction.Currency)
	}

	return &config.Result{
		Operation:              config.OperationRefund,
		Status:                 status,
		TradeNo:                ref.PaymentIntent.ID,
		OutTradeNo:             ref.Metadata["out_trade_no"],
		Currency:               string(ref.Currency),
		RefundFee:              CentToAmount(ref.Amount, string(ref.Currency)),
		RefundNo:               ref.ID,
		OutRefundNo:            ref.Metadata["out_refund_no"],
		Reason:                 string(ref.Reason),
		TransactionFeeValue:    feeValue,
		TransactionFeeCurrency: feeCurrency,
		Raw:                    ref,
	}, nil
}
