package errors

import "errors"

var (
	ErrRepeatClose = errors.New("重复关闭")
	ErrGoExit      = errors.New("go关闭")
)

// IsRepeatClose: 重复关闭
func IsRepeatClose(err error) bool {
	return errors.Is(err, ErrRepeatClose)
}

// 判断是否为go关闭
func IsGoExit(err error) bool {
	return errors.Is(err, ErrGoExit)
}
