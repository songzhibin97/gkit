package goroutine

import (
	"Songzhibin/GKit/log"
	"time"
)

// Option: 选项模式
type Option func(o *options)

// options
type options struct {

	// stopTimeout: 关闭超时时间
	// 控制shutdown关闭超时时间
	// <=0 不启动超时时间
	stopTimeout time.Duration

	// max 最大goroutine以及初始化channel大小,channel长度不可更改
	max int64

	// logger 日志输出对象
	logger log.Logger
}

// StopTimeout: 设置停止超时时间
func StopTimeout(d time.Duration) Option {
	return func(o *options) { o.stopTimeout = d }
}

// Max: 设置pool最大容量
func Max(max int64) Option {
	return func(o *options) { o.max = max }
}

// Logger: 设置pool最大容量
func Logger(logger log.Logger) Option {
	return func(o *options) { o.logger = logger }
}
