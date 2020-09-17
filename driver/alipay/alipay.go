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

func (a *Alipay) Pay(cfg *config.Pay) (param.StringMap, error) {
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
	var err error
	result := param.StringMap{}
	switch cfg.Device {
	case config.App:
		payConfig.ProductCode = `QUICK_MSECURITY_PAY`
		pay := alipay.TradeAppPay{Trade: payConfig}
		results, err := a.Client().TradeAppPay(pay)
		if err != nil {
			return result, err
		}
		result["orderString"] = param.String(results)
	case config.Web:
		pay := alipay.TradePagePay{Trade: payConfig}
		url, err := a.Client().TradePagePay(pay)
		if err != nil {
			return result, err
		}
		result["orderString"] = param.String(url.String())
		result["url"] = result["orderString"]
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
		result["orderString"] = param.String(url.String())
		result["url"] = result["orderString"]
	default:
		return nil, config.ErrUnknowDevice
	}
	return result, err
}

func (a *Alipay) Notify(ctx echo.Context) error {
	formData := url.Values(ctx.Forms())
	notify, err := a.getAlipayTradeNotificationData(formData)
	if err != nil {
		log.Error(err)
		return err
	}

	notify[`operation`] = `payment`
	if notify.Float64(`refund_fee`) > 0 {
		notify[`operation`] = `refund`
	}
	var isSuccess = true
	if a.notifyCallback != nil {
		ctx.Set(`notify`, notify)
		if err := a.notifyCallback(ctx); err != nil {
			log.Error(err)
			isSuccess = false
		}
	}
	if isSuccess {
		err = config.NewOKString(`success`)
	} else {
		err = config.NewOKString(`faild`)
	}
	//alipay.AckNotification(rep) // 确认收到通知消息
	return ctx.String(err.Error())
}

func (a *Alipay) Query(ctx echo.Context, cfg *config.Query) (config.TradeStatus, error) {
	pay := alipay.TradeQuery{
		OutTradeNo:   cfg.OutTradeNo,
		TradeNo:      cfg.TradeNo,
		QueryOptions: []string{"TRADE_SETTLE_INFO"},
	}
	resp, err := a.Client().TradeQuery(pay)
	if err != nil {
		return config.EmptyTradeStatus, err
	}
	if !resp.IsSuccess() {
		if len(resp.Content.SubMsg) > 0 {
			resp.Content.Msg += `: ` + resp.Content.SubMsg
		}
		return config.EmptyTradeStatus, errors.New(resp.Content.Msg)
	}
	return config.NewTradeStatus(string(resp.Content.TradeStatus)), err
}

func (a *Alipay) Refund(cfg *config.Refund) (param.StringMap, error) {
	result := param.StringMap{}
	refundConfig := alipay.TradeRefund{
		OutTradeNo:   cfg.TradeNo,
		RefundAmount: MoneyFeeToString(cfg.RefundAmount),
		RefundReason: cfg.RefundReason,
		OutRequestNo: fmt.Sprintf("%d%d", time.Now().Local().Unix(), rand.Intn(9999)),
		OperatorId:   ``, // 可选 商户的操作员编号
		StoreId:      ``, // 可选 商户的门店编号
		TerminalId:   ``, // 可选 商户的终端编号
	}
	resp, err := a.Client().TradeRefund(refundConfig)
	if err != nil {
		return nil, err
	}
	refund := resp.Content // 退款信息
	result[`code`] = param.String(refund.Code)
	if resp.IsSuccess() {
		result[`success`] = `1`
	} else {
		result[`success`] = `0`
	}
	result[`msg`] = param.String(refund.Msg)
	result[`sub_code`] = param.String(refund.SubCode)
	result[`sub_msg`] = param.String(refund.SubMsg)
	result[`trade_no`] = param.String(refund.TradeNo)
	result[`out_trade_no`] = param.String(refund.OutTradeNo)
	result[`buyer_logon_id`] = param.String(refund.BuyerLogonId)
	result[`buyer_user_id`] = param.String(refund.BuyerUserId)
	result[`fund_change`] = param.String(refund.FundChange)      // 本次退款是否发生了资金变化
	result[`refund_fee`] = param.String(refund.RefundFee)        // 退款总金额
	result[`gmt_refund_pay`] = param.String(refund.GmtRefundPay) // 退款支付时间
	result[`store_name`] = param.String(refund.StoreName)        // 交易在支付时候的门店名称
	//result[`refund_detail_item_list`]= param.String(refund.RefundDetailItemList)
	result[`sign`] = param.String(resp.Sign)

	return result, err
}
