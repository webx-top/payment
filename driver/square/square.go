package square

import (
	"fmt"
	"net/http"

	"github.com/admpub/resty/v2"
	"github.com/webx-top/echo"
	"github.com/webx-top/payment"
	"github.com/webx-top/payment/config"
	"github.com/webx-top/restyclient"
)

const Name = `square`

var supports = config.Supports{
	config.SupportPayQuery,
	config.SupportRefund,
	config.SupportRefundNotify,
	config.SupportRefundQuery,
}

var SupportedCurrencies = []string{
	"USD", "CAD", "EUR", "GBP", "AUD", "JPY",
}

// Square API 版本: https://developer.squareup.com/reference/square/versioning
var apiVersion = "2024-04-17"

func init() {
	payment.Register(Name, echo.T(`Square`), New)
}

func New() payment.Driver {
	return &Square{}
}

type Square struct {
	account        *config.Account
	notifyCallback payment.NotifyCallback
	apiURL         string
}

func (a *Square) IsSupported(s config.Support) bool {
	return supports.IsSupported(s)
}

func (a *Square) SetNotifyCallback(callback payment.NotifyCallback) payment.Driver {
	a.notifyCallback = callback
	return a
}

func (a *Square) SetAccount(account *config.Account) payment.Driver {
	a.account = account
	a.apiURL = account.Options.Extra.String("apiURL")
	if len(a.apiURL) == 0 {
		a.apiURL = "https://connect.squareup.com"
	}
	return a
}

func (a *Square) client() *resty.Request {
	return restyclient.Retryable().
		SetHeader("Authorization", "Bearer "+a.account.AppSecret).
		SetHeader("Square-Version", apiVersion).
		SetHeader("Content-Type", "application/json")
}

// Pay 创建支付链接(使用 Square Payments API + Web Payment SDK 流程)
// 文档: https://developer.squareup.com/reference/square/payments-api/create-payment
func (a *Square) Pay(ctx echo.Context, cfg *config.Pay) (*config.PayResponse, error) {
	amount := MoneyFeeToString(cfg.Amount)

	body := map[string]interface{}{
		"source_id":      "", // 前端提交的 nonce/token，这里返回给前端自行处理
		"idempotency_key": payment.GenerateNonce(),
		"amount_money": map[string]interface{}{
			"amount":   AmountToCentInt(cfg.Amount),
			"currency": cfg.Currency.String(),
		},
		"reference_id": cfg.OutTradeNo,
		"note":         cfg.Subject,
		"autocomplete": true,
	}

	// Square 通常配合前端 SDK 使用（Web Payment SDK）
	// 返回必要参数让前端完成支付流程
	result := &config.PayResponse{
		Params: echo.H{
			"location_id":     a.account.AppID,
			"amount":          amount,
			"currency":        cfg.Currency.String(),
			"reference_id":    cfg.OutTradeNo,
			"idempotency_key": body["idempotency_key"],
		},
		Raw: body,
	}
	return result, nil
}

// PayNotify Square 通过 webhook 发送通知
// 文档: https://developer.square.com/docs/webhooks/overview
func (a *Square) PayNotify(ctx echo.Context) error {
	if err := a.VerifySign(ctx); err != nil {
		return ctx.JSON(echo.H{"error": err.Error()})
	}

	var payload struct {
		Type string `json:"type"`
		Data struct {
			Object struct {
				Payment *struct {
					ID            string `json:"id"`
					Status        string `json:"status"`
					AmountMoney   struct {
						Amount   int64  `json:"amount"`
						Currency string `json:"currency"`
					} `json:"amount_money"`
					OrderID     string `json:"order_id"`
					ReferenceID string `json:"reference_id"`
					CreatedAt   string `json:"created_at"`
				} `json:"payment"`
			} `json:"object"`
		} `json:"data"`
	}
	err := ctx.MustBind(&payload)
	if err != nil {
		return err
	}

	switch payload.Type {
	case "payment.created":
		// 支付已创建
	case "payment.updated":
	case "payment.captured":
		// 支付已捕获
	case "payment.completed":
		// 支付已完成
		if a.notifyCallback != nil && payload.Data.Object.Payment != nil {
			p := payload.Data.Object.Payment
			if p.Status == "COMPLETED" {
				result := &config.Result{
					Operation:   config.OperationPayment,
					Status:      config.TradeStatusSuccess,
					TradeNo:     p.ID,
					OutTradeNo:  p.ReferenceID,
					Currency:    p.AmountMoney.Currency,
					TotalAmount: CentToAmount(p.AmountMoney.Amount),
					Raw:         &payload,
				}
				a.notifyCallback(ctx, result)
			}
		}
	}

	return ctx.JSON(echo.H{"status": "ok"})
}

// PayQuery 查询支付状态
// 文档: https://developer.squareup.com/reference/square/payments-api/get-payment
func (a *Square) PayQuery(ctx echo.Context, cfg *config.Query) (*config.Result, error) {
	var response struct {
		Payment struct {
			ID            string `json:"id"`
			Status        string `json:"status"`
			AmountMoney   struct {
				Amount   int64  `json:"amount"`
				Currency string `json:"currency"`
			} `json:"amount_money"`
			ReferenceID string `json:"reference_id"`
			OrderID     string `json:"order_id"`
		} `json:"payment"`
	}

	resp, err := a.client().
		SetResult(&response).
		Get(a.apiURL + "/v2/payments/" + cfg.TradeNo)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode() != http.StatusOK {
		return nil, fmt.Errorf("square: query failed %d", resp.StatusCode())
	}

	p := response.Payment
	status := MappingStatus(p.Status)

	return &config.Result{
		Operation:   config.OperationPayment,
		Status:      status,
		TradeNo:     p.ID,
		OutTradeNo:  p.ReferenceID,
		Currency:    p.AmountMoney.Currency,
		TotalAmount: CentToAmount(p.AmountMoney.Amount),
		Raw:         &response,
	}, nil
}

// Refund 发起退款
// 文档: https://developer.squareup.com/reference/square/refunds-api/create-refund
func (a *Square) Refund(ctx echo.Context, cfg *config.Refund) (*config.Result, error) {
	body := map[string]interface{}{
		"idempotency_key": payment.GenerateNonce(),
		"amount_money": map[string]interface{}{
			"amount":   AmountToCentInt(cfg.RefundAmount),
			"currency": cfg.Currency.String(),
		},
		"payment_id":  cfg.TradeNo,
		"reason":      refundReasonString(cfg.RefundReason),
	}

	var response struct {
		Refund struct {
			ID           string `json:"id"`
			Status       string `json:"status"`
			AmountMoney  struct {
				Amount   int64  `json:"amount"`
				Currency string `json:"currency"`
			} `json:"amount_money"`
			PaymentID string `json:"payment_id"`
		} `json:"refund"`
	}

	resp, err := a.client().
		SetResult(&response).
		SetBody(body).
		Post(a.apiURL + "/v2/refunds")
	if err != nil {
		return nil, err
	}
	if resp.StatusCode() != http.StatusOK {
		return nil, fmt.Errorf("square: refund failed %d", resp.StatusCode())
	}

	r := response.Refund
	status := MapRefundStatus(r.Status)

	return &config.Result{
		Operation:   config.OperationRefund,
		Status:      status,
		TradeNo:     r.PaymentID,
		OutTradeNo:  cfg.OutTradeNo,
		Currency:    r.AmountMoney.Currency,
		RefundFee:   CentToAmount(r.AmountMoney.Amount),
		RefundNo:    r.ID,
		OutRefundNo: cfg.OutRefundNo,
		Raw:         &response,
	}, nil
}

// RefundNotify 退款通知处理
func (a *Square) RefundNotify(ctx echo.Context) error {
	if err := a.VerifySign(ctx); err != nil {
		return ctx.JSON(echo.H{"error": err.Error()})
	}

	var payload struct {
		Type string `json:"type"`
		Data struct {
			Object struct {
				Refund *struct {
					ID          string `json:"id"`
					Status      string `json:"status"`
					AmountMoney struct {
						Amount   int64  `json:"amount"`
						Currency string `json:"currency"`
					} `json:"amount_money"`
					PaymentID  string `json:"payment_id"`
				} `json:"refund"`
			} `json:"object"`
		} `json:"data"`
	}
	err := ctx.MustBind(&payload)
	if err != nil {
		return err
	}

	if payload.Type == "refund.updated" && a.notifyCallback != nil && payload.Data.Object.Refund != nil {
		r := payload.Data.Object.Refund
		if r.Status == "COMPLETED" {
			result := &config.Result{
				Operation:   config.OperationRefund,
				Status:      config.TradeStatusSuccess,
				TradeNo:     r.PaymentID,
				RefundNo:    r.ID,
				Currency:    r.AmountMoney.Currency,
				RefundFee:   CentToAmount(r.AmountMoney.Amount),
				Raw:         &payload,
			}
			a.notifyCallback(ctx, result)
		}
	}

	return ctx.JSON(echo.H{"status": "ok"})
}

// RefundQuery 查询退款状态
// 文档: https://developer.squareup.com/reference/square/refunds-api/get-refund
func (a *Square) RefundQuery(ctx echo.Context, cfg *config.Query) (*config.Result, error) {
	var response struct {
		Refund struct {
			ID           string `json:"id"`
			Status       string `json:"status"`
			AmountMoney  struct {
				Amount   int64  `json:"amount"`
				Currency string `json:"currency"`
			} `json:"amount_money"`
			PaymentID string `json:"payment_id"`
			Reason    string `json:"reason"`
		} `json:"refund"`
	}

	resp, err := a.client().
		SetResult(&response).
		Get(a.apiURL + "/v2/refunds/" + cfg.RefundNo)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode() != http.StatusOK {
		return nil, fmt.Errorf("square: refund query failed %d", resp.StatusCode())
	}

	r := response.Refund
	status := MapRefundStatus(r.Status)

	return &config.Result{
		Operation:   config.OperationRefund,
		Status:      status,
		TradeNo:     r.PaymentID,
		OutTradeNo:  cfg.OutTradeNo,
		Currency:    r.AmountMoney.Currency,
		RefundFee:   CentToAmount(r.AmountMoney.Amount),
		RefundNo:    r.ID,
		OutRefundNo: cfg.OutRefundNo,
		Reason:      r.Reason,
		Raw:         &response,
	}, nil
}
