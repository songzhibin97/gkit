package egroup

import (
	"os"
	"time"
)

// Option: 选项模式
type Option func(o *options)

// options
type options struct {
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
func StartTimeout(d time.Duration) Option {
	return func(o *options) { o.startTimeout = d }
}

// StopTimeout: 设置停止超时时间
func StopTimeout(d time.Duration) Option {
	return func(o *options) { o.stopTimeout = d }
}

// Signal: 设置信号集合,和处理信号的函数
func Signal(handler func(*LifeAdmin, os.Signal), signals ...os.Signal) Option {
	return func(o *options) {
		o.handler = handler
		o.signals = signals
	}
}
