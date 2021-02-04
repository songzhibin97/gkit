package egroup

import (
	"Songzhibin/GKit/options"
	"os"
	"time"
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

// StartTimeout: 设置启动超时时间
func StartTimeout(d time.Duration) options.Option {
	return func(c interface{}) { c.(*config).startTimeout = d }
}

// StopTimeout: 设置停止超时时间
func StopTimeout(d time.Duration) options.Option {
	return func(c interface{}) { c.(*config).stopTimeout = d }
}

// Signal: 设置信号集合,和处理信号的函数
func Signal(handler func(*LifeAdmin, os.Signal), signals ...os.Signal) options.Option {
	return func(c interface{}) {
		conf := c.(*config)
		conf.handler = handler
		conf.signals = signals
	}
}
