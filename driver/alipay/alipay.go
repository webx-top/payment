package alipay

import (
	"errors"
	"fmt"
	"math/rand"
	"net/url"
	"time"

	"github.com/admpub/log"
	alipay "github.com/smartwalle/alipay/v3"
	"github.com/webx-top/echo"
	"github.com/webx-top/echo/param"
	"github.com/webx-top/payment"
	"github.com/webx-top/payment/config"
)

const Name = `alipay`

func init() {
	payment.Register(Name, `支付宝`, New)
}

func New() payment.Hook {
	return &Alipay{}
}

type Alipay struct {
	account        *config.Account
	client         *alipay.Client
	notifyCallback func(echo.Context) error
}

func (a *Alipay) SetNotifyCallback(callback func(echo.Context) error) payment.Hook {
	a.notifyCallback = callback
	return a
}

func (a *Alipay) SetAccount(account *config.Account) payment.Hook {
	a.account = account
	return a
}

func (a *Alipay) Client() *alipay.Client {
	if a.client != nil {
		return a.client
	}
	var err error
	a.client, err = alipay.New(
		a.account.AppID,
		a.account.PrivateKey,
		!a.account.Debug,
		alipay.WithTimeLocation(time.Local),
	)
	if err != nil {
		panic(err)
	}
	if len(a.account.PublicKey) > 0 {
		if err := a.client.LoadAliPayPublicKey(a.account.PublicKey); err != nil {
			log.Error(err)
		}
	}
	if len(a.account.CertPath) > 0 {
		if err := a.client.LoadAliPayPublicCertFromFile(a.account.CertPath); err != nil {
			log.Error(err)
		}
	}
	return a.client
}

func (a *Alipay) Pay(ctx echo.Context, cfg *config.Pay) (*config.PayResponse, error) {
	payConfig := alipay.Trade{
		NotifyURL:      cfg.NotifyURL,
		ReturnURL:      cfg.ReturnURL,
		Subject:        cfg.Subject,
		OutTradeNo:     cfg.OutTradeNo,
		TotalAmount:    MoneyFeeToString(cfg.Amount),
		ProductCode:    "FAST_INSTANT_TRADE_PAY",
		GoodsType:      cfg.GoodsType.String(),
		PassbackParams: cfg.PassbackParams,
	}
	if !cfg.ExpiredAt.IsZero() {
		payConfig.TimeExpire = cfg.ExpiredAt.Format(`2006-01-02 15:04:05`)
	}
	var err error
	result := &config.PayResponse{}
	switch cfg.Device {
	case config.App:
		payConfig.ProductCode = `QUICK_MSECURITY_PAY`
		pay := alipay.TradeAppPay{Trade: payConfig}
		results, err := a.Client().TradeAppPay(pay)
		if err != nil {
			return result, err
		}
		result.Raw = results
	case config.Web:
		pay := alipay.TradePagePay{Trade: payConfig}
		url, err := a.Client().TradePagePay(pay)
		if err != nil {
			return result, err
		}
		result.RedirectURL = url.String()
	case config.Wap:
		payConfig.ProductCode = `QUICK_WAP_WAY`
		pay := alipay.TradeWapPay{
			Trade:   payConfig,
			QuitURL: cfg.CancelURL,
		}
		url, err := a.Client().TradeWapPay(pay)
		if err != nil {
			return result, err
		}
		result.RedirectURL = url.String()
	default:
		return nil, config.ErrUnknowDevice
	}
	return result, err
}

func (a *Alipay) PayNotify(ctx echo.Context) error {
	formData := url.Values(ctx.Forms())
	notify, err := a.getAlipayTradeNotificationData(formData)
	if err != nil {
		return err
	}
	var isSuccess = true
	if a.notifyCallback != nil {
		status := notify.String(`trade_status`)
		result := &config.Result{
			Operation:      config.OperationPayment,
			Status:         status,
			TradeNo:        notify.String(`trade_no`),
			OutTradeNo:     notify.String(`out_trade_no`),
			PassbackParams: notify.String(`passback_params`),
			Currency:       ``,
			TotalAmount:    param.AsFloat64(notify.Float64(`total_amount`)),
			Reason:         notify.String(`reason`),
			Raw:            notify,
		}
		refundFee := notify.Float64(`refund_fee`)
		if refundFee > 0 {
			result.Operation = config.OperationRefund
			// https://opensupport.alipay.com/support/helpcenter/193/201602484855?ant_source=zsearch#
			result.OutRefundNo = notify.String(`out_biz_no`)
			result.RefundFee = refundFee
		}
		ctx.Set(`notify`, result)
		if err := a.notifyCallback(ctx); err != nil {
			isSuccess = false
		}
	}
	//alipay.AckNotification(rep) // 确认收到通知消息
	if isSuccess {
		return ctx.String(`success`)
	}
	return ctx.String(`faild`)
}

func (a *Alipay) PayQuery(ctx echo.Context, cfg *config.Query) (*config.Result, error) {
	pay := alipay.TradeQuery{
		QueryOptions: []string{"TRADE_SETTLE_INFO"},
	}
	if len(cfg.TradeNo) > 0 {
		pay.TradeNo = cfg.TradeNo
	} else {
		pay.OutTradeNo = cfg.OutTradeNo
	}
	resp, err := a.Client().TradeQuery(pay)
	if err != nil {
		return nil, err
	}
	if !resp.IsSuccess() {
		if len(resp.Content.SubMsg) > 0 {
			resp.Content.Msg += `: ` + resp.Content.SubMsg
		}
		return nil, errors.New(resp.Content.Msg)
	}
	return &config.Result{
		Operation:   config.OperationPayment,
		Status:      config.TradeStatusSuccess,
		TradeNo:     resp.Content.TradeNo,
		OutTradeNo:  resp.Content.OutTradeNo,
		Currency:    resp.Content.PayCurrency,
		TotalAmount: param.AsFloat64(resp.Content.PayAmount),
		Reason:      resp.Content.SubMsg,
		Raw:         resp,
	}, err
}

func (a *Alipay) Refund(ctx echo.Context, cfg *config.Refund) (*config.Result, error) {
	refundConfig := alipay.TradeRefund{
		OutTradeNo:   cfg.OutTradeNo,
		TradeNo:      cfg.TradeNo,
		RefundAmount: MoneyFeeToString(cfg.RefundAmount),
		RefundReason: cfg.RefundReason,
		OutRequestNo: cfg.OutRefundNo,
		OperatorId:   ``, // 可选 商户的操作员编号
		StoreId:      ``, // 可选 商户的门店编号
		TerminalId:   ``, // 可选 商户的终端编号
	}
	if len(refundConfig.OutRequestNo) == 0 {
		refundConfig.OutRequestNo = fmt.Sprintf("%d%d", time.Now().Local().Unix(), rand.Intn(9999))
	}
	resp, err := a.Client().TradeRefund(refundConfig)
	if err != nil {
		return nil, err
	}
	if !resp.IsSuccess() {
		if len(resp.Content.SubMsg) > 0 {
			resp.Content.Msg += `: ` + resp.Content.SubMsg
		}
		return nil, errors.New(resp.Content.Msg)
	}
	return &config.Result{
		Operation:   config.OperationRefund,
		Status:      config.TradeStatusSuccess,
		TradeNo:     resp.Content.TradeNo,
		OutTradeNo:  resp.Content.OutTradeNo,
		Currency:    ``,
		TotalAmount: 0,
		Reason:      resp.Content.SubMsg,
		RefundFee:   param.AsFloat64(resp.Content.RefundFee),
		OutRefundNo: cfg.OutRefundNo,
		Raw:         resp,
	}, err
}

func (a *Alipay) RefundNotify(ctx echo.Context) error {
	formData := url.Values(ctx.Forms())
	notify, err := a.getAlipayTradeNotificationData(formData)
	if err != nil {
		return err
	}

	var isSuccess = true
	if a.notifyCallback != nil {
		status := notify.String(`trade_status`)
		result := &config.Result{
			Operation:   config.OperationRefund,
			Status:      status,
			TradeNo:     notify.String(`trade_no`),
			OutTradeNo:  notify.String(`out_trade_no`),
			Currency:    ``,
			TotalAmount: param.AsFloat64(notify.Float64(`total_amount`)),
			Reason:      notify.String(`reason`),
			RefundFee:   notify.Float64(`refund_fee`),
			OutRefundNo: notify.String(`out_biz_no`),
			Raw:         notify,
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
	return ctx.String(`faild`)
}

func (a *Alipay) RefundQuery(ctx echo.Context, cfg *config.Query) (*config.Result, error) {
	pay := alipay.TradeFastPayRefundQuery{
		OutRequestNo: cfg.OutRefundNo,
		//QueryOptions: []string{"refund_detail_item_list"},
	}
	if len(cfg.TradeNo) > 0 {
		pay.TradeNo = cfg.TradeNo
	} else {
		pay.OutTradeNo = cfg.OutTradeNo
	}
	resp, err := a.Client().TradeFastPayRefundQuery(pay)
	if err != nil {
		return nil, err
	}
	if !resp.IsSuccess() {
		if len(resp.Content.SubMsg) > 0 {
			resp.Content.Msg += `: ` + resp.Content.SubMsg
		}
		return nil, errors.New(resp.Content.Msg)
	}
	return &config.Result{
		Operation:   config.OperationRefund,
		Status:      config.TradeStatusSuccess,
		TradeNo:     resp.Content.TradeNo,
		OutTradeNo:  resp.Content.OutTradeNo,
		Currency:    ``,
		TotalAmount: param.AsFloat64(resp.Content.TotalAmount),
		Reason:      resp.Content.SubMsg,
		RefundFee:   param.AsFloat64(resp.Content.RefundAmount),
		OutRefundNo: resp.Content.OutRequestNo,
		Raw:         resp,
	}, err
}
