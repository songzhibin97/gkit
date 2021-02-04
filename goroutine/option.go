package goroutine

import (
	"Songzhibin/GKit/log"
	"Songzhibin/GKit/options"
	"time"
)

// config
type config struct {

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
func StopTimeout(d time.Duration) options.Option {
	return func(c interface{}) { c.(*config).stopTimeout = d }
}

// Max: 设置pool最大容量
func Max(max int64) options.Option {
	return func(c interface{}) { c.(*config).max = max }
}

// Logger: 设置pool最大容量
func Logger(logger log.Logger) options.Option {
	return func(c interface{}) { c.(*config).logger = logger }
}
