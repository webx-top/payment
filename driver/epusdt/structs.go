package epusdt

import (
	"fmt"
	"net/url"
)

// CreateTransactionRequest 创建交易请求
type CreateTransactionRequest struct {
	OrderId     string  `json:"order_id" validate:"required|maxLen:32"`
	Amount      float64 `json:"amount" validate:"required|isFloat|gt:0.01"`
	NotifyUrl   string  `json:"notify_url" validate:"required"`
	Signature   string  `json:"signature"  validate:"required"`
	RedirectUrl string  `json:"redirect_url"`
}

func (c *CreateTransactionRequest) URLValues() url.Values {
	return url.Values{
		"order_id":     []string{c.OrderId},
		"amount":       []string{fmt.Sprint(c.Amount)},
		"notify_url":   []string{c.NotifyUrl},
		"redirect_url": []string{c.RedirectUrl},
	}
}

type Response struct {
	StatusCode int                        `json:"status_code"`
	Message    string                     `json:"message"`
	Data       *CreateTransactionResponse `json:"data"`
	RequestID  string                     `json:"request_id"`
}

// CreateTransactionResponse 创建订单成功返回
type CreateTransactionResponse struct {
	TradeId        string  `json:"trade_id"`        //  epusdt订单号
	OrderId        string  `json:"order_id"`        //  客户交易id
	Amount         float64 `json:"amount"`          //  订单金额，保留4位小数
	ActualAmount   float64 `json:"actual_amount"`   //  订单实际需要支付的金额，保留4位小数
	Token          string  `json:"token"`           //  收款钱包地址
	ExpirationTime int64   `json:"expiration_time"` // 过期时间 时间戳
	PaymentUrl     string  `json:"payment_url"`     // 收银台地址
}

// OrderNotifyResponse 订单异步回调结构体
type OrderNotifyResponse struct {
	TradeId            string  `json:"trade_id"`             //  epusdt订单号
	OrderId            string  `json:"order_id"`             //  客户交易id
	Amount             float64 `json:"amount"`               //  订单金额，保留4位小数
	ActualAmount       float64 `json:"actual_amount"`        //  订单实际需要支付的金额，保留4位小数
	Token              string  `json:"token"`                //  收款钱包地址
	BlockTransactionId string  `json:"block_transaction_id"` // 区块id
	Signature          string  `json:"signature"`            // 签名
	Status             int     `json:"status"`               //  1：等待支付，2：支付成功，3：已过期
}

const (
	StatusWaitPay = 1
	StatusPaid    = 2
	StatusExpired = 3
)

func (c *OrderNotifyResponse) URLValues() url.Values {
	return url.Values{
		"trade_id":             []string{c.TradeId},
		"order_id":             []string{c.OrderId},
		"amount":               []string{fmt.Sprint(c.Amount)},
		"actual_amount":        []string{fmt.Sprint(c.ActualAmount)},
		"token":                []string{c.Token},
		"block_transaction_id": []string{c.BlockTransactionId},
		"status":               []string{fmt.Sprint(c.Status)},
	}
}

func (c *OrderNotifyResponse) Verify(token string) bool {
	return c.Signature == GenerateSign(c.URLValues(), token)
}
