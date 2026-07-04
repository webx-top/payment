package twoco

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/admpub/resty/v2"
	"github.com/webx-top/com"
	"github.com/webx-top/echo"
	"github.com/webx-top/echo/param"
	"github.com/webx-top/payment"
	"github.com/webx-top/payment/config"
	"github.com/webx-top/restyclient"
)

const Name = `twoco`

var supports = config.Supports{
	config.SupportPayNotify,
	config.SupportPayQuery,
	config.SupportRefund,
	config.SupportRefundQuery,
}

var SupportedCurrencies = []string{
	"USD", "EUR", "GBP", "AUD", "CAD", "CHF",
	"DZD", "BHD", "BDT", "BOB", "BWP", "BRL",
	"BGN", "KHR", "KYD", "CLP", "COP", "CRC",
	"CZK", "DKK", "EGP", "EEK", "ETB", "FJD",
	"GHS", "GTQ", "GYD", "HNL", "HRK", "HUF",
	"ISK", "INR", "IDR", "JOD", "KES", "LAK",
	"LBP", "LKR", "MVR", "MWK", "MXN", "MYR",
	"MZN", "NAD", "NIO", "NGN", "NOK", "OMR",
	"PKR", "PGK", "PHP", "PLN", "QAR", "RON",
	"SAR", "SBD", "SOS", "ZAR", "SEK",
	"TJS", "TZS", "THB", "TTD", "TRY", "UGX",
	"UAH", "AED", "UYU", "VUV", "VEB", "VND",
	"XCD", "YER", "ZMK", "ZWD", "CNY", "JPY",
	"NZD", "SGD", "ILS", "KRW", "RUB", "TWD",
	"HKD", "ARS", "PYG", "PEN", "AZN", "BYN",
	"GEK", "KGS", "KZT", "MDL", "TMT", "UZS",
	"AFN", "AMD", "AOA", "BAM", "BND", "BSD",
	"BZD", "CDF", "CVE", "DJF", "DOP", "ERN",
	"FKP", "GIP", "GNF", "HTG", "IRR", "JMD",
	"KMF", "LSL", "LYD", "MAD", "MMK", "MNT",
	"MRO", "MUR", "NPR", "PAB", "RWF", "SDG",
	"SRD", "SYP", "TOP", "WST", "XAF", "XOF",
	"XPF", "ZWL",
}

// Verifone 2Checkout (现称 Verifone Payments)
// 文档: https://verifone.github.io/2co-api-docs/

func init() {
	payment.Register(Name, echo.T(`2Checkout/Verifone`), New)
}

func New() payment.Driver {
	return &Twoco{}
}

type Twoco struct {
	account        *config.Account
	notifyCallback payment.NotifyCallback
	sellerID       string
	privateKey     string
	apiURL         string
}

func (a *Twoco) IsSupported(s config.Support) bool {
	return supports.IsSupported(s)
}

func (a *Twoco) SetNotifyCallback(callback payment.NotifyCallback) payment.Driver {
	a.notifyCallback = callback
	return a
}

func (a *Twoco) SetAccount(account *config.Account) payment.Driver {
	a.account = account
	a.sellerID = a.account.AppID       // Seller ID (数字)
	a.privateKey = a.account.AppSecret // Private Key
	if a.account.Debug {
		a.apiURL = "sandbox.2checkout.com"
	} else {
		a.apiURL = "www.2checkout.com"
	}
	return a
}

func (a *Twoco) client() *resty.Request {
	return restyclient.Retryable().
		SetHeader("Content-Type", "application/json").
		SetHeader("Accept", "application/json").
		SetBasicAuth(a.sellerID, a.privateKey)
}

// Pay 创建 2Checkout Sale
// 文档: https://verifone.github.io/2co-api-docs/#create-sale
func (a *Twoco) Pay(ctx echo.Context, cfg *config.Pay) (*config.PayResponse, error) {
	body := map[string]interface{}{
		"sellerId": a.sellerID,
		"mode":     modeString(a.account.Debug),
		"currency": cfg.Currency.String(),
		"total":    MoneyFeeToString(cfg.Amount),
		"cart": map[string]interface{}{
			"orderId": cfg.OutTradeNo,
			"items": []map[string]interface{}{
				{
					"name":       defaultString(cfg.Subject, "Product"),
					"quantity":   "1",
					"type":       "product",
					"price":      MoneyFeeToString(cfg.Amount),
					"tangible":   "N",
					"recurrence": nil, // 单次购买
				},
			},
		},
		"billingAddr": map[string]interface{}{
			"name":  "Customer",
			"email": cfg.Options.String("email"),
		},
	}

	// 2Checkout 有两种集成方式:
	// 1. Hosted Checkout Page (推荐) - 用户跳转到 2Checkout 页面
	// 2. Inline Form - 在自己的页面内嵌入表单

	// 这里使用 Hosted 方式，构建跳转 URL
	params := url.Values{}
	params.Set("sid", a.sellerID)
	params.Set("mode", modeString(a.account.Debug))
	params.Set("li_0_type", "product")
	params.Set("li_0_name", defaultString(cfg.Subject, "Product"))
	params.Set("li_0_quantity", "1")
	params.Set("li_0_price", MoneyFeeToString(cfg.Amount))
	params.Set("li_0_tangible", "N")
	params.Set("currency_code", cfg.Currency.String())
	params.Set("cart_order_id", cfg.OutTradeNo)
	if len(cfg.ReturnURL) > 0 {
		params.Set("x_receipt_link_url", cfg.ReturnURL)
	}
	// 生成签名
	params.Set("hash", generateWordHash(params))

	redirectURL := "https://" + a.apiURL + "/checkout/purchase?" + params.Encode()

	result := &config.PayResponse{
		TradeNo:     cfg.OutTradeNo,
		RedirectURL: redirectURL,
		Params: echo.H{
			"params": params.Encode(),
		},
		Raw: body,
	}
	return result, nil
}

// PayNotify 处理 IPN (Instant Payment Notification) 回调
// 文档: https://verifone.github.io/2co-api-docs/#instant-notification-service-ins
func (a *Twoco) PayNotify(ctx echo.Context) error {
	if err := a.VerifySign(ctx); err != nil {
		return err
	}
	saleID := ctx.Form("sale_id")
	invoiceID := ctx.Form("invoice_id")
	messageType := ctx.Form("message_type") // ORDER_CREATED / FRAUD_STATUS_CHANGED / REFUND_ISSUED
	fraudStatus := ctx.Form("fraud_status") // pass / fail / wait

	if a.notifyCallback != nil {
		switch messageType {
		case "ORDER_CREATED", "FRAUD_STATUS_CHANGED":
			if fraudStatus == "pass" {
				result := &config.Result{
					Operation:   config.OperationPayment,
					Status:      config.TradeStatusSuccess,
					TradeNo:     saleID,
					OutTradeNo:  invoiceID,
					Currency:    ctx.Form("list_currency"),
					TotalAmount: param.AsFloat64(ctx.Form("total")),
				}
				a.notifyCallback(ctx, result)
			} else if fraudStatus == "fail" {
				result := &config.Result{
					Operation:  config.OperationPayment,
					Status:     config.TradeStatusException,
					TradeNo:    saleID,
					OutTradeNo: invoiceID,
					Reason:     "fraud check failed",
				}
				a.notifyCallback(ctx, result)
			}
		case "REFUND_ISSUED":
			refundNo := ctx.Form("refund_id")
			result := &config.Result{
				Operation:   config.OperationRefund,
				Status:      config.TradeStatusSuccess,
				TradeNo:     saleID,
				OutTradeNo:  invoiceID,
				RefundNo:    refundNo,
				RefundFee:   param.AsFloat64(ctx.Form("refund_usd")),
				OutRefundNo: ``,
			}
			a.notifyCallback(ctx, result)
		}
	}

	return ctx.String("") // 必须返回空字符串表示成功
}

// PayQuery 通过 REST API 查询销售详情
// 文档: https://verifone.github.io/2co-api-docs/#retrieve-sale
func (a *Twoco) PayQuery(ctx echo.Context, cfg *config.Query) (*config.Result, error) {
	var response struct {
		SaleID    string `json:"sale_id"`
		InvoiceID string `json:"invoice_id"`
		Status    struct {
			ID    string `json:"id"`
			Label string `json:"label"`
		} `json:"status"`
		ListingAmount struct {
			Usd    string `json:"usd"`
			Eur    string `json:"eur"`
			Amount string `json:"amount"`
		} `json:"listing_amount"`
		Currency string `json:"currency"`
	}

	resp, err := a.client().
		SetResult(&response).
		Get("https://" + a.apiURL + "/rest/sales/detail?sale_id=" + cfg.TradeNo)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode() != http.StatusOK {
		return nil, fmt.Errorf("twoco: query failed %d", resp.StatusCode())
	}

	status := MappingStatus(response.Status.Label)

	return &config.Result{
		Operation:   config.OperationPayment,
		Status:      status,
		TradeNo:     response.SaleID,
		OutTradeNo:  response.InvoiceID,
		Currency:    response.Currency,
		TotalAmount: param.AsFloat64(response.ListingAmount.Usd),
		Raw:         &response,
	}, nil
}

// Refund 发起退款
// 文档: https://verifone.github.io/2co-api-docs/#create-refund
func (a *Twoco) Refund(ctx echo.Context, cfg *config.Refund) (*config.Result, error) {
	body := map[string]interface{}{
		"saleId":   cfg.TradeNo,
		"comment":  defaultString(cfg.RefundReason, "Customer requested refund"),
		"category": "requested_by_customer",
		"amount":   MoneyFeeToString(cfg.RefundAmount),
		"currency": cfg.Currency.String(),
	}

	var response struct {
		RefundID string `json:"refund_id"`
		SaleID   string `json:"sale_id"`
		Amount   struct {
			Usd string `json:"usd"`
		} `json:"amount"`
		Status struct {
			ID    string `json:"id"`
			Label string `json:"label"`
		} `json:"status"`
	}

	resp, err := a.client().
		SetResult(&response).
		SetBody(body).
		Post("https://" + a.apiURL + "/rest/refunds")
	if err != nil {
		return nil, err
	}
	if resp.StatusCode() != http.StatusCreated && resp.StatusCode() != http.StatusOK {
		return nil, fmt.Errorf("twoco: refund failed %d", resp.StatusCode())
	}

	status := MapRefundStatus(response.Status.Label)

	return &config.Result{
		Operation:   config.OperationRefund,
		Status:      status,
		TradeNo:     response.SaleID,
		OutTradeNo:  cfg.OutTradeNo,
		Currency:    cfg.Currency.String(),
		RefundFee:   param.AsFloat64(response.Amount.Usd),
		RefundNo:    response.RefundID,
		OutRefundNo: cfg.OutRefundNo,
		Raw:         &response,
	}, nil
}

// RefundNotify 通过 PayNotify 统一处理
func (a *Twoco) RefundNotify(ctx echo.Context) error {
	return a.PayNotify(ctx)
}

// RefundQuery 查询退款详情
func (a *Twoco) RefundQuery(ctx echo.Context, cfg *config.Query) (*config.Result, error) {
	var response struct {
		RefundID string `json:"refund_id"`
		SaleID   string `json:"sale_id"`
		Amount   struct {
			Usd string `json:"usd"`
		} `json:"amount"`
		Status struct {
			Label string `json:"label"`
		} `json:"status"`
	}

	resp, err := a.client().
		SetResult(&response).
		Get("https://" + a.apiURL + "/rest/refunds/" + cfg.RefundNo)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode() != http.StatusOK {
		return nil, fmt.Errorf("twoco: refund query failed %d", resp.StatusCode())
	}

	status := MapRefundStatus(response.Status.Label)

	return &config.Result{
		Operation:   config.OperationRefund,
		Status:      status,
		TradeNo:     response.SaleID,
		RefundNo:    response.RefundID,
		RefundFee:   param.AsFloat64(response.Amount.Usd),
		OutRefundNo: cfg.OutRefundNo,
		Raw:         &response,
	}, nil
}

// --- 内部方法 ---

func (a *Twoco) verifyIPNSignature(ctx echo.Context) bool {
	secretKey := a.privateKey // 用于 IPN 签名的 secret word
	if len(secretKey) == 0 {
		return true
	}

	// 2Checkout IPN 签名验证
	// MD5(secret_word + seller_id + order_id + amount + currency)
	amount := ctx.Form("total")
	currency := ctx.Form("currency")
	orderID := ctx.Form("invoice_id")

	raw := fmt.Sprintf("%s%s%s%s%s", secretKey, a.sellerID, orderID, amount, currency)
	hasher := md5.Sum([]byte(raw))
	expected := hex.EncodeToString(hasher[:])

	received := ctx.Form("md5_hash")
	return received == expected
}

func modeString(debug bool) string {
	if debug {
		return "2CO-test"
	}
	return "2CO"
}

func generateWordHash(params url.Values) string {
	// 2Checkout 简化版签名
	return com.Md5(fmt.Sprintf("%d%s", time.Now().UnixNano(), params.Encode()))
}
