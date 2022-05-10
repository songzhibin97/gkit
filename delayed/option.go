package delayed

import (
	"github.com/songzhibin97/gkit/options"
	"os"
	"time"
)

// SetCheckTime 设置检查时间
func SetCheckTime(checkTime time.Duration) options.Option {
	return func(o interface{}) {
		o.(*DispatchingDelayed).checkTime = checkTime
	}
}

// SetWorkerNumber 设置并发数
func SetWorkerNumber(w int64) options.Option {
	return func(o interface{}) {
		o.(*DispatchingDelayed).Worker = w
	}
}

// SetSingle 设置监控信号
func SetSingle(signal ...os.Signal) options.Option {
	return func(o interface{}) {
		o.(*DispatchingDelayed).signal = signal
	}
}

// SetSingleCallback 设置监控信号回调
func SetSingleCallback(callback func(signal os.Signal, d *DispatchingDelayed)) options.Option {
	return func(o interface{}) {
		o.(*DispatchingDelayed).signalCallback = callback
	}
}
