package paypal

import (
	"github.com/smartwalle/paypal"
	"github.com/webx-top/echo"
	"github.com/webx-top/echo/param"
	"github.com/webx-top/payment"
	"github.com/webx-top/payment/config"
)

const Name = `paypal`

func init() {
	payment.Register(Name, `贝宝`, New)
}

func New() payment.Hook {
	return &Paypal{}
}

type Paypal struct {
	account        *config.Account
	client         *paypal.Client
	notifyCallback func(echo.Context) error
}

func (a *Paypal) SetNotifyCallback(callback func(echo.Context) error) payment.Hook {
	a.notifyCallback = callback
	return a
}

func (a *Paypal) SetAccount(account *config.Account) payment.Hook {
	a.account = account
	return a
}

func (a *Paypal) Client() *paypal.Client {
	if a.client != nil {
		return a.client
	}
	a.client = paypal.New(
		a.account.AppID,
		a.account.AppSecret,
		!a.account.Debug,
	)
	return a.client
}

func (a *Paypal) Pay(cfg *config.Pay) (param.StringMap, error) {
	result := param.StringMap{}
	var p = &paypal.Payment{}
	p.Intent = paypal.K_PAYMENT_INTENT_SALE
	p.Payer = &paypal.Payer{}
	p.Payer.PaymentMethod = paypal.K_PAYMENT_METHOD_PAYPAL
	p.RedirectURLs = &paypal.RedirectURLs{}
	p.RedirectURLs.CancelURL = cfg.ReturnURL
	p.RedirectURLs.ReturnURL = cfg.NotifyURL

	var transaction = &paypal.Transaction{}
	transaction.InvoiceNumber = cfg.OutTradeNo // 保存我方订单号
	p.Transactions = []*paypal.Transaction{transaction}

	transaction.Amount = &paypal.Amount{}
	transaction.Amount.Total = MoneyFeeToString(cfg.Amount)
	transaction.Amount.Currency = cfg.Currency.String()

	payment, err := a.Client().CreatePayment(p)
	if err != nil {
		return result, err
	}
	//com.Dump(payment)
	for _, link := range payment.Links {
		if link.Rel == `approval_url` {
			result[`url`] = param.String(link.Href)
			break
		}
	}
	return result, err
}

func (a *Paypal) Finish(ctx echo.Context) (param.StringMap, error) {
	paymentID := ctx.Form(`paymentId`)
	playerID := ctx.Form(`PayerID`)
	payment, err := a.Client().ExecuteApprovedPayment(paymentID, playerID)
	if err != nil {
		return nil, err
	}
	var (
		paid        bool
		tradeNo     string
		outOrderNo  string
		totalAmount string
	)
	if payment.State == paypal.K_PAYMENT_STATE_APPROVED {
		paid = true
	}
	for _, transaction := range payment.Transactions {
		for _, resource := range transaction.RelatedResources {
			tradeNo = resource.Sale.Id
			outOrderNo = resource.Sale.InvoiceNumber
			totalAmount = resource.Sale.Amount.Total
			if resource.Sale.State != paypal.K_SALE_STATE_COMPLETED {
				paid = false
				break
			}
		}
	}
	notify := param.StringMap{}
	if paid {
		notify[`paid`] = param.String(`1`)
	} else {
		notify[`paid`] = param.String(`0`)
	}

	notify[`trade_no`] = param.String(tradeNo)        // 作为交易流水号
	notify[`out_trade_no`] = param.String(outOrderNo) // 保存我方订单号
	notify[`total_amount`] = param.String(totalAmount)
	return notify, err
}

func (a *Paypal) Notify(ctx echo.Context) error {
	event, err := a.Client().GetWebhookEvent(a.account.WebhookID, ctx.Request().StdRequest())
	if err != nil {
		return err
	}
	if event == nil {
		return nil
	}
	switch event.EventType {
	case paypal.K_EVENT_TYPE_PAYMENT_SALE_COMPLETED:
		sale := event.Sale()
		notify := param.StringMap{}
		notify[`operation`] = `payment`
		notify[`trade_no`] = param.String(sale.Id)                // 作为交易流水号
		notify[`out_trade_no`] = param.String(sale.InvoiceNumber) // 保存我方订单号
		notify[`total_amount`] = param.String(sale.Amount.Total)
		if a.notifyCallback != nil {
			ctx.Set(`notify`, notify)
			if err := a.notifyCallback(ctx); err != nil {
				return err
			}
		}
	case paypal.K_EVENT_TYPE_PAYMENT_SALE_REFUNDED:
		refund := event.Refund()
		notify := param.StringMap{}
		notify[`operation`] = `refund`
		notify[`trade_no`] = param.String(refund.SaleId)
		notify[`out_trade_no`] = param.String(refund.InvoiceNumber)
		notify[`total_amount`] = param.String(refund.Amount.Total)
		if a.notifyCallback != nil {
			ctx.Set(`notify`, notify)
			if err := a.notifyCallback(ctx); err != nil {
				return err
			}
		}
	default:
		println(`event.EventType: `, event.EventType, echo.Dump(event, false))
	}
	return nil
}

func (a *Paypal) Refund(cfg *config.Refund) (param.StringMap, error) {
	result := param.StringMap{}
	refundConfig := &paypal.RefundSaleParam{
		Amount: &paypal.Amount{
			Total:    MoneyFeeToString(cfg.RefundAmount),
			Currency: cfg.Currency.String(),
		},
		Description:   ``,
		Reason:        cfg.RefundReason,
		InvoiceNumber: cfg.OutTradeNo,
	}
	refund, err := a.Client().RefundSale(cfg.TradeNo, refundConfig)
	if err != nil {
		return result, err
	}
	result[`success`] = ``
	if refund.State == paypal.K_REFUND_STATE_COMPLETED {
		result[`success`] = `1`
	} else if refund.State == paypal.K_REFUND_STATE_COMPLETED || refund.State == paypal.K_REFUND_STATE_FAILED {
		result[`success`] = `0`
	}
	result[`id`] = param.String(refund.Id)
	result[`trade_no`] = param.String(refund.SaleId)
	result[`out_trade_no`] = param.String(refund.InvoiceNumber)
	result[`refund_fee`] = param.String(refund.Amount.Total) // 退款总金额

	return result, err
}
