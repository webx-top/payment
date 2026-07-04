package mollie

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/admpub/resty/v2"
	"github.com/webx-top/echo"
	"github.com/webx-top/echo/param"
	"github.com/webx-top/payment"
	"github.com/webx-top/payment/config"
	"github.com/webx-top/restyclient"
)

const Name = `mollie`

var supports = config.Supports{
	config.SupportPayNotify,
	config.SupportPayQuery,
	config.SupportRefund,
	config.SupportRefundQuery,
}

var SupportedCurrencies = []string{
	"USD", "EUR", "GBP", "AUD", "CAD", "CHF",
	"DKK", "NOK", "SEK", "PLN", "CZK", "HUF",
	"RON", "BGN", "HRK", "ISK", "TRY", "JPY",
}

func init() {
	payment.Register(Name, echo.T(`Mollie`), New)
}

func New() payment.Driver {
	return &Mollie{}
}

type Mollie struct {
	account        *config.Account
	notifyCallback payment.NotifyCallback
	apiURL         string
}

func (a *Mollie) IsSupported(s config.Support) bool {
	return supports.IsSupported(s)
}

func (a *Mollie) SetNotifyCallback(callback payment.NotifyCallback) payment.Driver {
	a.notifyCallback = callback
	return a
}

func (a *Mollie) SetAccount(account *config.Account) payment.Driver {
	a.account = account
	a.apiURL = account.Options.Extra.String("apiURL")
	if len(a.apiURL) == 0 {
		if a.account.Debug {
			a.apiURL = "https://api.mollie.test"
		} else {
			a.apiURL = "https://api.mollie.nl"
		}
	}
	return a
}

func (a *Mollie) client() *resty.Request {
	return restyclient.Retryable().
		SetHeader("Authorization", "Bearer "+a.account.AppSecret).
		SetHeader("Content-Type", "application/json")
}

// Pay 创建 Mollie Payment
// 文档: https://docs.mollie.com/reference/v2/payments-api/create-payment
func (a *Mollie) Pay(ctx echo.Context, cfg *config.Pay) (*config.PayResponse, error) {
	body := map[string]interface{}{
		"amount": map[string]interface{}{
			"currency": cfg.Currency.String(),
			"value":    MoneyFeeToString(cfg.Amount),
		},
		"description": defaultString(cfg.Subject, "Order"),
		"redirectUrl": defaultString(cfg.ReturnURL, ""),
		"cancelUrl":   defaultString(cfg.CancelURL, ""),
		"metadata": map[string]string{
			"out_trade_no": cfg.OutTradeNo,
		},
		"webhookUrl": defaultString(cfg.NotifyURL, ""),
		"method":     defaultString(cfg.Subtype, ""), // ideal/bancontact/creditcard等
	}

	var response struct {
		ID           string `json:"id"`
		Status       string `json:"status"`
		IsCancelable bool   `json:"isCancelable"`
		CheckoutURL  struct {
			Href string `json:"href"`
		} `json:"checkout"`
		Links struct {
			PaymentURL struct {
				Href string `json:"href"`
			} `json:"paymentUrl"`
		} `json:"_links"`
	}

	resp, err := a.client().
		SetResult(&response).
		SetBody(body).
		Post(a.apiURL + "/v2/payments")
	if err != nil {
		return nil, err
	}
	if resp.StatusCode() != http.StatusCreated {
		var mollieErr struct {
			Status int    `json:"status"`
			Title  string `json:"title"`
			Detail string `json:"detail"`
		}
		json.Unmarshal(resp.Body(), &mollieErr)
		return nil, fmt.Errorf("mollie[%d]: %s - %s", mollieErr.Status, mollieErr.Title, mollieErr.Detail)
	}

	redirectURL := ""
	if len(response.Links.PaymentURL.Href) > 0 {
		redirectURL = response.Links.PaymentURL.Href
	} else if len(response.CheckoutURL.Href) > 0 {
		redirectURL = response.CheckoutURL.Href
	}

	result := &config.PayResponse{
		TradeNo:     response.ID,
		RedirectURL: redirectURL,
		Raw:         &response,
	}
	return result, nil
}

// PayNotify 处理 Mollie Webhook 回调
// 文档: https://docs.mollie.com/guides/credits/webhook#handling-the-webhook-call
func (a *Mollie) PayNotify(ctx echo.Context) error {
	paymentID := ctx.Form("id")
	if len(paymentID) == 0 {
		return ctx.JSON(echo.H{"error": "missing id"})
	}

	pi, err := a.getPayment(paymentID)
	if err != nil {
		return ctx.JSON(echo.H{"error": err.Error()})
	}

	if a.notifyCallback != nil && pi.PaidAt != "" {
		result := &config.Result{
			Operation:   config.OperationPayment,
			Status:      config.TradeStatusSuccess,
			TradeNo:     pi.ID,
			OutTradeNo:  pi.Metadata["out_trade_no"],
			Currency:    pi.Amount["currency"],
			TotalAmount: param.AsFloat64(pi.Amount["value"]),
			Raw:         pi,
		}
		if err := a.notifyCallback(ctx, result); err != nil {
			return err
		}
	}
	return ctx.String("ok")
}

// PayQuery 查询支付状态
// 文档: https://docs.mollie.com/reference/v2/payments-api/get-payment
func (a *Mollie) PayQuery(ctx echo.Context, cfg *config.Query) (*config.Result, error) {
	pi, err := a.getPayment(cfg.TradeNo)
	if err != nil {
		return nil, err
	}

	result := &config.Result{
		Operation:   config.OperationPayment,
		TradeNo:     pi.ID,
		OutTradeNo:  pi.Metadata["out_trade_no"],
		Currency:    pi.Amount["currency"],
		TotalAmount: param.AsFloat64(pi.Amount["value"]),
		Raw:         pi,
	}
	MappingStatus(pi.Status, result)
	return result, nil
}

// Refund 创建退款
// 文档: https://docs.mollie.com/reference/v2/refunds-api/create-refund
func (a *Mollie) Refund(ctx echo.Context, cfg *config.Refund) (*config.Result, error) {
	body := map[string]interface{}{
		"amount": map[string]interface{}{
			"currency": strings.ToLower(cfg.Currency.String()),
			"value":    MoneyFeeToString(cfg.RefundAmount),
		},
		"metadata": map[string]string{
			"out_refund_no": cfg.OutRefundNo,
		},
	}

	var response struct {
		Resource  string            `json:"resource"`
		ID        string            `json:"id"`
		Status    string            `json:"status"`
		Amount    map[string]string `json:"amount"`
		SettledAt string            `json:"settledAt"`
		CreatedAt string            `json:"createdAt"`
	}

	resp, err := a.client().
		SetResult(&response).
		SetBody(body).
		Post(a.apiURL + "/v2/payments/" + cfg.TradeNo + "/refunds")
	if err != nil {
		return nil, err
	}
	if resp.StatusCode() != http.StatusCreated {
		return nil, fmt.Errorf("mollie: refund create failed %d", resp.StatusCode())
	}

	status := MapRefundStatus(response.Status)

	return &config.Result{
		Operation:   config.OperationRefund,
		Status:      status,
		TradeNo:     cfg.TradeNo,
		OutTradeNo:  cfg.OutTradeNo,
		Currency:    response.Amount["currency"],
		RefundFee:   param.AsFloat64(response.Amount["value"]),
		RefundNo:    response.ID,
		OutRefundNo: cfg.OutRefundNo,
		Raw:         &response,
	}, nil
}

// RefundNotify 处理退款通知
// Mollie 不单独发退款 webhook，但此方法需独立返回退款语义的结果。
// 调用方应传入 refund_id，通过查询退款详情获取信息
// 文档: https://docs.mollie.com/reference/v2/refunds-api/get-refund
func (a *Mollie) RefundNotify(ctx echo.Context) error {
	// 优先从表单/参数获取 refund_id，否则回退到 payment id
	refundID := ctx.Form("refund_id")
	if len(refundID) == 0 {
		refundID = ctx.Param("id")
	}

	if len(refundID) == 0 {
		return ctx.JSON(echo.H{"error": "missing refund id"})
	}

	refund, err := a.getRefund(refundID)
	if err != nil {
		return ctx.JSON(echo.H{"error": err.Error()})
	}

	if a.notifyCallback != nil && refund.Status == "refunded" {
		result := &config.Result{
			Operation:   config.OperationRefund,
			Status:      config.TradeStatusSuccess,
			TradeNo:     refund.PaymentID,
			Currency:    refund.Amount["currency"],
			TotalAmount: param.AsFloat64(refund.Amount["value"]),
			RefundFee:   param.AsFloat64(refund.Amount["value"]),
			RefundNo:    refund.ID,
			OutRefundNo: defaultString(refund.Metadata["out_refund_no"], ""),
			Raw:         refund,
		}
		a.notifyCallback(ctx, result)
	}
	return ctx.String("ok")
}

// RefundQuery 查询退款状态
// 文档: https://docs.mollie.com/reference/v2/refunds-api/get-refund
func (a *Mollie) RefundQuery(ctx echo.Context, cfg *config.Query) (*config.Result, error) {
	var response struct {
		ID        string            `json:"id"`
		Status    string            `json:"status"`
		Amount    map[string]string `json:"amount"`
		PaymentID string            `json:"paymentId"`
		SettledAt string            `json:"settledAt"`
	}

	resp, err := a.client().
		SetResult(&response).
		Get(a.apiURL + "/v2/refunds/" + cfg.RefundNo)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode() != http.StatusOK {
		return nil, fmt.Errorf("mollie: refund query failed %d", resp.StatusCode())
	}

	status := MapRefundStatus(response.Status)

	return &config.Result{
		Operation:   config.OperationRefund,
		Status:      status,
		TradeNo:     response.PaymentID,
		OutTradeNo:  cfg.OutTradeNo,
		Currency:    response.Amount["currency"],
		RefundFee:   param.AsFloat64(response.Amount["value"]),
		RefundNo:    response.ID,
		OutRefundNo: cfg.OutRefundNo,
		Raw:         &response,
	}, nil
}

// getPayment 内部方法：获取支付信息
func (a *Mollie) getPayment(id string) (*molliePayment, error) {
	var response molliePayment
	resp, err := a.client().
		SetResult(&response).
		Get(a.apiURL + "/v2/payments/" + id)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode() != http.StatusOK {
		return nil, fmt.Errorf("mollie: get payment failed %d", resp.StatusCode())
	}
	return &response, nil
}

// getRefund 内部方法：获取退款信息
// 文档: https://docs.mollie.com/reference/v2/refunds-api/get-refund
func (a *Mollie) getRefund(id string) (*mollieRefund, error) {
	var refund mollieRefund
	resp, err := a.client().
		SetResult(&refund).
		Get(a.apiURL + "/v2/refunds/" + id)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode() != http.StatusOK {
		return nil, fmt.Errorf("mollie: get refund failed %d", resp.StatusCode())
	}
	return &refund, nil
}

type molliePayment struct {
	ID             string            `json:"id"`
	Status         string            `json:"status"`
	IsCancelable   bool              `json:"isCancelable"`
	Amount         map[string]string `json:"amount"`
	AmountCaptured map[string]string `json:"amountCaptured"`
	AmountRefunded map[string]string `json:"amountRefunded"`
	Description    string            `json:"description"`
	Method         string            `json:"method"`
	Metadata       map[string]string `json:"metadata"`
	CreatedAt      string            `json:"createdAt"`
	PaidAt         string            `json:"paidAt"`
	ExpiresAt      string            `json:"expiresAt"`
	ExpiredAt      string            `json:"expiredAt"`
	CancelledAt    string            `json:"cancelledAt"`
	Links          struct {
		PaymentURL struct{ Href string } `json:"paymentUrl"`
	} `json:"_links"`
}

type mollieRefund struct {
	ID        string            `json:"id"`
	Status    string            `json:"status"`
	Amount    map[string]string `json:"amount"`
	PaymentID string            `json:"paymentId"`
	Metadata  map[string]string `json:"metadata"`
	SettledAt string            `json:"settledAt"`
	CreatedAt string            `json:"createdAt"`
}
