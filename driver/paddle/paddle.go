package paddle

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

const Name = `paddle`

var supports = config.Supports{
	config.SupportPayNotify,
	config.SupportPayQuery,
	config.SupportRefund,
	config.SupportRefundQuery,
}

var SupportedCurrencies = []string{
	"USD", "EUR", "GBP", "AUD", "CAD", "CHF",
	"NOK", "SEK", "DKK", "JPY",
}

// Paddle Billing API v2 (2024+)
// 文档: https://developer.paddle.com/api-reference/

func init() {
	payment.Register(Name, echo.T(`Paddle`), New)
}

func New() payment.Driver {
	return &Paddle{}
}

type Paddle struct {
	account        *config.Account
	notifyCallback payment.NotifyCallback
	apiURL         string
	apiKey         string
}

func (a *Paddle) IsSupported(s config.Support) bool {
	return supports.IsSupported(s)
}

func (a *Paddle) SetNotifyCallback(callback payment.NotifyCallback) payment.Driver {
	a.notifyCallback = callback
	return a
}

func (a *Paddle) SetAccount(account *config.Account) payment.Driver {
	a.account = account
	a.apiURL = account.Options.Extra.String("apiURL")
	if len(a.apiURL) == 0 {
		a.apiURL = "https://sandbox-api.paddle.com" // 默认沙箱环境
	}
	if !a.account.Debug {
		a.apiURL = "https://api.paddle.com" // 生产环境
	}
	a.apiKey = a.account.AppSecret // API Key
	return a
}

func (a *Paddle) client() *resty.Request {
	return restyclient.Retryable().
		SetHeader("Authorization", "Bearer "+a.apiKey).
		SetHeader("Content-Type", "application/json")
}

// Pay 创建 Paddle Checkout (Price/Session)
// Paddle v2 使用 Price → Payment Link 流程
// 文档: https://developer.paddle.com/api-reference/checkouts/create-checkout
func (a *Paddle) Pay(ctx echo.Context, cfg *config.Pay) (*config.PayResponse, error) {
	// Step 1: 创建 Price
	priceResp, err := a.createPrice(cfg)
	if err != nil {
		return nil, err
	}
	priceID := priceResp.Data.(map[string]interface{})["id"].(string)

	// Step 2: 创建 Checkout Session
	checkoutResp, err := a.createCheckout(priceID, cfg)
	if err != nil {
		return nil, err
	}

	data := checkoutResp.Data.(map[string]interface{})
	redirectURL := ""
	if urlData, ok := data["url"]; ok {
		redirectURL, _ = urlData.(string)
	}
	tradeNo := ""
	if idData, ok := data["id"]; ok {
		tradeNo, _ = idData.(string)
	}

	result := &config.PayResponse{
		TradeNo:     tradeNo,
		RedirectURL: redirectURL,
		Raw:         checkoutResp,
	}
	return result, nil
}

// PayNotify 处理 Paddle Webhook
// 文档: https://developer.paddle.com/webhooks
// Paddle v2 使用 PWA (Paddle Webhook Agent) 或自托管验证
func (a *Paddle) PayNotify(ctx echo.Context) error {
	if err := a.VerifySign(ctx); err != nil {
		return ctx.JSON(echo.H{"error": err.Error()})
	}

	var payload struct {
		EventType string `json:"event_type"` // checkout.completed / transaction.completed / payment.refunded
		EventID   string `json:"event_id"`
		Data      json.RawMessage `json:"data"`
	}
	err := ctx.MustBind(&payload)
	if err != nil {
		return err
	}

	switch payload.EventType {
	case "transaction.completed":
		var txData struct {
			ID             string `json:"id"`
			Status          string `json:"status"`
			InvoiceNumber  string `json:"invoice_number"`
			CustomData     string `json:"custom_data,omitempty"`
			Details struct {
				Total struct {
					Amount   string `json:"amount"`
					Currency string `json:"currency"`
				} `json:"totals"`
			} `json:"details"`
		}
		json.Unmarshal(payload.Data, &txData)
		if a.notifyCallback != nil && txData.Status == "completed" {
			result := &config.Result{
				Operation:   config.OperationPayment,
				Status:      config.TradeStatusSuccess,
				TradeNo:     txData.ID,
				OutTradeNo:  txData.CustomData,
				Currency:    txData.Details.Total.Currency,
				TotalAmount: param.AsFloat64(txData.Details.Total.Amount),
				Raw:         &payload,
			}
			a.notifyCallback(ctx, result)
		}
	case "payment.refunded":
		var refData struct {
			ID          string `json:"id"`
			TransactionID string `json:"transaction_id"`
			Amount      string `json:"amount"`
			Currency    string `json:"currency"`
			Status      string `json:"status"`
		}
		json.Unmarshal(payload.Data, &refData)
		if a.notifyCallback != nil && refData.Status == "completed" {
			result := &config.Result{
				Operation:   config.OperationRefund,
				Status:      config.TradeStatusSuccess,
				TradeNo:     refData.TransactionID,
				RefundNo:    refData.ID,
				Currency:    refData.Currency,
				RefundFee:   param.AsFloat64(refData.Amount),
				Raw:         &payload,
			}
			a.notifyCallback(ctx, result)
		}
	}

	return ctx.JSON(echo.H{"status": "ok"})
}

// PayQuery 查询交易状态
// 文档: https://developer.paddle.com/api-reference/transactions/get-transaction
func (a *Paddle) PayQuery(ctx echo.Context, cfg *config.Query) (*config.Result, error) {
	var response struct {
		Data json.RawMessage `json:"data"`
	}

	resp, err := a.client().
		SetResult(&response).
		Get(a.apiURL + "/v2/transactions/" + cfg.TradeNo)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode() != http.StatusOK {
		return nil, fmt.Errorf("paddle: query failed %d", resp.StatusCode())
	}

	var txData struct {
		ID        string `json:"id"`
		Status    string `json:"status"`
		Details   struct {
			Total struct {
				Amount   string `json:"amount"`
				Currency string `json:"currency"`
			} `json:"totals"`
		} `json:"details"`
	}
	json.Unmarshal(response.Data, &txData)

	status := MappingStatus(txData.Status)

	return &config.Result{
		Operation:   config.OperationPayment,
		Status:      status,
		TradeNo:     txData.ID,
		Currency:    txData.Details.Total.Currency,
		TotalAmount: param.AsFloat64(txData.Details.Total.Amount),
		Raw:         &response,
	}, nil
}

// Refund 发起退款
// 文档: https://developer.paddle.com/api-reference/refunds/create-refund
func (a *Paddle) Refund(ctx echo.Context, cfg *config.Refund) (*config.Result, error) {
	body := map[string]interface{}{
		"transaction_id": cfg.TradeNo,
		"reason":         defaultString(cfg.RefundReason, "customer_request"),
		"amount":         fmt.Sprintf("%.2f", cfg.RefundAmount),
		"currency":       cfg.Currency.String(),
		"custom_data": map[string]string{
			"out_refund_no": cfg.OutRefundNo,
			"out_trade_no":  cfg.OutTradeNo,
		},
	}

	var response struct {
		Data json.RawMessage `json:"data"`
	}

	resp, err := a.client().
		SetResult(&response).
		SetBody(body).
		Post(a.apiURL + "/v2/refunds")
	if err != nil {
		return nil, err
	}
	if resp.StatusCode() != http.StatusOK && resp.StatusCode() != http.StatusCreated {
		return nil, fmt.Errorf("paddle: refund failed %d", resp.StatusCode())
	}

	var refData struct {
		ID          string `json:"id"`
		Status      string `json:"status"`
		Amount      string `json:"amount"`
		Currency    string `json:"currency"`
	}
	json.Unmarshal(response.Data, &refData)

	status := MapRefundStatus(refData.Status)

	return &config.Result{
		Operation:   config.OperationRefund,
		Status:      status,
		TradeNo:     cfg.TradeNo,
		OutTradeNo:  cfg.OutTradeNo,
		Currency:    refData.Currency,
		RefundFee:   param.AsFloat64(refData.Amount),
		RefundNo:    refData.ID,
		OutRefundNo: cfg.OutRefundNo,
		Raw:         &response,
	}, nil
}

// RefundNotify 通过 PayNotify 处理
func (a *Paddle) RefundNotify(ctx echo.Context) error {
	return a.PayNotify(ctx)
}

// RefundQuery 查询退款状态
// 文档: https://developer.paddle.com/api-reference/refunds/get-refund
func (a *Paddle) RefundQuery(ctx echo.Context, cfg *config.Query) (*config.Result, error) {
	var response struct {
		Data json.RawMessage `json:"data"`
	}

	resp, err := a.client().
		SetResult(&response).
		Get(a.apiURL + "/v2/refunds/" + cfg.RefundNo)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode() != http.StatusOK {
		return nil, fmt.Errorf("paddle: refund query failed %d", resp.StatusCode())
	}

	var refData struct {
		ID            string `json:"id"`
		Status        string `json:"status"`
		TransactionID string `json:"transaction_id"`
		Amount        string `json:"amount"`
		Currency      string `json:"currency"`
		CustomData    map[string]string `json:"custom_data"`
	}
	json.Unmarshal(response.Data, &refData)

	status := MapRefundStatus(refData.Status)
	outRefundNo := refData.CustomData["out_refund_no"]

	return &config.Result{
		Operation:   config.OperationRefund,
		Status:      status,
		TradeNo:     refData.TransactionID,
		OutTradeNo:  refData.CustomData["out_trade_no"],
		Currency:    refData.Currency,
		RefundFee:   param.AsFloat64(refData.Amount),
		RefundNo:    refData.ID,
		OutRefundNo: outRefundNo,
		Raw:         &response,
	}, nil
}

// --- 内部辅助方法 ---

type paddleAPIResponse struct {
	Data interface{} `json:"data"`
}

func (a *Paddle) createPrice(cfg *config.Pay) (*paddleAPIResponse, error) {
	body := map[string]interface{}{
		"product_name": defaultString(cfg.Subject, "Product"),
		"price": map[string]interface{}{
			"amount":   fmt.Sprintf("%.2f", cfg.Amount),
			"currency": strings.ToUpper(cfg.Currency.String()),
		},
		"description": defaultString(cfg.Subject, ""),
		"billing_period": map[string]interface{}{
			"interval": "day",
			"frequency": 1,
		},
		"trial_period": nil,
		"quantity": map[string]interface{}{
			"minimum": 1,
			"maximum": 99999,
		},
		"custom_data": map[string]string{
			"out_trade_no": cfg.OutTradeNo,
		},
	}

	var response paddleAPIResponse
	resp, err := a.client().
		SetResult(&response).
		SetBody(body).
		Post(a.apiURL + "/v2/prices")
	if err != nil {
		return nil, err
	}
	if resp.StatusCode() != http.StatusCreated {
		return nil, fmt.Errorf("paddle: create price failed %d", resp.StatusCode())
	}
	return &response, nil
}

func (a *Paddle) createCheckout(priceID string, cfg *config.Pay) (*paddleAPIResponse, error) {
	body := map[string]interface{}{
		"items": []map[string]interface{}{
			{
				"price_id": priceID,
				"quantity": 1,
			},
		},
		"settings": map[string]interface{}{
			"success_url": defaultString(cfg.ReturnURL, ""),
		},
		"custom_data": map[string]string{
			"out_trade_no": cfg.OutTradeNo,
		},
	}

	if len(cfg.CancelURL) > 0 {
		body["settings"].(map[string]interface{})["cancel_url"] = cfg.CancelURL
	}

	var response paddleAPIResponse
	resp, err := a.client().
		SetResult(&response).
		SetBody(body).
		Post(a.apiURL + "/v2/checkouts")
	if err != nil {
		return nil, err
	}
	if resp.StatusCode() != http.StatusCreated {
		return nil, fmt.Errorf("paddle: create checkout failed %d", resp.StatusCode())
	}
	return &response, nil
}
