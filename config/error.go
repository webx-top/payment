package config

import "errors"

var (
	ErrUnknowDevice  = errors.New("Unknow device type")
	ErrSignature     = errors.New("Signature error")
	ErrPaymentFailed = errors.New("Payment failed")
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
