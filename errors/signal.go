package errors

import "errors"

var (
	ErrRepeatClose = errors.New("重复关闭")
)

// IsRepeatClose: 重复关闭
func IsRepeatClose(err error) bool {
	return errors.Is(err, ErrRepeatClose)
}
