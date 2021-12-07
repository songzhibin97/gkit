package egroup

import (
	"os"
	"time"

	"github.com/songzhibin97/gkit/options"
)

// config
type config struct {
	// startTimeout: 启动超时时间
	// <=0 不启动超时时间,注意要在shutdown处理关闭通知
	startTimeout time.Duration

	// stopTimeout: 关闭超时时间
	// <=0 不启动超时时间
	stopTimeout time.Duration

	// signals: 信号集
	signals []os.Signal

	// handler: 捕捉信号后处理函数
	handler func(*LifeAdmin, os.Signal)
}

// SetStartTimeout 设置启动超时时间
func SetStartTimeout(d time.Duration) options.Option {
	return func(c interface{}) { c.(*config).startTimeout = d }
}

// SetStopTimeout 设置停止超时时间
func SetStopTimeout(d time.Duration) options.Option {
	return func(c interface{}) { c.(*config).stopTimeout = d }
}

// SetSignal 设置信号集合,和处理信号的函数
func SetSignal(handler func(*LifeAdmin, os.Signal), signals ...os.Signal) options.Option {
	return func(c interface{}) {
		conf := c.(*config)
		conf.handler = handler
		conf.signals = signals
	}
}
