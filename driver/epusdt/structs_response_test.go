package epusdt

import "testing"

func TestSignOrderNotifyResponse(t *testing.T) {
	req := OrderNotifyResponse{
		TradeId:            `abc`,
		OrderId:            `def`,
		Amount:             200.12,
		ActualAmount:       200.12,
		Token:              `0xabcdefghijkmln`,
		BlockTransactionId: `uvwxyz`,
		Signature:          ``,
		Status:             2,
		Nonce:              `iajfiefioahfoeflfbjpftnaoeof`,
	}
	data := req.URLValues()
	sign := GenerateSign(data, `testsecret`)
	t.Logf(`sign: %s`, sign)
}
