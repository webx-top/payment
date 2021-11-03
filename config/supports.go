package config

type Support int

const (
	SupportPayNotify Support = iota
	SupportPayQuery
	SupportRefund
	SupportRefundNotify
	SupportRefundQuery
)

type Supports []Support

func (a Supports) IsSupported(s Support) bool {
	for _, v := range a {
		if v == s {
			return true
		}
	}
	return false
}
