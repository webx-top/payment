package klarna

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/admpub/resty/v2"
	"github.com/webx-top/echo"
	"github.com/webx-top/payment"
	"github.com/webx-top/payment/config"
	"github.com/webx-top/restyclient"
)

const Name = `klarna`

var supports = config.Supports{
	config.SupportPayNotify,
	config.SupportPayQuery,
	config.SupportRefund,
	config.SupportRefundQuery,
}

var SupportedCurrencies = []string{
	"AUD", "CAD", "DKK", "EUR", "GBP", "NOK",
	"NZD", "PLN", "SEK", "USD", "CHF",
}

func init() {
	payment.Register(Name, echo.T(`Klarna`), New)
}

func New() payment.Driver {
	return &Klarna{}
}

type Klarna struct {
	account        *config.Account
	notifyCallback payment.NotifyCallback
	apiURL         string
	username       string
	password       string
}

func (a *Klarna) IsSupported(s config.Support) bool {
	return supports.IsSupported(s)
}

func (a *Klarna) SetNotifyCallback(callback payment.NotifyCallback) payment.Driver {
	a.notifyCallback = callback
	return a
}

func (a *Klarna) SetAccount(account *config.Account) payment.Driver {
	a.account = account
	if a.account.Debug {
		a.apiURL = "https://api.playground.klarna.com"
	} else {
		a.apiURL = "https://api.klarna.com"
	}
	a.username = a.account.AppID       // UID
	a.password = a.account.AppSecret   // Password
	return a
}

func (a *Klarna) client() *resty.Request {
	return restyclient.Retryable().
		SetBasicAuth(a.username, a.password).
		SetHeader("Content-Type", "application/json").
		SetHeader("Accept-Version", "klarna.checkout.aggregated.min.version-v3+json").
		SetQueryParam("locale", "en-US")
}

// Pay 创建 Klarna Checkout Session (Hosted Payment Page)
// 文档: https://developers.klarna.com/documentation/klarna-checkout/integration-guide-html/
func (a *Klarna) Pay(ctx echo.Context, cfg *config.Pay) (*config.PayResponse, error) {
	orderLines := []map[string]interface{}{{
		"name":            defaultString(cfg.Subject, "Order"),
		"quantity":        1,
		"unit_price":      AmountToMinorInt(cfg.Amount),
		"tax_rate":        0,
		"total_amount":    AmountToMinorInt(cfg.Amount),
		"total_discount_amount": 0,
		"total_tax_amount": 0,
	}}

	body := map[string]interface{}{
		"purchase_country":  countryFromCurrency(cfg.Currency.String()),
		"purchase_currency": strings.ToUpper(cfg.Currency.String()),
		"locale":            "en-US",
		"order_amount":      AmountToMinorInt(cfg.Amount),
		"order_lines":       orderLines,
		"merchant_urls": map[string]string{
			"terms":      defaultString(cfg.Options.String("terms_url"), ""),
			"checkout":    defaultString(cfg.ReturnURL, ""),
			"confirmation": defaultString(cfg.NotifyURL, ""), // Klarna 用 confirmation URL 做回调
			"push":        defaultString(cfg.NotifyURL, ""),
		},
		"options": map[string]interface{}{
			"allow_separate_shipping_address": false,
		},
	}

	var response struct {
		OrderID     string `json:"order_id"`
		HTMLSnippet string `json:"html_snippet"`
		CheckoutURL string `json:"checkout_url,omitempty"`
	}

	resp, err := a.client().
		SetResult(&response).
		SetBody(body).
		Post(a.apiURL + "/checkout/v3/orders")
	if err != nil {
		return nil, err
	}
	if resp.StatusCode() != http.StatusCreated {
		var klErr struct {
			HttpStatusCode int    `json:"http_status_code"`
			ErrorCode      string `json:"error_code"`
			ErrorMessage   string `json:"error_messages"`
		}
		json.Unmarshal(resp.Body(), &klErr)
		return nil, fmt.Errorf("klarna[%d]: %s - %s", klErr.HttpStatusCode, klErr.ErrorCode, klErr.ErrorMessage)
	}

	result := &config.PayResponse{
		TradeNo:     response.OrderID,
		RedirectURL: response.CheckoutURL,
		QRCodeContent: response.HTMLSnippet,
		Raw:         &response,
	}
	return result, nil
}

// PayNotify 处理 Klarna Push Notification / Confirmation Callback
// 文档: https://developers.klarna.com/documentation/klarna-payments/api/callback-notifications/
func (a *Klarna) PayNotify(ctx echo.Context) error {
	orderID := ctx.Form("klarna_order_id")
	if len(orderID) == 0 {
		orderID = ctx.Param("id") // 从 URL 参数获取
	}

	order, err := a.getOrder(orderID)
	if err != nil {
		return err
	}

	switch order.Status {
	case "checkout_complete":
		if a.notifyCallback != nil {
			total := totalAmount(order.OrderAmount)
			result := &config.Result{
				Operation:   config.OperationPayment,
				Status:      config.TradeStatusSuccess,
				TradeNo:     order.OrderID,
				OutTradeNo:  "",
				Currency:    order.PurchaseCurrency,
				TotalAmount: total,
				Raw:         order,
			}
			if err := a.notifyCallback(ctx, result); err != nil {
				return err
			}
		}
		// 返回 HTML snippet 让前端完成确认流程
		return ctx.String(order.HTMLSnippet)
	case "created":
		return ctx.JSON(echo.H{"html_snippet": order.HTMLSnippet})
	default:
		return ctx.JSON(echo.H{"status": order.Status})
	}
}

// PayQuery 查询订单状态
// 文档: https://developers.klarna.com/documentation/klarna-checkout/in-store-api/
func (a *Klarna) PayQuery(ctx echo.Context, cfg *config.Query) (*config.Result, error) {
	order, err := a.getOrder(cfg.TradeNo)
	if err != nil {
		return nil, err
	}

	result := &config.Result{
		Operation:   config.OperationPayment,
		TradeNo:     order.OrderID,
		Currency:    order.PurchaseCurrency,
		TotalAmount: totalAmount(order.OrderAmount),
		Raw:         order,
	}
	MappingStatus(order.Status, result)
	return result, nil
}

// Refund 发起退款
// 文档: https://developers.klarna.com/documentation/klarna-payments/api/refunds/
func (a *Klarna) Refund(ctx echo.Context, cfg *config.Refund) (*config.Result, error) {
	body := map[string]interface{}{
		"refunded_amount": AmountToMinorInt(cfg.RefundAmount),
		"description":      defaultString(cfg.RefundReason, "Customer requested refund"),
	}

	var response struct {
		RefundID      string `json:"refund_id"`
		RefundedAmount int64 `json:"refunded_amount"`
	}

	resp, err := a.client().
		SetResult(&response).
		SetBody(body).
		Post(a.apiURL + "/order-management/v1/orders/" + cfg.TradeNo + "/refunds")
	if err != nil {
		return nil, err
	}
	if resp.StatusCode() != http.StatusOK {
		return nil, fmt.Errorf("klarna: refund failed %d", resp.StatusCode())
	}

	return &config.Result{
		Operation:   config.OperationRefund,
		Status:      config.TradeStatusSuccess,
		TradeNo:     cfg.TradeNo,
		OutTradeNo:  cfg.OutTradeNo,
		Currency:    cfg.Currency.String(),
		RefundFee:   MinorToAmount(response.RefundedAmount),
		RefundNo:    response.RefundID,
		OutRefundNo: cfg.OutRefundNo,
		Raw:         &response,
	}, nil
}

// RefundNotify Klarna 退款通过统一通知处理
func (a *Klarna) RefundNotify(ctx echo.Context) error {
	return a.PayNotify(ctx)
}

// RefundQuery 查询退款状态
func (a *Klarna) RefundQuery(ctx echo.Context, cfg *config.Query) (*config.Result, error) {
	var response struct {
		RefundID       string `json:"refund_id"`
		RefundedAmount int64 `json:"refunded_amount"`
		Status         string `json:"status"`
	}

	resp, err := a.client().
		SetResult(&response).
		Get(a.apiURL + "/order-management/v1/orders/" + cfg.TradeNo + "/refunds/" + cfg.RefundNo)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode() != http.StatusOK {
		return nil, fmt.Errorf("klarna: refund query failed %d", resp.StatusCode())
	}

	status := MapRefundStatus(response.Status)

	return &config.Result{
		Operation:   config.OperationRefund,
		Status:      status,
		TradeNo:     cfg.TradeNo,
		OutTradeNo:  cfg.OutTradeNo,
		RefundNo:    response.RefundID,
		RefundFee:   MinorToAmount(response.RefundedAmount),
		OutRefundNo: cfg.OutRefundNo,
		Raw:         &response,
	}, nil
}

// getOrder 内部方法获取订单信息
func (a *Klarna) getOrder(orderID string) (*klarnaOrder, error) {
	var order klarnaOrder
	resp, err := a.client().
		SetResult(&order).
		Get(a.apiURL + "/checkout/v3/orders/" + orderID)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode() != http.StatusOK {
		return nil, fmt.Errorf("klarna: get order failed %d", resp.StatusCode())
	}
	return &order, nil
}

type klarnaOrder struct {
	OrderID          string                   `json:"order_id"`
	Status           string                   `json:"status"`
	PurchaseCountry  string                   `json:"purchase_country"`
	PurchaseCurrency string                   `json:"purchase_currency"`
	Locale           string                   `json:"locale"`
	OrderAmount      int64                    `json:"order_amount"`
	HTMLSnippet      string                   `json:"html_snippet"`
	MerchantURLs     map[string]string        `json:"merchant_urls"`
	OrderLines       []map[string]interface{}  `json:"order_lines"`
}
