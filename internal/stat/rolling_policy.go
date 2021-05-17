package stat

import (
	"sync"
	"time"
)

// RollingPolicy 基于持续时间的环形窗口的策略,随时间段移动存储桶偏移量。
type RollingPolicy struct {
	mu     sync.RWMutex
	size   int
	window *Window
	offset int

	bucketDuration time.Duration
	lastAppendTime time.Time
}

// timespan 时间跨度
func (r *RollingPolicy) timespan() int {
	v := int(time.Since(r.lastAppendTime) / r.bucketDuration)
	if v > -1 {
		// 时钟回滚?
		return v
	}
	return r.size
}

// add
func (r *RollingPolicy) add(f func(offset int, val float64), val float64) {
	r.mu.Lock()
	timespan := r.timespan()
	if timespan > 0 {
		r.lastAppendTime = r.lastAppendTime.Add(time.Duration(timespan * int(r.bucketDuration)))
		offset := r.offset
		// 重置过期的 bucket
		s := offset + 1
		if timespan > r.size {
			timespan = r.size
		}
		// e: reset offset must start from offset+1
		e, e1 := s+timespan, 0
		if e > r.size {
			e1 = e - r.size
			e = r.size
		}
		for i := s; i < e; i++ {
			r.window.ResetBucket(i)
			offset = i
		}
		for i := 0; i < e1; i++ {
			r.window.ResetBucket(i)
			offset = i
		}
		r.offset = offset
	}
	f(r.offset, val)
	r.mu.Unlock()
}

// Append 将给定的点附加到窗口
func (r *RollingPolicy) Append(val float64) {
	r.add(r.window.Append, val)
}

// Add 将给定值添加到存储桶中的最新点
func (r *RollingPolicy) Add(val float64) {
	r.add(r.window.Add, val)
}

// Reduce 缩减应用窗口
func (r *RollingPolicy) Reduce(f func(Iterator) float64) (val float64) {
	r.mu.RLock()
	timespan := r.timespan()
	if count := r.size - timespan; count > 0 {
		offset := r.offset + timespan + 1
		if offset >= r.size {
			offset = offset - r.size
		}
		val = f(r.window.Iterator(offset, count))
	}
	r.mu.RUnlock()
	return val
}

// NewRollingPolicy 实例化 RollingPolicy 对象
func NewRollingPolicy(window *Window, bucketDuration time.Duration) *RollingPolicy {
	return &RollingPolicy{
		window: window,
		size:   window.Size(),
		offset: 0,

		bucketDuration: bucketDuration,
		lastAppendTime: time.Now(),
	}
}
