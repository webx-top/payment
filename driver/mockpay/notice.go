package mockpay

import (
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/admpub/log"
	"github.com/webx-top/payment"
	"github.com/webx-top/payment/config"
)

func (a *Mockpay) getNoticeDelay() (delay time.Duration, err error) {
	v := a.getOptionValue(`noticeDelay`, nil)
	if len(v) > 0 {
		delay, err = time.ParseDuration(v)
	} else {
		delay = 2 * time.Second
	}
	return
}

func (a *Mockpay) delaySubmitPayNotice(account config.Account, cfg config.Pay, tradeNo string, delay time.Duration) error {
	err := setCachedData(`pay.`+tradeNo, GatewayPayData{
		TradeNo:     tradeNo,
		TotalAmount: cfg.Amount,
		Currency:    cfg.Currency.String(),
		Config:      cfg,
	})
	if err != nil {
		return err
	}
	if !a.IsSupported(config.SupportPayNotify) {
		return err
	}
	if delay <= 0 {
		return a.submitPayNotice(account, cfg, tradeNo)
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

func (a *Mockpay) SubmitPayNotice(tradeNo string) error {
	if !a.IsSupported(config.SupportPayNotify) {
		return config.ErrUnsupported
	}
	tradeNo = strings.TrimPrefix(tradeNo, `pay.`)
	data, err := getCachedPayData(`pay.` + tradeNo)
	if err != nil {
		return err
	}
	return a.submitPayNotice(*a.account, data.Config, data.TradeNo)
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
	response, err := a.callbackClient().SetFormDataFromValues(data).Post(cfg.NotifyURL)
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

func (a *Mockpay) delaySubmitRefundNotice(account config.Account, cfg config.Refund, refundNo string) error {
	err := setCachedData(`refund.`+refundNo, GatewayRefundData{
		RefundNo:    refundNo,
		TotalAmount: cfg.TotalAmount,
		RefundFee:   cfg.RefundAmount,
		Currency:    cfg.Currency.String(),
		Config:      cfg,
	})
	if err != nil {
		return err
	}
	if !a.IsSupported(config.SupportRefundNotify) {
		return err
	}
	var delay time.Duration
	delay, err = a.getNoticeDelay()
	if err != nil {
		return err
	}
	if delay <= 0 {
		return a.submitRefundNotice(account, cfg, refundNo)
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

func (a *Mockpay) SubmitRefundNotice(refundNo string) error {
	if !a.IsSupported(config.SupportRefundNotify) {
		return config.ErrUnsupported
	}
	refundNo = strings.TrimPrefix(refundNo, `refund.`)
	data, err := getCachedRefundData(`refund.` + refundNo)
	if err != nil {
		return err
	}
	return a.submitRefundNotice(*a.account, data.Config, data.RefundNo)
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
	response, err := a.callbackClient().SetFormDataFromValues(data).Post(cfg.NotifyURL)
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
