package config

import "errors"

var (
	ErrTradeAlreadyExists  = errors.New("payment trade already exists")
	ErrUnknownDevice       = errors.New("unknown device type")
	ErrSignature           = errors.New("signature error")
	ErrPaymentFailed       = errors.New("payment failed")
	ErrRefundAlreadyExists = errors.New("refund request already exists")
	ErrRefundFailed        = errors.New("refund failed")
	ErrUnsupported         = errors.New("unsupported")

	ErrAppIDRequired     = errors.New(`AppID required`)
	ErrAppSecretRequired = errors.New(`AppSecret required`)
	ErrSubtypeRequired   = errors.New(`Subtype required`)
)

func IsOK(err error) bool {
	_, ok := err.(OKer)
	return ok
}

func NewOK(msg error) *OK {
	return &OK{error: msg}
}

func NewOKString(msg string) *OK {
	return NewOK(errors.New(msg))
}

type OKer interface {
	OK()
}

type OK struct {
	error
}

func (s *OK) OK() {
}
