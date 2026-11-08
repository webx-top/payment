package wechat

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"

	wechatv3 "github.com/go-pay/gopay/wechat/v3"
	"github.com/webx-top/echo"
	"github.com/webx-top/payment"
	"github.com/webx-top/payment/config"
)

// MoneyFeeToInt64 微信金额浮点转整数(单位:分)
func MoneyFeeToInt64(moneyFee float64) int64 {
	i, _ := strconv.ParseInt(payment.MulFloat(moneyFee, 100, 0), 10, 64)
	return i
}

// MappingTradeState 支付状态映射
/*
SUCCESS—支付成功
REFUND—转入退款
NOTPAY—未支付
CLOSED—已关闭
REVOKED—已撤销（付款码支付）
USERPAYING--用户支付中（付款码支付）
PAYERROR--支付失败(其他原因，如银行返回失败)
*/
func MappingTradeState(tradeState string) string {
	switch tradeState {
	case wechatv3.TradeStateSuccess:
		return config.TradeStatusSuccess
	case wechatv3.TradeStateRefund, wechatv3.TradeStateClosed:
		return config.TradeStatusClosed
	case wechatv3.TradeStateNoPay, wechatv3.TradeStateRevoked, wechatv3.TradeStatePaying, wechatv3.TradeStatePayError:
		return config.TradeStatusWaitBuyerPay
	}
	return tradeState
}

// MappingRefundStatus 退款状态映射
/*
SUCCESS—退款成功
CLOSED—退款关闭
PROCESSING—退款处理中
ABNORMAL—退款异常
*/
func MappingRefundStatus(refundStatus string) string {
	switch refundStatus {
	case `SUCCESS`:
		return config.TradeStatusSuccess
	case `CLOSED`:
		return config.TradeStatusClosed
	case `PROCESSING`:
		return config.TradeStatusProcessing
	case `ABNORMAL`:
		return config.TradeStatusException
	}
	return refundStatus
}

// checkV3Rsp 检查V3接口响应错误
func checkV3Rsp(code int, errRsp *wechatv3.ErrResponse, errMsg string) error {
	if code == wechatv3.Success {
		return nil
	}
	if errRsp != nil && len(errRsp.Code) > 0 {
		return fmt.Errorf(`wechat: %s: %s`, errRsp.Code, errRsp.Message)
	}
	return errors.New(errMsg)
}

// paramsToStore 结构体转 echo.H
func paramsToStore(v interface{}) echo.H {
	h := echo.H{}
	b, err := json.Marshal(v)
	if err != nil {
		return h
	}
	_ = json.Unmarshal(b, &h)
	return h
}
