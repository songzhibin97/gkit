package stat

import (
	"fmt"
	"time"
)

// RollingCounter: 滚动窗口接口
type RollingCounter interface {
	Metric
	Aggregation
	Timespan() int
	// Reduce: 将缩减功能应用于窗口内的所有存储桶。
	Reduce(func(Iterator) float64) float64
}

// rollingCounter: 实现接口 RollingCounter
type rollingCounter struct {
	policy *RollingPolicy
}

// RollingCounterOpts: 创建 rollingCounter 所需要的参数
type RollingCounterOpts struct {
	Size           int
	BucketDuration time.Duration
}

// Add:
func (r *rollingCounter) Add(val int64) {
	if val < 0 {
		panic(fmt.Errorf("stat/metric: cannot decrease in value. val: %d", val))
	}
	r.policy.Add(float64(val))
}

// Reduce:
func (r *rollingCounter) Reduce(f func(Iterator) float64) float64 {
	return r.policy.Reduce(f)
}

// Avg:
func (r *rollingCounter) Avg() float64 {
	return r.policy.Reduce(Avg)
}

// Min:
func (r *rollingCounter) Min() float64 {
	return r.policy.Reduce(Min)
}

// Max:
func (r *rollingCounter) Max() float64 {
	return r.policy.Reduce(Max)
}

// Sum:
func (r *rollingCounter) Sum() float64 {
	return r.policy.Reduce(Sum)
}

// Value:
func (r *rollingCounter) Value() int64 {
	return int64(r.Sum())
}

// Timespan:
func (r *rollingCounter) Timespan() int {
	return r.policy.timespan()
}

// NewRollingCounter: 实例化 RollingCounter 方法
func NewRollingCounter(opts RollingCounterOpts) RollingCounter {
	window := NewWindow(WindowOpts{Size: opts.Size})
	policy := NewRollingPolicy(window, RollingPolicyOpts{BucketDuration: opts.BucketDuration})
	return &rollingCounter{
		policy: policy,
	}
}
