package config

import (
	"errors"
	"fmt"
)

var (
	errNoCertWhenInsecure = errors.New("cannot specify tls certificate details when running insecurely")
	errCertRequired       = errors.New("certificate file path is required when running securely")
	errKeyRequired        = errors.New("certificate key file path is required when running securely")
	errClientCARequired   = errors.New("client certificate key file path is required when running mTLS")
)

type certMissingError struct {
	subject string
	target  string
}

func (e *certMissingError) Error() string {
	return fmt.Sprintf("%s %s does not exist", e.subject, e.target)
}

func newCertMissingError(subject, target string) *certMissingError {
	return &certMissingError{
		subject: subject,
		target:  target,
	}
}
