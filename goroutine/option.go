package goroutine

import (
	"github.com/songzhibin97/gkit/log"
	"github.com/songzhibin97/gkit/options"
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

// SetStopTimeout 设置停止超时时间
func SetStopTimeout(d time.Duration) options.Option {
	return func(c interface{}) { c.(*config).stopTimeout = d }
}

// SetMax 设置pool最大容量
func SetMax(max int64) options.Option {
	return func(c interface{}) { c.(*config).max = max }
}

// SetLogger 设置日志对象
func SetLogger(logger log.Logger) options.Option {
	return func(c interface{}) { c.(*config).logger = logger }
}
