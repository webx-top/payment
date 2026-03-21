package mockpay

import (
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/admpub/log"
	"github.com/admpub/resty/v2"
	"github.com/webx-top/com"
	"github.com/webx-top/echo"
	"github.com/webx-top/echo/param"
	"github.com/webx-top/payment"
	"github.com/webx-top/payment/config"
	"github.com/webx-top/restyclient"
)

const Name = `mockpay`

var supports = config.Supports{
	config.SupportPayNotify,
	config.SupportPayQuery,
	config.SupportRefund,
	config.SupportRefundNotify,
	config.SupportRefundQuery,
}

func init() {
	payment.Register(Name, echo.T(`模拟支付`), New)
}

func New() payment.Driver {
	return &Mockpay{}
}

type Mockpay struct {
	account        *config.Account
	notifyCallback func(echo.Context) error
}

func (a *Mockpay) IsSupported(s config.Support) bool {
	return supports.IsSupported(s)
}

func (a *Mockpay) SetNotifyCallback(callback func(echo.Context) error) payment.Driver {
	a.notifyCallback = callback
	return a
}

func (a *Mockpay) SetAccount(account *config.Account) payment.Driver {
	a.account = account
	return a
}

func (a *Mockpay) callbackClient() *resty.Request {
	return restyclient.Retryable()
}

func (a *Mockpay) VerifySign(ctx echo.Context) error {
	log.Infof(`[Mockpay] VerifySign Form Data: %s`, com.Dump(ctx.Forms(), false))
	return config.ErrUnsupported
}

// name: queryStatus / supportDevices / noticeDelay
func (a *Mockpay) getOptionValue(name string, cfg *config.Pay) string {
	var optionValue string
	if a.account.Options.Extra != nil {
		optionValue = a.account.Options.Extra.String(name)
	}
	if len(optionValue) == 0 && cfg != nil && cfg.Options != nil {
		optionValue = cfg.Options.String(name)
	}
	return optionValue
}

func (a *Mockpay) Pay(ctx echo.Context, cfg *config.Pay) (*config.PayResponse, error) {
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
	var err error
	result := &config.PayResponse{
		TradeNo: tradeNo,
		Params:  echo.H{},
	}
	err = a.delaySubmitPayNotice(*a.account, *cfg, tradeNo)
	return result, err
}

func (a *Mockpay) delaySubmitPayNotice(account config.Account, cfg config.Pay, tradeNo string) error {
	err := setCachedData(`pay.`+tradeNo, GatewayPayData{
		TradeNo:     tradeNo,
		TotalAmount: cfg.Amount,
		Currency:    cfg.Currency.String(),
	})
	if err != nil {
		return err
	}
	var delay time.Duration
	delay, err = a.getNoticeDelay()
	if err != nil {
		return err
	}
	go func() {
		time.Sleep(delay)
		err := a.submitPayNotice(account, cfg, tradeNo)
		if err != nil {
			log.Error(err)
		} else {
			log.Okay(`[Mockpay] succeed in submitPayNotice`)
		}
	}()
	return err
}

func (a *Mockpay) submitPayNotice(account config.Account, cfg config.Pay, tradeNo string) error {
	noticeStatus := a.getOptionValue(`noticeStatus`, nil)
	if len(noticeStatus) == 0 {
		noticeStatus = config.TradeStatusSuccess
	}
	data := url.Values{
		`status`:          []string{noticeStatus},
		`trade_no`:        []string{tradeNo},
		`out_trade_no`:    []string{cfg.OutTradeNo},
		`passback_params`: []string{cfg.PassbackParams},
		`total_amount`:    []string{payment.CutFloat(cfg.Amount, 2)},
		`reason`:          []string{``},
	}
	data.Set(`hash`, GenerateHash(data, account.AppSecret))
	response, err := a.callbackClient().Post(cfg.NotifyURL)
	if err != nil {
		return err
	}
	if response.IsError() {
		return fmt.Errorf(`[Mockpay] failed to submitPayNotice: %s`, response.String())
	}
	if response.String() != `success` {
		return fmt.Errorf(`[Mockpay] succeed in submitPayNotice, but the response to the result is not the desired "success": %s`, response.String())
	}
	return err
}

func (a *Mockpay) PayNotify(ctx echo.Context) error {
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
		ctx.Set(`notify`, result)
		if err := a.notifyCallback(ctx); err != nil {
			isSuccess = false
		}
	}
	if isSuccess {
		return ctx.String(`success`)
	}
	return ctx.String(`failed`)
}

func (a *Mockpay) PayQuery(ctx echo.Context, cfg *config.Query) (*config.Result, error) {
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

func (a *Mockpay) getNoticeDelay() (delay time.Duration, err error) {
	v := a.getOptionValue(`noticeDelay`, nil)
	if len(v) > 0 {
		delay, err = time.ParseDuration(v)
	} else {
		delay = 2 * time.Second
	}
	return
}

func (a *Mockpay) delaySubmitRefundNotice(account config.Account, cfg config.Refund, refundNo string) error {
	err := setCachedData(`refund.`+refundNo, GatewayRefundData{
		RefundNo:    refundNo,
		TotalAmount: cfg.TotalAmount,
		RefundFee:   cfg.RefundAmount,
		Currency:    cfg.Currency.String(),
	})
	if err != nil {
		return err
	}
	var delay time.Duration
	delay, err = a.getNoticeDelay()
	if err != nil {
		return err
	}
	go func() {
		time.Sleep(delay)
		err := a.submitRefundNotice(account, cfg, refundNo)
		if err != nil {
			log.Error(err)
		} else {
			log.Okay(`[Mockpay] succeed in submitRefundNotice`)
		}
	}()
	return err
}

func (a *Mockpay) submitRefundNotice(account config.Account, cfg config.Refund, refundNo string) error {
	noticeStatus := a.getOptionValue(`noticeStatus`, nil)
	if len(noticeStatus) == 0 {
		noticeStatus = config.TradeStatusSuccess
	}
	data := url.Values{
		`status`:        []string{noticeStatus},
		`trade_no`:      []string{cfg.TradeNo},
		`out_trade_no`:  []string{cfg.OutTradeNo},
		`out_refund_no`: []string{cfg.OutRefundNo},
		`refund_no`:     []string{refundNo},
		`refund_fee`:    []string{payment.CutFloat(cfg.RefundAmount, 2)},
		`currency`:      []string{cfg.Currency.String()},
		`reason`:        []string{cfg.RefundReason},
	}
	data.Set(`hash`, GenerateHash(data, account.AppSecret))
	response, err := a.callbackClient().Post(cfg.NotifyURL)
	if err != nil {
		return err
	}
	if response.IsError() {
		return fmt.Errorf(`[Mockpay] failed to submitRefundNotice: %s`, response.String())
	}
	if response.String() != `success` {
		return fmt.Errorf(`[Mockpay] succeed in submitRefundNotice, but the response to the result is not the desired "success": %s`, response.String())
	}
	return err
}

func (a *Mockpay) RefundNotify(ctx echo.Context) error {
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
		ctx.Set(`notify`, result)
		if err := a.notifyCallback(ctx); err != nil {
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
