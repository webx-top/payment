package razorpay

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

const Name = `razorpay`

var supports = config.Supports{
	config.SupportPayNotify,
	config.SupportPayQuery,
	config.SupportRefund,
	config.SupportRefundQuery,
}

var SupportedCurrencies = []string{
	"INR", "USD", "EUR", "GBP", "AED", "AUD",
	"CAD", "CHF", "HKD", "MYR", "SGD",
}

func init() {
	payment.Register(Name, echo.T(`Razorpay`), New)
}

func New() payment.Driver {
	return &Razorpay{}
}

type Razorpay struct {
	account        *config.Account
	notifyCallback payment.NotifyCallback
	apiURL         string
	apiKey         string
	apiSecret      string
}

func (a *Razorpay) IsSupported(s config.Support) bool {
	return supports.IsSupported(s)
}

func (a *Razorpay) SetNotifyCallback(callback payment.NotifyCallback) payment.Driver {
	a.notifyCallback = callback
	return a
}

func (a *Razorpay) SetAccount(account *config.Account) payment.Driver {
	a.account = account
	a.apiURL = account.Options.Extra.String("apiURL")
	if len(a.apiURL) == 0 {
		if a.account.Debug {
			a.apiURL = "https://api.razorpay.test"
		} else {
			a.apiURL = "https://api.razorpay.com"
		}
	}
	a.apiKey = a.account.AppID       // key_id (rzp_live_xxx)
	a.apiSecret = a.account.AppSecret // key_secret
	return a
}

func (a *Razorpay) client() *resty.Request {
	return restyclient.Retryable().
		SetBasicAuth(a.apiKey, a.apiSecret).
		SetHeader("Content-Type", "application/json").
		SetHeader("User-Agent", "webx-top-payment-razorpay/1.0")
}

// Pay 创建 Razorpay Order
// 文档: https://razorpay.com/docs/api/orders/#create-an-order
// Razorpay 使用 Order + Payment 两步流程，前端拿到 order_id 后用 Checkout SDK 完成支付
func (a *Razorpay) Pay(ctx echo.Context, cfg *config.Pay) (*config.PayResponse, error) {
	body := map[string]interface{}{
		"amount":          AmountToPaise(cfg.Amount),
		"currency":        strings.ToLower(cfg.Currency.String()),
		"receipt":         cfg.OutTradeNo,
		"notes": map[string]string{
			"out_trade_no": cfg.OutTradeNo,
		},
	}

	var response struct {
		ID       string `json:"id"`
		Entity   string `json:"entity"`
		Amount   int64  `json:"amount"`
		Currency string `json:"currency"`
		Status   string `json:"status"`
	}

	resp, err := a.client().
		SetResult(&response).
		SetBody(body).
		Post(a.apiURL + "/v1/orders")
	if err != nil {
		return nil, err
	}
	if resp.StatusCode() != http.StatusOK {
		var rzErr struct {
			Error struct {
				Code    string `json:"code"`
				Description string `json:"description"`
			} `json:"error"`
		}
		json.Unmarshal(resp.Body(), &rzErr)
		return nil, fmt.Errorf("razorpay[%s]: %s", rzErr.Error.Code, rzErr.Error.Description)
	}

	result := &config.PayResponse{
		TradeNo: response.ID,
		Params: echo.H{
			"order_id": response.ID,
			"key_id":   a.apiKey,
			"currency": strings.ToLower(cfg.Currency.String()),
			"amount":   fmt.Sprintf("%.0f", AmountToPaise(cfg.Amount)),
		},
		Raw: &response,
	}
	return result, nil
}

// razorpayWebhookPayload Razorpay webhook 通用载荷结构
type razorpayWebhookPayload struct {
	PaymentID string `json:"payment_id"`
	Entity    string `json:"entity"`
	Event     string `json:"event"` // payment.captured / payment.failed / refund.processed / refund.created
	Payload   struct {
		Payment struct {
			ID        string            `json:"id"`
			Entity    string            `json:"entity"`
			Amount    int64             `json:"amount"`
			Currency  string            `json:"currency"`
			Status    string            `json:"status"`
			OrderID   string            `json:"order_id"`
			Method    string            `json:"method"`
			BankRefNo string            `json:"acquirer_data.bank_ref_no"`
			CreatedAt int64             `json:"created_at"`
			Notes     map[string]string `json:"notes"`
		} `json:"payment"`
		Refund struct {
			ID        string            `json:"id"`
			Entity    string            `json:"entity"`
			Amount    int64             `json:"amount"`
			Currency  string            `json:"currency"`
			Status    string            `json:"status"`
			CreatedAt int64             `json:"created_at"`
			Notes     map[string]string `json:"notes"`
		} `json:"refund"`
		Order struct {
			ID      string `json:"id"`
			Entity  string `json:"entity"`
			Amount  int64  `json:"amount"`
			Receipt string `json:"receipt"`
			Status  string `json:"status"`
		} `json:"order"`
	} `json:"payload"`
}

// PayNotify 处理 Webhook 回调(仅支付事件)
// 文档: https://razorpay.com/docs/webhooks/
func (a *Razorpay) PayNotify(ctx echo.Context) error {
	if err := a.VerifySign(ctx); err != nil {
		return ctx.JSON(echo.H{"error": err.Error()})
	}

	var payload razorpayWebhookPayload
	err := ctx.MustBind(&payload)
	if err != nil {
		return err
	}

	p := payload.Payload.Payment
	switch payload.Event {
	case "payment.captured":
		if a.notifyCallback != nil && p.Status == "captured" {
			result := &config.Result{
				Operation:   config.OperationPayment,
				Status:      config.TradeStatusSuccess,
				TradeNo:     p.ID,
				OutTradeNo:  defaultString(p.Notes["out_trade_no"], p.OrderID),
				Currency:    p.Currency,
				TotalAmount: PaiseToAmount(p.Amount),
				Raw:         &payload,
			}
			a.notifyCallback(ctx, result)
		}
	case "payment.failed":
		if a.notifyCallback != nil {
			result := &config.Result{
				Operation:   config.OperationPayment,
				Status:      config.TradeStatusClosed,
				TradeNo:     p.ID,
				OutTradeNo:  defaultString(p.Notes["out_trade_no"], p.OrderID),
				Currency:    p.Currency,
				TotalAmount: PaiseToAmount(p.Amount),
				Raw:         &payload,
			}
			a.notifyCallback(ctx, result)
		}
	}

	return ctx.JSON(echo.H{"status": "ok"})
}

// PayQuery 查询支付状态(通过 Payment API)
// 文档: https://razorpay.com/docs/api/payments/#fetch-a-payment
func (a *Razorpay) PayQuery(ctx echo.Context, cfg *config.Query) (*config.Result, error) {
	var response struct {
		ID       string `json:"id"`
		Amount   int64  `json:"amount"`
		Currency string `json:"currency"`
		Status   string `json:"status"`
		Method   string `json:"method"`
		Notes    map[string]string `json:"notes"`
	}

	resp, err := a.client().
		SetResult(&response).
		Get(a.apiURL + "/v1/payments/" + cfg.TradeNo)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode() != http.StatusOK {
		return nil, fmt.Errorf("razorpay: query failed %d", resp.StatusCode())
	}

	status := MappingStatus(response.Status)

	return &config.Result{
		Operation:   config.OperationPayment,
		Status:      status,
		TradeNo:     response.ID,
		OutTradeNo:  defaultString(response.Notes["out_trade_no"], ""),
		Currency:    response.Currency,
		TotalAmount: PaiseToAmount(response.Amount),
		Raw:         &response,
	}, nil
}

// Refund 发起退款
// 文档: https://razorpay.com/docs/api/refunds/#create-refund
func (a *Razorpay) Refund(ctx echo.Context, cfg *config.Refund) (*config.Result, error) {
	body := map[string]interface{}{
		"amount": AmountToPaise(cfg.RefundAmount),
		"speed":  "normal", // optimal / normal / worst
		"notes": map[string]string{
			"out_refund_no": cfg.OutRefundNo,
		},
	}

	var response struct {
		ID       string `json:"id"`
		Entity   string `json:"entity"`
		Amount   int64  `json:"amount"`
		Currency string `json:"currency"`
		Status   string `json:"status"`
	}

	resp, err := a.client().
		SetResult(&response).
		SetBody(body).
		Post(a.apiURL + "/v1/payments/" + cfg.TradeNo + "/refund")
	if err != nil {
		return nil, err
	}
	if resp.StatusCode() != http.StatusOK && resp.StatusCode() != http.StatusAccepted {
		return nil, fmt.Errorf("razorpay: refund failed %d", resp.StatusCode())
	}

	status := MapRefundStatus(response.Status)

	return &config.Result{
		Operation:   config.OperationRefund,
		Status:      status,
		TradeNo:     cfg.TradeNo,
		OutTradeNo:  cfg.OutTradeNo,
		Currency:    response.Currency,
		RefundFee:   PaiseToAmount(response.Amount),
		RefundNo:    response.ID,
		OutRefundNo: cfg.OutRefundNo,
		Raw:         &response,
	}, nil
}

// RefundNotify 处理退款通知(Razorpay退款webhook与支付共用同一endpoint)
// 此方法独立处理退款语义，仅响应 refund.* 事件
// 文档: https://razorpay.com/docs/webhooks/
func (a *Razorpay) RefundNotify(ctx echo.Context) error {
	if err := a.VerifySign(ctx); err != nil {
		return ctx.JSON(echo.H{"error": err.Error()})
	}

	var payload razorpayWebhookPayload
	err := ctx.MustBind(&payload)
	if err != nil {
		return err
	}

	// 仅处理退款事件
	switch payload.Event {
	case "refund.processed":
		refund := payload.Payload.Refund
		p := payload.Payload.Payment
		if a.notifyCallback != nil {
			result := &config.Result{
				Operation:   config.OperationRefund,
				Status:      config.TradeStatusSuccess,
				TradeNo:     p.ID,
				OutTradeNo:  defaultString(p.Notes["out_trade_no"], p.OrderID),
				Currency:    refund.Currency,
				TotalAmount: PaiseToAmount(p.Amount),
				RefundFee:   PaiseToAmount(refund.Amount),
				RefundNo:    refund.ID,
				OutRefundNo: defaultString(refund.Notes["out_refund_no"], ""),
				Raw:         &payload,
			}
			a.notifyCallback(ctx, result)
		}
	case "refund.created":
		refund := payload.Payload.Refund
		p := payload.Payload.Payment
		if a.notifyCallback != nil {
			result := &config.Result{
				Operation:   config.OperationRefund,
				Status:      config.TradeStatusProcessing,
				TradeNo:     p.ID,
				OutTradeNo:  defaultString(p.Notes["out_trade_no"], p.OrderID),
				Currency:    refund.Currency,
				TotalAmount: PaiseToAmount(p.Amount),
				RefundFee:   PaiseToAmount(refund.Amount),
				RefundNo:    refund.ID,
				OutRefundNo: defaultString(refund.Notes["out_refund_no"], ""),
				Raw:         &payload,
			}
			a.notifyCallback(ctx, result)
		}
	default:
		// 非退款事件不处理
	}

	return ctx.JSON(echo.H{"status": "ok"})
}

// RefundQuery 查询退款状态
// 文档: https://razorpay.com/docs/api/refunds/#fetch-a-refund
func (a *Razorpay) RefundQuery(ctx echo.Context, cfg *config.Query) (*config.Result, error) {
	var response struct {
		ID        string `json:"id"`
		Amount    int64  `json:"amount"`
		Currency  string `json:"currency"`
		Status    string `json:"status"`
		PaymentID string `json:"payment_id"`
		Notes     map[string]string `json:"notes"`
	}

	resp, err := a.client().
		SetResult(&response).
		Get(a.apiURL + "/v1/refunds/" + cfg.RefundNo)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode() != http.StatusOK {
		return nil, fmt.Errorf("razorpay: refund query failed %d", resp.StatusCode())
	}

	status := MapRefundStatus(response.Status)

	return &config.Result{
		Operation:   config.OperationRefund,
		Status:      status,
		TradeNo:     response.PaymentID,
		OutTradeNo:  defaultString(response.Notes["out_trade_no"], ""),
		Currency:    response.Currency,
		RefundFee:   PaiseToAmount(response.Amount),
		RefundNo:    response.ID,
		OutRefundNo: defaultString(response.Notes["out_refund_no"], ""),
		Raw:         &response,
	}, nil
}
