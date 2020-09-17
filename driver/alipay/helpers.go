package alipay

import (
	"net/url"

	alipay "github.com/smartwalle/alipay/v3"
	"github.com/webx-top/echo"
	"github.com/webx-top/echo/param"
	"github.com/webx-top/payment"
	"github.com/webx-top/payment/config"
)

func (a *Alipay) VerifySign(ctx echo.Context, req url.Values) error {
	return a.verifySign(req)
}

func (a *Alipay) verifySign(req url.Values) error {
	ok, err := a.Client().VerifySign(req)
	if err != nil {
		return err
	}
	if !ok {
		return config.ErrSignature
	}
	return nil
}

func (a *Alipay) getAlipayTradeNotification(req url.Values) (*alipay.TradeNotification, error) {
	err := a.verifySign(req)
	if err != nil {
		return nil, err
	}
	noti := &alipay.TradeNotification{}
	noti.AppId = req.Get("app_id")
	noti.AuthAppId = req.Get("auth_app_id")
	noti.NotifyId = req.Get("notify_id")
	noti.NotifyType = req.Get("notify_type")
	noti.NotifyTime = req.Get("notify_time")
	noti.TradeNo = req.Get("trade_no")
	noti.TradeStatus = alipay.TradeStatus(req.Get("trade_status"))
	noti.TotalAmount = req.Get("total_amount")
	noti.ReceiptAmount = req.Get("receipt_amount")
	noti.InvoiceAmount = req.Get("invoice_amount")
	noti.BuyerPayAmount = req.Get("buyer_pay_amount")
	noti.SellerId = req.Get("seller_id")
	noti.SellerEmail = req.Get("seller_email")
	noti.BuyerId = req.Get("buyer_id")
	noti.BuyerLogonId = req.Get("buyer_logon_id")
	noti.FundBillList = req.Get("fund_bill_list")
	noti.Charset = req.Get("charset")
	noti.PointAmount = req.Get("point_amount")
	noti.OutTradeNo = req.Get("out_trade_no")
	noti.OutBizNo = req.Get("out_biz_no")
	noti.GmtCreate = req.Get("gmt_create")
	noti.GmtPayment = req.Get("gmt_payment")
	noti.GmtRefund = req.Get("gmt_refund")
	noti.GmtClose = req.Get("gmt_close")
	noti.Subject = req.Get("subject")
	noti.Body = req.Get("body")
	noti.RefundFee = req.Get("refund_fee")
	noti.Version = req.Get("version")
	noti.SignType = req.Get("sign_type")
	noti.Sign = req.Get("sign")
	noti.PassbackParams = req.Get("passback_params")
	noti.VoucherDetailList = req.Get("voucher_detail_list")

	return noti, nil
}

func (a *Alipay) getAlipayTradeNotificationData(req url.Values) (param.StringMap, error) {
	err := a.verifySign(req)
	if err != nil {
		return nil, err
	}

	result := param.StringMap{}
	for k := range req {
		result[k] = param.String(req.Get(k))
	}
	return result, nil
}

// MoneyFeeToString 支付宝金额转字符串
func MoneyFeeToString(moneyFee float64) string {
	return payment.CutFloat(moneyFee, 2)
}
