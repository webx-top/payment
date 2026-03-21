package config

import "slices"

type Support int

const (
	SupportPayNotify Support = iota + 1
	SupportPayQuery
	SupportRefund
	SupportRefundNotify
	SupportRefundQuery
)

type Supports []Support

func (a Supports) IsSupported(s Support) bool {
	return slices.Contains(a, s)
}
