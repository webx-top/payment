package config

import "github.com/webx-top/echo"

// NewResult 构造一个付款或退款结果数据实例
func NewResult() *Result {
	return &Result{}
}

const (
	// OperationPayment 付款操作
	OperationPayment = "payment"
	// OperationRefund 退款操作
	OperationRefund = "refund"
)

type PayResponse struct {
	RedirectURL    string
	QRCodeImageURL string
	QRCodeContent  string
	Params         echo.H
	Raw            interface{}
}

// Result 付款或退款结果数据
type Result struct {
	Operation              string  // 操作类型
	Status                 string  // 状态
	TradeNo                string  // 支付网关交易号
	OutTradeNo             string  // 业务方交易号
	TotalAmount            float64 // 订单总金额
	Currency               string  // 币种
	TransactionFeeValue    float64 // 交易手续费金额
	TransactionFeeCurrency string  // 交易手续费币种
	Reason                 string  // 失败原因
	PassbackParams         string  // 原样回传参数

	// - 退款数据 -

	OutRefundNo string        // 本地退款单号（退款时有效）
	RefundNo    string        // 支付网关退款号
	RefundFee   float64       // 退款金额（退款时有效）
	RefundItems []*RefundItem // 退款项列表

	// - 原始数据 -
	Raw interface{}
}

// AddRefundItem 添加退款项数据
func (r *Result) AddRefundItem(items ...*RefundItem) *Result {
	r.RefundItems = append(r.RefundItems, items...)
	return r
}

// GetRefundItem 获取退款项数据
func (r *Result) GetRefundItem(outRefundNo string, refundNo string) *RefundItem {
	if (len(outRefundNo) > 0 && r.OutRefundNo == outRefundNo) || (len(refundNo) > 0 && r.RefundNo == refundNo) {
		return &RefundItem{
			Status:      r.Status,
			OutRefundNo: r.OutRefundNo,
			RefundNo:    r.RefundNo,
			RefundFee:   r.RefundFee,
		}
	}
	for _, item := range r.RefundItems {
		if (len(outRefundNo) > 0 && item.OutRefundNo == outRefundNo) || (len(refundNo) > 0 && item.RefundNo == refundNo) {
			return item
		}
	}
	return nil
}

// NewRefundItem 构造一个退款项数据实例
func NewRefundItem() *RefundItem {
	return &RefundItem{}
}

// RefundItem 退款项数据
type RefundItem struct {
	Status      string  // 退款状态
	RefundFee   float64 // 退款金额
	OutRefundNo string  // 业务方退款单号
	RefundNo    string  // 支付网关退款号
}

func IsSuccess(status string) bool {
	return status == TradeStatusSuccess
}

func IsWaitPay(status string) bool {
	return status == TradeStatusWaitBuyerPay
}

func IsClosed(status string) bool {
	return status == TradeStatusClosed
}

func IsFinished(status string) bool {
	return status == TradeStatusFinished
}

// IsProcessing 是否退款中
func IsProcessing(status string) bool {
	return status == TradeStatusProcessing
}
