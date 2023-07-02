package paypal

import (
	"github.com/smartwalle/paypal"
	"github.com/webx-top/echo"
	"github.com/webx-top/echo/param"
	"github.com/webx-top/payment"
	"github.com/webx-top/payment/config"
)

const Name = `paypal`

var supports = config.Supports{
	config.SupportPayNotify,
	config.SupportPayQuery,
	config.SupportRefund,
	config.SupportRefundNotify,
	config.SupportRefundQuery,
}

func init() {
	payment.Register(Name, `贝宝`, New)
}

func New() payment.Driver {
	return &Paypal{}
}

type Paypal struct {
	account        *config.Account
	client         *paypal.Client
	notifyCallback func(echo.Context) error
}

func (a *Paypal) IsSupported(s config.Support) bool {
	return supports.IsSupported(s)
}

func (a *Paypal) SetNotifyCallback(callback func(echo.Context) error) payment.Driver {
	a.notifyCallback = callback
	return a
}

func (a *Paypal) SetAccount(account *config.Account) payment.Driver {
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

func (a *Paypal) Pay(ctx echo.Context, cfg *config.Pay) (*config.PayResponse, error) {
	var p = &paypal.Payment{}
	p.Intent = paypal.PaymentIntentSale
	p.Payer = &paypal.Payer{}
	p.Payer.PaymentMethod = paypal.PaymentMethodPayPal
	p.RedirectURLs = &paypal.RedirectURLs{}
	p.RedirectURLs.CancelURL = cfg.CancelURL
	p.RedirectURLs.ReturnURL = cfg.ReturnURL

	var transaction = &paypal.Transaction{}
	transaction.InvoiceNumber = cfg.OutTradeNo // 保存我方订单号
	p.Transactions = []*paypal.Transaction{transaction}

	transaction.Amount = &paypal.Amount{}
	transaction.Amount.Total = MoneyFeeToString(cfg.Amount)
	transaction.Amount.Currency = cfg.Currency.String()

	payment, err := a.Client().CreatePayment(p)
	if err != nil {
		return nil, err
	}
	//com.Dump(payment)
	result := &config.PayResponse{
		RedirectURL: ``,
		Raw:         payment,
	}
	for _, link := range payment.Links {
		if link.Rel == `approval_url` {
			result.RedirectURL = link.Href
			break
		}
	}
	return result, err
}

func (a *Paypal) PayQuery(ctx echo.Context, cfg *config.Query) (*config.Result, error) {
	paymentID := ctx.Form(`paymentId`)
	playerID := ctx.Form(`PayerID`)
	payment, err := a.Client().ExecuteApprovedPayment(paymentID, playerID)
	if err != nil {
		return nil, err
	}
	var (
		paid                   bool
		custom                 string
		tradeNo                string
		outOrderNo             string
		totalAmount            string
		currency               string
		transactionFeeValue    string // 交易手续费金额
		transactionFeeCurrency string // 交易手续费币种
	)
	if payment.State == paypal.PaymentStateApproved {
		paid = true
	}
	for _, transaction := range payment.Transactions {
		outOrderNo = transaction.InvoiceNumber
		custom = transaction.Custom
		for _, resource := range transaction.RelatedResources {
			tradeNo = resource.Sale.Id
			totalAmount = resource.Sale.Amount.Total
			currency = resource.Sale.Amount.Currency
			transactionFeeValue = resource.Sale.TransactionFee.Value
			transactionFeeCurrency = resource.Sale.TransactionFee.Currency
			if resource.Sale.State != paypal.SaleStateCompleted {
				paid = false
				break
			}
		}
	}
	var status string
	if paid {
		status = config.TradeStatusSuccess
	} else {
		status = config.TradeStatusWaitBuyerPay
	}
	return &config.Result{
		Operation:              config.OperationPayment,
		Status:                 status,
		TradeNo:                tradeNo,
		OutTradeNo:             outOrderNo,
		Currency:               currency,
		PassbackParams:         custom,
		TotalAmount:            param.AsFloat64(totalAmount),
		TransactionFeeValue:    param.AsFloat64(transactionFeeValue),
		TransactionFeeCurrency: transactionFeeCurrency,
		Reason:                 payment.FailureReason,
		Raw:                    payment,
	}, err
}

func (a *Paypal) PayNotify(ctx echo.Context) error {
	event, err := a.Client().GetWebhookEvent(a.account.WebhookID, ctx.Request().StdRequest())
	if err != nil {
		return err
	}
	if event == nil {
		return nil
	}
	switch event.EventType {
	case paypal.EventTypePaymentSaleCompleted:
		sale := event.Sale()
		if a.notifyCallback != nil {
			var reason, sep string
			for _, v := range sale.PaymentHoldReasons {
				if len(v.PaymentHoldReason) > 0 {
					reason += sep + v.PaymentHoldReason
					sep = `, `
				}
			}
			result := &config.Result{
				Operation:              config.OperationPayment,
				Status:                 config.TradeStatusSuccess,
				TradeNo:                sale.Id,
				OutTradeNo:             sale.InvoiceNumber,
				Currency:               sale.Amount.Currency,
				PassbackParams:         sale.Custom,
				TotalAmount:            param.AsFloat64(sale.Amount.Total),
				TransactionFeeValue:    param.AsFloat64(sale.TransactionFee.Value),
				TransactionFeeCurrency: sale.TransactionFee.Currency,
				Reason:                 reason,
				Raw:                    sale,
			}
			ctx.Set(`notify`, result)
			if err := a.notifyCallback(ctx); err != nil {
				return err
			}
		}
	case paypal.EventTypePaymentSaleRefunded:
		refund := event.Refund()
		if len(refund.Description) > 0 {
			refund.Reason += "; " + refund.Description
		}
		if a.notifyCallback != nil {
			result := &config.Result{
				Operation:      config.OperationRefund,
				Status:         config.TradeStatusSuccess,
				TradeNo:        refund.SaleId,
				OutTradeNo:     refund.InvoiceNumber,
				Currency:       refund.Amount.Currency,
				PassbackParams: refund.Custom,
				TotalAmount:    0,
				Reason:         refund.Reason,
				RefundFee:      param.AsFloat64(refund.Amount.Total),
				RefundNo:       refund.Id,
				OutRefundNo:    ``,
				Raw:            refund,
			}
			ctx.Set(`notify`, result)
			if err := a.notifyCallback(ctx); err != nil {
				return err
			}
		}
	default:
		if a.account.Debug {
			println(`event.EventType: `, event.EventType, echo.Dump(event, false))
		}
	}
	return nil
}

func (a *Paypal) Refund(ctx echo.Context, cfg *config.Refund) (*config.Result, error) {
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
		return nil, err
	}
	var status string
	switch refund.State {
	case paypal.RefundStateCompleted:
		status = config.TradeStatusSuccess
	case paypal.RefundStateCancelled:
		status = config.TradeStatusClosed
	case paypal.RefundStateFailed:
		status = config.TradeStatusException
	case paypal.RefundStatePending:
		status = config.TradeStatusProcessing
	}
	return &config.Result{
		Operation:      config.OperationRefund,
		Status:         status,
		TradeNo:        refund.SaleId,
		OutTradeNo:     refund.InvoiceNumber,
		Currency:       refund.Amount.Currency,
		TotalAmount:    0,
		PassbackParams: refund.Custom,
		Reason:         refund.Reason,
		RefundFee:      param.AsFloat64(refund.Amount.Total),
		RefundNo:       refund.Id,
		OutRefundNo:    ``,
		Raw:            refund,
	}, err
}

func (a *Paypal) RefundQuery(ctx echo.Context, cfg *config.Query) (*config.Result, error) {
	refund, err := a.Client().GetRefundDetails(cfg.RefundNo)
	if err != nil {
		return nil, err
	}
	var status string
	switch refund.State {
	case paypal.RefundStateCompleted:
		status = config.TradeStatusSuccess
	case paypal.RefundStateCancelled:
		status = config.TradeStatusClosed
	case paypal.RefundStateFailed:
		status = config.TradeStatusException
	case paypal.RefundStatePending:
		status = config.TradeStatusProcessing
	}
	return &config.Result{
		Operation:      config.OperationRefund,
		Status:         status,
		TradeNo:        refund.SaleId,
		OutTradeNo:     refund.InvoiceNumber,
		Currency:       refund.Amount.Currency,
		PassbackParams: refund.Custom,
		TotalAmount:    0,
		Reason:         refund.Reason,
		RefundFee:      param.AsFloat64(refund.Amount.Total),
		RefundNo:       refund.Id,
		OutRefundNo:    ``,
		Raw:            refund,
	}, err
}

func (a *Paypal) RefundNotify(ctx echo.Context) error {
	return a.PayNotify(ctx)
}
