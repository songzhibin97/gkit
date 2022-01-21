package task

import (
	"fmt"
	"time"
)

type errNonsupportType struct {
	valueType string
}

func NewErrNonsupportType(valueType string) error {
	return &errNonsupportType{valueType: valueType}
}

func (e *errNonsupportType) Error() string {
	return e.valueType + ":不是支持类型"
}

// ErrRetryTaskLater 重试错误
type ErrRetryTaskLater struct {
	msg     string
	retryIn time.Duration
}

// RetryIn 返回重试时间,从现在开始到执行的间隔
func (e ErrRetryTaskLater) RetryIn() time.Duration {
	return e.retryIn
}

// Error 实现标准error接口
func (e ErrRetryTaskLater) Error() string {
	return fmt.Sprintf("Task error: %s Will retry in: %s", e.msg, e.retryIn)
}

// NewErrRetryTaskLater 生成重试错误
func NewErrRetryTaskLater(msg string, retryIn time.Duration) ErrRetryTaskLater {
	return ErrRetryTaskLater{msg: msg, retryIn: retryIn}
}

type Retrievable interface {
	RetryIn() time.Duration
}
