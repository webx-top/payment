package authorize

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"net/http"

	"github.com/webx-top/echo"
	"github.com/webx-top/echo/param"
	"github.com/webx-top/payment"
	"github.com/webx-top/payment/config"
	"github.com/webx-top/restyclient"
)

const Name = `authorize`

var supports = config.Supports{
	config.SupportPayNotify,
	config.SupportPayQuery,
	config.SupportRefund,
	config.SupportRefundQuery,
}

var SupportedCurrencies = []string{"USD"} // Authorize.Net 主要支持 USD

func init() {
	payment.Register(Name, echo.T(`Authorize.Net`), New)
}

func New() payment.Driver {
	return &Authorize{}
}

type Authorize struct {
	account        *config.Account
	notifyCallback payment.NotifyCallback
	apiURL         string
	loginID        string
	transactionKey string
}

func (a *Authorize) IsSupported(s config.Support) bool {
	return supports.IsSupported(s)
}

func (a *Authorize) SetNotifyCallback(callback payment.NotifyCallback) payment.Driver {
	a.notifyCallback = callback
	return a
}

func (a *Authorize) SetAccount(account *config.Account) payment.Driver {
	a.account = account
	a.loginID = a.account.AppID
	a.transactionKey = a.account.AppSecret
	if a.account.Debug {
		a.apiURL = "apitest.authorize.net/xml/v1/request.api" // 沙箱
	} else {
		a.apiURL = "api.authorize.net/xml/v1/request.api" // 生产
	}
	return a
}

// Pay 创建交易(Authorize.Net JSON API)
// 文档: https://developer.authorize.com/api/reference/index.html#payment-transactions
// Authorize.Net 通常配合前端表单收集卡号信息，这里返回必要参数
func (a *Authorize) Pay(ctx echo.Context, cfg *config.Pay) (*config.PayResponse, error) {
	// Authorize.Net 支持两种模式：
	// 1. Accept.js/AcceptUI (前端SDK) - 推荐
	// 2. 直接API调用(需要PCI合规)

	// 返回参数给前端使用 Accept.js SDK
	result := &config.PayResponse{
		Params: echo.H{
			"login_id":        a.loginID,
			"client_key":      a.account.Options.Extra.String("clientKey"), // 公钥用于 Accept.js
			"amount":          MoneyFeeToString(cfg.Amount),
			"currency":        cfg.Currency.String(),
			"invoice_number":  cfg.OutTradeNo,
			"description":     defaultString(cfg.Subject, "Purchase"),
		},
		Raw: map[string]interface{}{
			"mode": "acceptjs",
		},
	}
	return result, nil
}

// PayNotify 处理 Silent Post / Webhook 通知
// 文档: https://developer.authorize.com/api/reference/features/authentication.html#Silent_Post
func (a *Authorize) PayNotify(ctx echo.Context) error {
	responseCode := ctx.Form("x_response_code")
	transID := ctx.Form("x_trans_id")
	invoiceNum := ctx.Form("x_invoice_num")
	amountStr := ctx.Form("x_amount")
	md5Hash := ctx.Form("x_md5_hash")

	// MD5 Hash 验证
	if !a.verifyMD5Hash(md5Hash, transID, amountStr) {
		return ctx.String("error: invalid signature")
	}

	if a.notifyCallback != nil {
		status := config.TradeStatusException
		switch responseCode {
		case "1": // Approved
			status = config.TradeStatusSuccess
		case "2": // Declined
			status = config.TradeStatusClosed
		case "3": // Error
			status = config.TradeStatusException
		case "4": // Held for Review
			status = config.TradeStatusProcessing
		}

		result := &config.Result{
			Operation:   config.OperationPayment,
			Status:      status,
			TradeNo:     transID,
			OutTradeNo:  invoiceNum,
			Currency:    "USD",
			TotalAmount: param.AsFloat64(amountStr),
		}
		a.notifyCallback(ctx, result)
	}

	return ctx.String("")
}

// PayQuery 查询交易状态
// 文档: https://developer.authorize.com/api/reference/index.html#get-transaction-details-request
func (a *Authorize) PayQuery(ctx echo.Context, cfg *config.Query) (*config.Result, error) {
	body := map[string]interface{}{
		"getTransactionDetailsRequest": map[string]interface{}{
			"merchantAuthentication": a.merchantAuth(),
			"transId":               cfg.TradeNo,
		},
	}

	var response getTransactionDetailsResponse
	resp, err := restyclient.Retryable().
		SetResult(&response).
		SetBody(body).
		Post(a.apiURL)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode() != http.StatusOK {
		return nil, fmt.Errorf("authorize: query failed %d", resp.StatusCode())
	}

	tx := response.Transaction
	status := MappingStatus(tx.ResponseCode)

	return &config.Result{
		Operation:   config.OperationPayment,
		Status:      status,
		TradeNo:     tx.TransID,
		OutTradeNo:  tx.InvoiceNumber,
		Currency:    tx.Currency,
		TotalAmount: param.AsFloat64(tx.Amount),
		Raw:         &response,
	}, nil
}

// Refund 发起退款
// 文档: https://developer.authorize.com/api/reference/index.html#refund-a-transaction
func (a *Authorize) Refund(ctx echo.Context, cfg *config.Refund) (*config.Result, error) {
	body := map[string]interface{}{
		"createTransactionRequest": map[string]interface{}{
			"merchantAuthentication": a.merchantAuth(),
			"transactionType":       "refundTransaction",
			"amount":                MoneyFeeToString(cfg.RefundAmount),
			"payment": map[string]interface{}{
				"creditCard": map[string]interface{}{
					"cardNumber":     "XXXX" + lastFourDigits(cfg.TradeNo), // 需要原卡号后四位
					"expirationDate": "XXXX",
				},
			},
			"refTransId":           cfg.TradeNo,
		},
	}

	var response createTransactionResponse
	resp, err := restyclient.Retryable().
		SetResult(&response).
		SetBody(body).
		Post(a.apiURL)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode() != http.StatusOK {
		return nil, fmt.Errorf("authorize: refund failed %d", resp.StatusCode())
	}

	tx := response.Transaction
	status := MapRefundStatus(tx.ResponseCode)

	return &config.Result{
		Operation:   config.OperationRefund,
		Status:      status,
		TradeNo:     tx.TransID,
		OutTradeNo:  cfg.OutTradeNo,
		Currency:    tx.Currency,
		RefundFee:   param.AsFloat64(tx.Amount),
		RefundNo:    tx.TransID, // Authorize.Net 退款复用同一trans ID
		OutRefundNo: cfg.OutRefundNo,
		Raw:         &response,
	}, nil
}

// RefundNotify 通过 PayNotify 统一处理
func (a *Authorize) RefundNotify(ctx echo.Context) error {
	return a.PayNotify(ctx)
}

// RefundQuery 查询退款(通过查询原始交易获取退款信息)
func (a *Authorize) RefundQuery(ctx echo.Context, cfg *config.Query) (*config.Result, error) {
	// Authorize.Net 的退款查询和普通交易查询相同
	return a.PayQuery(ctx, cfg)
}

// --- 内部方法 ---

func (a *Authorize) merchantAuth() map[string]string {
	return map[string]string{
		"name":         a.loginID,
		"transactionKey": a.transactionKey,
	}
}

// verifyMD5Hash 验证 Authorize.Net MD5 Hash
// 格式: md5(loginID^transID^amount^md5_value_of_transaction_key)
func (a *Authorize) verifyMD5Hash(hash, transID, amount string) bool {
	md5Key := a.account.Options.Extra.String("md5Key")
	if len(md5Key) == 0 { return true } // 未配置则跳过验证

	raw := fmt.Sprintf("%s^%s^%s^%s", a.loginID, transID, amount, md5Key)
	hasher := md5.Sum([]byte(raw))
	computed := hex.EncodeToString(hasher[:])
	return computed == hash
}

func lastFourDigits(id string) string {
	if len(id) >= 4 { return id[len(id)-4:] }
	return "0000"
}

// --- 响应结构体 ---

type transactionDetail struct {
	TransID       string `json:"transId"`
	ResponseCode  string `json:"responseCode"`
	ResponseType  string `json:"responseType"`
	Amount        string `json:"amount"`
	Currency      string `json:"currency"`
	InvoiceNumber string `json:"invoice_number"`
	SettleAmount  string `json:"settleAmount"`
	SubmitTimeUTC string `json:"submitTimeLocal"`
	Payment       struct {
		CreditCard struct {
			CardNumber     string `json:"cardNumber"`
			CreditCardType string `json:"creditCardType"`
		} `json:"creditCard"`
	} `json:"payment"`
}

type getTransactionDetailsResponse struct {
	Transaction transactionDetail `json:"transaction"`
	Messages    struct {
		ResultCode string `json:"resultCode"`
		Message    []struct {
			Code string `json:"code"`
			Text string `json:"text"`
		} `json:"message"`
	} `json:"messages"`
}

type createTransactionResponse struct {
	Transaction transactionDetail `json:"transaction"`
	Messages    struct {
		ResultCode string `json:"resultCode"`
		Message    []struct {
			Code string `json:"code"`
			Text string `json:"text"`
		} `json:"message"`
	} `json:"messages"`
}
