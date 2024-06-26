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
	Signature   string  `json:"signature" validate:"required"`
	RedirectUrl string  `json:"redirect_url"`
	Timestamp   int64   `json:"timestamp" validate:"required"`
}

func (c *CreateTransactionRequest) URLValues() url.Values {
	return url.Values{
		"order_id":     []string{c.OrderId},
		"amount":       []string{fmt.Sprint(c.Amount)},
		"notify_url":   []string{c.NotifyUrl},
		"redirect_url": []string{c.RedirectUrl},
		"timestamp":    []string{fmt.Sprint(c.Timestamp)},
	}
}

// QueryTransactionRequest 查询交易请求
type QueryTransactionRequest struct {
	TradeId   string `json:"trade_id" validate:"required|maxLen:32"`
	Timestamp int64  `json:"timestamp" validate:"required"`
	Signature string `json:"signature"  validate:"required"`
}

func (c *QueryTransactionRequest) URLValues() url.Values {
	return url.Values{
		"trade_id":  []string{c.TradeId},
		"timestamp": []string{fmt.Sprint(c.Timestamp)},
	}
}
