package clock

import (
	"sync/atomic"
	"time"
)

var nowInMs = uint64(0)

// StartTicker 启动一个后台任务,该任务每毫秒缓存当前时间戳,在高并发情况下可能会提供更好的性能.
func StartTicker() {
	atomic.StoreUint64(&nowInMs, uint64(time.Now().UnixNano())/UnixTimeUnitOffset)
	go func() {
		defer func() {
			if err := recover(); err != nil {
				// no possible
			}
		}()
		for {
			now := uint64(time.Now().UnixNano()) / UnixTimeUnitOffset
			atomic.StoreUint64(&nowInMs, now)
			time.Sleep(time.Millisecond)
		}
	}()
}

// GetTimestamp 获得当前时间戳,单位是ms
func GetTimestamp() uint64 {
	return atomic.LoadUint64(&nowInMs)
}
