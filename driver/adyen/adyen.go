package adyen

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/admpub/resty/v2"
	"github.com/webx-top/echo"
	"github.com/webx-top/payment"
	"github.com/webx-top/payment/config"
	"github.com/webx-top/restyclient"
)

const Name = `adyen`

var supports = config.Supports{
	config.SupportPayNotify,
	config.SupportPayQuery,
	config.SupportRefund,
	config.SupportRefundQuery,
}

var SupportedCurrencies = []string{
	"USD", "EUR", "GBP", "AUD", "CAD", "CHF",
	"CNY", "HKD", "JPY", "MXN", "SGD", "SEK",
	"NOK", "DKK", "PLN", "INR", "BRL", "ZAR",
}

func init() {
	payment.Register(Name, echo.T(`Adyen`), New)
}

func New() payment.Driver {
	return &Adyen{}
}

type Adyen struct {
	account        *config.Account
	notifyCallback payment.NotifyCallback
	apiURL         string
	clientKey      string
	hmacKey        string
}

func (a *Adyen) IsSupported(s config.Support) bool {
	return supports.IsSupported(s)
}

func (a *Adyen) SetNotifyCallback(callback payment.NotifyCallback) payment.Driver {
	a.notifyCallback = callback
	return a
}

func (a *Adyen) SetAccount(account *config.Account) payment.Driver {
	a.account = account
	a.apiURL = account.Options.Extra.String("apiURL")
	if len(a.apiURL) == 0 {
		if a.account.Debug {
			a.apiURL = "https://checkout-test.adyen.com"
		} else {
			a.apiURL = "https://checkout.adyen.com"
		}
	}
	a.clientKey = a.account.AppID
	a.hmacKey = account.Options.Extra.String("hmacKey") // 用于webhook签名验证
	return a
}

func (a *Adyen) client() *resty.Request {
	return restyclient.Retryable().
		SetHeader("X-API-Key", a.account.AppSecret).
		SetHeader("Content-Type", "application/json").
		SetQueryParam("clientKey", a.clientKey)
}

// Pay 创建 Adyen Checkout Session
// 文档: https://docs.adyen.com/api-explorer/Checkout/Session/post
func (a *Adyen) Pay(ctx echo.Context, cfg *config.Pay) (*config.PayResponse, error) {
	body := map[string]interface{}{
		"merchantAccount": a.account.MerchantID,
		"amount": map[string]interface{}{
			"value":    AmountToMinor(cfg.Amount, cfg.Currency.String()),
			"currency": cfg.Currency.String(),
		},
		"reference":     cfg.OutTradeNo,
		"returnUrl":     defaultString(cfg.ReturnURL, ""),
		"shopperLocale": "en_US",
		"lineItems": []map[string]interface{}{
			{
				"description":        defaultString(cfg.Subject, ""),
				"amountExcludingTax": AmountToMinor(cfg.Amount, cfg.Currency.String()),
				"amountIncludingTax": AmountToMinor(cfg.Amount, cfg.Currency.String()),
				"taxAmount":          0,
				"quantity":           1,
				"taxPercentage":      0,
			},
		},
	}
	if len(cfg.NotifyURL) > 0 {
		body["notificationUrl"] = cfg.NotifyURL
	}
	if len(cfg.CancelURL) > 0 {
		body["cancelUrl"] = cfg.CancelURL
	}

	var response struct {
		ID              string `json:"id"`
		SessionData     string `json:"sessionData"`
		CheckoutSession struct {
			ID string `json:"id"`
		} `json:"checkoutSession"`
	}

	resp, err := a.client().
		SetResult(&response).
		SetBody(body).
		Post(a.apiURL + "/v68/sessions")
	if err != nil {
		return nil, err
	}
	if resp.StatusCode() != http.StatusOK {
		var apiErr struct {
			Message   string `json:"message"`
			ErrorCode string `json:"errorCode"`
		}
		json.Unmarshal(resp.Body(), &apiErr)
		return nil, fmt.Errorf("adyen: %s - %s", apiErr.ErrorCode, apiErr.Message)
	}

	result := &config.PayResponse{
		TradeNo: response.ID,
		Params: echo.H{
			"id":           response.ID,
			"session_data": response.SessionData,
		},
		Raw: &response,
	}
	return result, nil
}

// PayNotify 处理 Adyen Webhook/Notification
// 文档: https://docs.adyen.com/development-resources/webhooks/verify-notifications
func (a *Adyen) PayNotify(ctx echo.Context) error {
	// 复用 parseNotifications，不再重复 MustBind
	notifications, err := a.parseNotifications(ctx)
	if err != nil {
		return ctx.JSON(echo.H{"error": err.Error()})
	}
	if err = a.verifySign(notifications); err != nil {
		return ctx.JSON(echo.H{"error": err.Error()})
	}
	for _, n := range notifications {
		switch n.EventCode {
		case "AUTHORISATION":
			if a.notifyCallback != nil && n.Success {
				status := config.TradeStatusSuccess
				result := &config.Result{
					Operation:   config.OperationPayment,
					Status:      status,
					TradeNo:     n.PspReference,
					OutTradeNo:  n.MerchantReference,
					Currency:    n.Currency,
					TotalAmount: MinorToAmount(n.Value),
					Raw:         &n,
				}
				a.notifyCallback(ctx, result)
			}
		case "REFUND":
			if a.notifyCallback != nil && n.Success {
				refundNo := n.AdditionalData["refund.pspspReference"]
				result := &config.Result{
					Operation:  config.OperationRefund,
					Status:     config.TradeStatusSuccess,
					TradeNo:    n.PspReference,
					OutTradeNo: n.MerchantReference,
					Currency:   n.Currency,
					RefundFee:  MinorToAmount(n.Value),
					RefundNo:   refundNo,
					Raw:        &n,
				}
				a.notifyCallback(ctx, result)
			}
		case "CANCELLATION":
		case "CAPTURE":
		case "REFUND_FAILED":
		}
	}

	// Adyen 需要返回 [accepted] 响应
	return ctx.String("[accepted]")
}

// PayQuery 查询支付状态(通过 Payment API)
// 文档: https://docs.adyen.com/api-explorer/Payment/ThirdParty/post
func (a *Adyen) PayQuery(ctx echo.Context, cfg *config.Query) (*config.Result, error) {
	body := map[string]interface{}{
		"merchantAccount": a.account.MerchantID,
		"pspReference":    cfg.TradeNo,
	}

	var response struct {
		ResultCode        string `json:"resultCode"`
		AdditionalDetails []struct {
			Key   string `json:"key"`
			Value string `json:"value"`
		} `json:"additionalDetails"`
	}

	resp, err := a.client().
		SetResult(&response).
		SetBody(body).
		Post(a.apiURL + "/v68/payments/" + cfg.TradeNo + "/result")
	if err != nil {
		return nil, err
	}
	if resp.StatusCode() != http.StatusOK {
		return nil, fmt.Errorf("adyen: query failed %d", resp.StatusCode())
	}

	status := MappingStatus(response.ResultCode)

	return &config.Result{
		Operation:  config.OperationPayment,
		Status:     status,
		TradeNo:    cfg.TradeNo,
		OutTradeNo: cfg.OutTradeNo,
		Raw:        &response,
	}, nil
}

// Refund 发起退款
// 文档: https://docs.adyen.com/api-explorer/Payment/modification/post-refund
func (a *Adyen) Refund(ctx echo.Context, cfg *config.Refund) (*config.Result, error) {
	body := map[string]interface{}{
		"merchantAccount": a.account.MerchantID,
		"amount": map[string]interface{}{
			"value":    AmountToMinor(cfg.RefundAmount, cfg.Currency.String()),
			"currency": cfg.Currency.String(),
		},
		"reference":         cfg.OutRefundNo,
		"originalReference": cfg.TradeNo,
	}

	var response struct {
		PspReference string `json:"pspReference"`
		ResultCode   string `json:"resultCode"`
		RefundReason string `json:"refundReason"`
	}

	resp, err := a.client().
		SetResult(&response).
		SetBody(body).
		Post(a.apiURL + "/v68/refund")
	if err != nil {
		return nil, err
	}
	if resp.StatusCode() != http.StatusOK {
		return nil, fmt.Errorf("adyen: refund failed %d", resp.StatusCode())
	}

	status := MapRefundStatus(response.ResultCode)

	return &config.Result{
		Operation:   config.OperationRefund,
		Status:      status,
		TradeNo:     cfg.TradeNo,
		OutTradeNo:  cfg.OutTradeNo,
		Currency:    cfg.Currency.String(),
		RefundFee:   MinorToAmount(AmountToMinor(cfg.RefundAmount, cfg.Currency.String())),
		RefundNo:    response.PspReference,
		OutRefundNo: cfg.OutRefundNo,
		Reason:      response.RefundReason,
		Raw:         &response,
	}, nil
}

// RefundNotify 退款通知(通过统一 PayNotify 处理)
func (a *Adyen) RefundNotify(ctx echo.Context) error {
	return a.PayNotify(ctx)
}

// RefundQuery 查询退款状态
func (a *Adyen) RefundQuery(ctx echo.Context, cfg *config.Query) (*config.Result, error) {
	body := map[string]interface{}{
		"merchantAccount":   a.account.MerchantID,
		"originalReference": cfg.TradeNo,
	}

	var response struct {
		ResultCode   string `json:"resultCode"`
		PspReference string `json:"pspReference"`
		Amount       struct {
			Value    int64  `json:"value"`
			Currency string `json:"currency"`
		} `json:"amount"`
	}

	resp, err := a.client().
		SetResult(&response).
		SetBody(body).
		Get(a.apiURL + "/v68/payment/" + cfg.TradeNo + "/refundResult?pspReference=" + cfg.RefundNo)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode() != http.StatusOK {
		return nil, fmt.Errorf("adyen: refund query failed %d", resp.StatusCode())
	}

	status := MapRefundStatus(response.ResultCode)

	return &config.Result{
		Operation:   config.OperationRefund,
		Status:      status,
		TradeNo:     cfg.TradeNo,
		OutTradeNo:  cfg.OutTradeNo,
		Currency:    response.Amount.Currency,
		RefundFee:   MinorToAmount(response.Amount.Value),
		RefundNo:    response.PspReference,
		OutRefundNo: cfg.OutRefundNo,
		Raw:         &response,
	}, nil
}
