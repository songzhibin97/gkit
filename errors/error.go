package errors

import (
	"errors"
	"fmt"
	"github.com/golang/protobuf/proto"
	"github.com/golang/protobuf/ptypes"
	"google.golang.org/grpc/codes"
)

var _ error = (*ErrorCode)(nil)

var ErrDetails = errors.New("no error details for status with code OK")

// ErrorCode: 错误码
type ErrorCode Status

// Error: 实现Error接口
func (e *ErrorCode) Error() string {
	return fmt.Sprintf("error: code = %d message = %s details = %+v", e.Code, e.Message, e.Details)
}

// Is: 跟 target Error 比较 判断是否相等
func (e *ErrorCode) Is(target error) bool {
	if err, ok := target.(*ErrorCode); !ok {
		return false
	} else {
		return e.Code == err.Code
	}
}

// WithDetails: 增加Details
func (e *ErrorCode) AddDetails(details ...proto.Message) error {
	for _, detail := range details {
		any, err := ptypes.MarshalAny(detail)
		if err != nil {
			return err
		}
		e.Details = append(e.Details, any)
	}
	return nil
}

// ErrToCode: error 中获取 Code 编码
func ErrToCode(err error) int32 {
	if err == nil {
		// code 0 == ok
		return 0
	}
	if eCode := new(ErrorCode); errors.As(err, &eCode) {
		return eCode.Code
	}
	// code 2 == unknown
	return 2
}

// ErrToMessage: error 中 获取 message 数据
func ErrToMessage(err error) string {
	if err == nil {
		// code 0 == ok
		return codes.OK.String()
	}
	if eCode := new(ErrorCode); errors.As(err, &eCode) {
		return eCode.Message
	}
	// code 2 == unknown
	return codes.Unknown.String()
}

// IsError: 传入code 和 err 判断是否数据该 error 是否属于该 code
func IsError(code int32, err error) bool {
	if eCode := new(ErrorCode); errors.As(err, &eCode) {
		return eCode.Code == code
	}
	return false
}

// Error: 实例化 ErrorCode 对象
func Error(code int32, message string) error {
	return &ErrorCode{
		Code:    code,
		Message: message,
	}
}

// Errorf:
func Errorf(code int32, format string, a ...interface{}) error {
	return Error(code, fmt.Sprintf(format, a...))
}
