package config

import "slices"

type Support int

func (s Support) String() string {
	return supportNames[s]
}

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

func (a Supports) Names() []string {
	names := make([]string, len(a))
	for i, v := range a {
		names[i] = v.String()
	}
	return names
}

var supportNames = map[Support]string{
	SupportPayNotify:    `payNotify`,
	SupportPayQuery:     `payQuery`,
	SupportRefund:       `refund`,
	SupportRefundNotify: `refundNotify`,
	SupportRefundQuery:  `refundQuery`,
}
