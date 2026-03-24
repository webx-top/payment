package mockpay

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/admpub/log"
	"github.com/webx-top/com"
	"github.com/webx-top/echo"
	"github.com/webx-top/echo/param"
	"github.com/webx-top/payment"
	"github.com/webx-top/payment/config"
)

const Name = `mockpay`

func init() {
	payment.Register(Name, echo.T(`模拟支付`), New)
}

func New() payment.Driver {
	return &Mockpay{}
}

type Mockpay struct {
	account        *config.Account
	notifyCallback payment.NotifyCallback
	features       config.Supports
}

func (a *Mockpay) IsSupported(s config.Support) bool {
	return a.getFeatures().IsSupported(s)
}

func (a *Mockpay) SetNotifyCallback(callback payment.NotifyCallback) payment.Driver {
	a.notifyCallback = callback
	return a
}

func (a *Mockpay) SetAccount(account *config.Account) payment.Driver {
	a.account = account
	return a
}

func (a *Mockpay) Pay(ctx echo.Context, cfg *config.Pay) (*config.PayResponse, error) {
	delay, err := a.getNoticeDelay()
	if err != nil {
		return nil, err
	}
	device := cfg.Device.String()
	if len(device) > 0 {
		var supportDevices []string
		if optionValue := a.getOptionValue(`supportDevices`, cfg); len(optionValue) > 0 {
			supportDevices = strings.Split(optionValue, `,`)
		}
		if len(supportDevices) > 0 && !com.InSlice(device, supportDevices) {
			return nil, config.ErrUnknownDevice
		}
	}
	tradeNo := fmt.Sprintf(`MOCKPAY%d%s`, time.Now().UnixMilli(), com.RandomAlphanumeric(5))
	result := &config.PayResponse{
		TradeNo:     tradeNo,
		RedirectURL: cfg.ReturnURL,
		Params:      echo.H{},
	}
	if delay > 0 {
		result.Params.Set(`delay`, uint(delay.Seconds()))
	}
	err = a.delaySubmitPayNotice(*a.account, *cfg, tradeNo, delay)
	return result, err
}

func (a *Mockpay) PayNotify(ctx echo.Context) error {
	if !a.IsSupported(config.SupportPayNotify) {
		return ctx.String(config.ErrUnsupported.Error(), http.StatusNotImplemented)
	}
	formData := url.Values(ctx.Forms())
	status := formData.Get(`status`)
	formHash := formData.Get(`hash`)
	hashString := GenerateHash(formData, a.account.AppSecret)
	if formHash != hashString {
		return ctx.String(config.ErrSignature.Error())
	}
	var isSuccess = true
	if a.notifyCallback != nil {
		result := &config.Result{
			Operation:      config.OperationPayment,
			Status:         status,
			TradeNo:        formData.Get(`trade_no`),
			OutTradeNo:     formData.Get(`out_trade_no`),
			PassbackParams: formData.Get(`passback_params`),
			Currency:       ``,
			TotalAmount:    param.AsFloat64(formData.Get(`total_amount`)),
			Reason:         formData.Get(`reason`),
			Raw:            formData,
		}
		if err := a.notifyCallback(ctx, result); err != nil {
			isSuccess = false
		}
	}
	if isSuccess {
		return ctx.String(`success`)
	}
	return ctx.String(`failed`)
}

func (a *Mockpay) PayQuery(ctx echo.Context, cfg *config.Query) (*config.Result, error) {
	if !a.IsSupported(config.SupportPayQuery) {
		return nil, config.ErrUnsupported
	}
	data, err := getCachedPayData(`pay.` + cfg.TradeNo)
	if err != nil {
		return nil, err
	}
	queryStatus := a.getOptionValue(`queryStatus`, nil)
	if len(queryStatus) == 0 {
		queryStatus = config.TradeStatusSuccess
	}
	return &config.Result{
		Operation:   config.OperationPayment,
		Status:      queryStatus,
		TradeNo:     cfg.TradeNo,
		OutTradeNo:  cfg.OutTradeNo,
		Currency:    data.Currency,
		TotalAmount: data.TotalAmount,
		Reason:      ``,
	}, err
}

func (a *Mockpay) Refund(ctx echo.Context, cfg *config.Refund) (*config.Result, error) {
	if !a.IsSupported(config.SupportRefund) {
		return nil, config.ErrUnsupported
	}
	refundNo := fmt.Sprintf(`MOCKREFUND%d%s`, time.Now().UnixMilli(), com.RandomAlphanumeric(5))
	err := a.delaySubmitRefundNotice(*a.account, *cfg, refundNo)
	return &config.Result{
		Operation:   config.OperationRefund,
		Status:      config.TradeStatusSuccess,
		TradeNo:     cfg.TradeNo,
		OutTradeNo:  cfg.OutTradeNo,
		Currency:    cfg.Currency.String(),
		TotalAmount: 0,
		Reason:      ``,
		RefundNo:    refundNo,
		RefundFee:   cfg.RefundAmount,
		OutRefundNo: cfg.OutRefundNo,
	}, err
}

func (a *Mockpay) RefundNotify(ctx echo.Context) error {
	if !a.IsSupported(config.SupportRefund) || !a.IsSupported(config.SupportRefundNotify) {
		return ctx.String(config.ErrUnsupported.Error(), http.StatusNotImplemented)
	}
	formData := url.Values(ctx.Forms())
	status := formData.Get(`status`)
	formHash := formData.Get(`hash`)
	hashString := GenerateHash(formData, a.account.AppSecret)
	if formHash != hashString {
		return ctx.String(config.ErrSignature.Error())
	}
	var isSuccess = true
	if a.notifyCallback != nil {
		result := &config.Result{
			Operation:   config.OperationRefund,
			Status:      status,
			TradeNo:     formData.Get(`trade_no`),
			OutTradeNo:  formData.Get(`out_trade_no`),
			Currency:    formData.Get(`currency`),
			TotalAmount: param.AsFloat64(formData.Get(`total_amount`)),
			Reason:      formData.Get(`reason`),
			RefundFee:   param.AsFloat64(formData.Get(`refund_fee`)),
			RefundNo:    formData.Get(`refund_no`),
			OutRefundNo: formData.Get(`out_refund_no`),
			Raw:         formData,
		}
		if err := a.notifyCallback(ctx, result); err != nil {
			log.Error(err)
			isSuccess = false
		}
	}
	if isSuccess {
		return ctx.String(`success`)
	}
	return ctx.String(`failed`)
}

func (a *Mockpay) RefundQuery(ctx echo.Context, cfg *config.Query) (*config.Result, error) {
	if !a.IsSupported(config.SupportRefund) || !a.IsSupported(config.SupportRefundQuery) {
		return nil, config.ErrUnsupported
	}
	data, err := getCachedRefundData(`refund.` + cfg.RefundNo)
	if err != nil {
		return nil, err
	}
	queryStatus := a.getOptionValue(`queryStatus`, nil)
	if len(queryStatus) == 0 {
		queryStatus = config.TradeStatusSuccess
	}
	return &config.Result{
		Operation:   config.OperationRefund,
		Status:      queryStatus,
		TradeNo:     cfg.TradeNo,
		OutTradeNo:  cfg.OutTradeNo,
		Currency:    data.Currency,
		TotalAmount: data.TotalAmount,
		Reason:      ``,
		RefundFee:   data.RefundFee,
		RefundNo:    data.RefundNo,
		OutRefundNo: cfg.OutRefundNo,
	}, err
}
