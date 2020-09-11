package alipay

import (
	"fmt"
	"math/rand"
	"net/url"
	"time"

	"github.com/admpub/log"
	"github.com/smartwalle/alipay"
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
	client         *alipay.AliPay
	notifyCallback func(echo.Context) error
}

func (a *Alipay) SetNotifyCallback(callback func(echo.Context) error) payment.Hook {
	a.notifyCallback = callback
	return a
}

func (a *Alipay) SetAccount(account *config.Account) payment.Hook {
	a.account = account
	a.client = alipay.New(
		account.AppID,
		account.PublicKey,
		account.PrivateKey,
		!account.Debug,
	)
	return a
}

func (a *Alipay) Pay(cfg *config.Pay) (param.StringMap, error) {
	payConfig := alipay.TradePay{
		NotifyURL:   cfg.NotifyURL,
		Subject:     cfg.Subject,
		OutTradeNo:  cfg.TradeNo,
		TotalAmount: MoneyFeeToString(cfg.Amount),
		ProductCode: "QUICK_WAP_WAY",
	}
	var err error
	result := param.StringMap{}
	switch cfg.Device {
	case config.App:
		pay := alipay.AliPayTradeAppPay{TradePay: payConfig}
		results, err := a.client.TradeAppPay(pay)
		if err != nil {
			return result, err
		}
		result["orderString"] = param.String(results)
	case config.Web:
		pay := alipay.AliPayTradePagePay{TradePay: payConfig}
		url, err := a.client.TradePagePay(pay)
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
	return ctx.String(err.Error())
}

func (a *Alipay) Refund(cfg *config.Refund) (param.StringMap, error) {
	result := param.StringMap{}
	refundConfig := alipay.AliPayTradeRefund{
		OutTradeNo:   cfg.TradeNo,
		RefundAmount: MoneyFeeToString(cfg.RefundAmount),
		RefundReason: cfg.RefundReason,
		OutRequestNo: fmt.Sprintf("%d%d", time.Now().Local().Unix(), rand.Intn(9999)),
	}
	resp, err := a.client.TradeRefund(refundConfig)
	if err != nil {
		return nil, err
	}
	refund := resp.AliPayTradeRefund // 退款信息
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
