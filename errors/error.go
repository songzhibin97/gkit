package errors

import (
	"errors"
	"fmt"

	"github.com/golang/protobuf/proto"
	httputil "github.com/songzhibin97/gkit/errors/internal"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc/status"
)

//go:generate protoc -I. --go_out=paths=source_relative:. errors.proto

const (
	// UnknownCode is unknown code for error info.
	UnknownCode = 500
	// UnknownReason is unknown reason for error info.
	UnknownReason = ""
)

var ErrDetails = errors.New("no error details for status with code OK")

// Error 实现Error接口
func (x *Error) Error() string {
	return fmt.Sprintf("error: code = %d reason = %s message = %s details = %v", x.Code, x.Reason, x.Message, x.Metadata)
}

// StatusCode HTTP code
func (x *Error) StatusCode() int {
	return int(x.Code)
}

// GRPCStatus 返回grpc status
func (x *Error) GRPCStatus() *status.Status {
	s, _ := status.New(httputil.GRPCCodeFromStatus(x.StatusCode()), x.Message).
		WithDetails(&errdetails.ErrorInfo{
			Reason:   x.Reason,
			Metadata: x.Metadata,
		})
	return s
}

// Is 跟 target Error 比较 判断是否相等
func (x *Error) Is(target error) bool {
	if err, ok := target.(*Error); !ok {
		return false
	} else {
		return x.Code == err.Code
	}
}

// AddMetadata 增加metadata
func (x *Error) AddMetadata(mp map[string]string) *Error {
	err := proto.Clone(x).(*Error)
	err.Metadata = mp
	return err
}

// New 实例化 Error 对象
func New(code int, reason, message string) *Error {
	return &Error{
		Code:    int32(code),
		Reason:  reason,
		Message: message,
	}
}

func Errorf(code int, reason, format string, a ...interface{}) *Error {
	return New(code, reason, fmt.Sprintf(format, a...))
}

func FromError(err error) *Error {
	if err == nil {
		return nil
	}
	if se := new(Error); errors.As(err, &se) {
		return se
	}
	gs, ok := status.FromError(err)
	if ok {
		for _, detail := range gs.Details() {
			switch d := detail.(type) {
			case *errdetails.ErrorInfo:
				return New(
					httputil.StatusFromGRPCCode(gs.Code()),
					d.Reason,
					gs.Message(),
				).AddMetadata(d.Metadata)
			}
		}
	}
	return New(UnknownCode, UnknownReason, err.Error())
}

// Code 返回err指定的错误码
func Code(err error) int {
	if err == nil {
		return 0
	}
	if se := FromError(err); err != nil {
		return int(se.Code)
	}
	return UnknownCode
}

// Reason 返回err的reason
func Reason(err error) string {
	if se := FromError(err); err != nil {
		return se.Reason
	}
	return UnknownReason
}

func Is(err, target error) bool { return errors.Is(err, target) }

func As(err error, target interface{}) bool { return errors.As(err, target) }

func Unwrap(err error) error { return errors.Unwrap(err) }
