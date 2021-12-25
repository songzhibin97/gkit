package tools

import "errors"

var (
	ErrorMustPtr       = errors.New("dst or src must ptr")
	ErrorNoEquals      = errors.New("dst and src must equal type")
	ErrorMustStructPtr = errors.New("dst or src must struct ptr")
	ErrorInvalidValue  = errors.New("invalid value")
)
