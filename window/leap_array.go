package window

import (
	"errors"
	"runtime"
	"sync/atomic"

	"github.com/songzhibin97/gkit/internal/clock"
	"github.com/songzhibin97/gkit/internal/sys/mutex"
)

var (
	ErrBucketBuilderIsNil    = errors.New("invalid parameters, Builder is nil")
	ErrWindowNotSegmentation = errors.New("invalid parameters,window is not segmentation")
	ErrTimeBehindStart       = errors.New("time already behind bucketStart")
)

// TimeChick 自定义校验 时间戳 函数
type TimeChick func(uint64) bool

// LeapArray 基于 Bucket 实现的 leap array
// 例如: bucketSize == 200ms,intervalSize == 1000ms,所以n = 5
// 假设当前是 时间是888ms 构建下图
//   B0       B1      B2     B3      B4
//   |_______|_______|_______|_______|_______|
//  1000    1200    1400    1600    800    (1000)
//                                        ^
//                                      time=888
type LeapArray struct {
	// lock: 互斥自旋锁
	mu mutex.Mutex

	// bucketSize: 桶的长度
	bucketSize uint64

	// intervalSize: array间隔大小
	intervalSize uint64

	// n: 代表桶的个数
	n uint64

	// array: 底层数组
	array *AtomicArray
}

// getTimeIndex 获取当前时间 命中的index索引
func (s *LeapArray) getTimeIndex(now uint64) uint64 {
	id := now / s.bucketSize
	return id % s.array.length
}

// getBucketOfTime 根据时间戳获取到对应的桶
func (s *LeapArray) getBucketOfTime(now uint64, builder BucketBuilder) (*Bucket, error) {
	index := s.getTimeIndex(now)
	start := calculateStartTime(now, s.bucketSize)

	for {
		// 获取当前的 bucket
		old := s.array.getBucket(index)
		if old == nil {
			// 懒加载
			b := &Bucket{
				Start: start,
				Value: atomic.Value{},
			}
			b.Value.Store(builder.NewEmptyBucket())
			if s.array.compareAndSwap(index, nil, b) {
				return b, nil
			}
			runtime.Gosched()
		} else if start == atomic.LoadUint64(&old.Start) {
			// 在时间范围内
			return old, nil
		} else if start > atomic.LoadUint64(&old.Start) {
			// 命中下一个周期

			// 尝试获取锁
			if s.mu.TryLock() {
				// 重置
				old = builder.Reset(old, start)
				s.mu.Unlock()
				return old, nil
			}
			runtime.Gosched()
		} else if start < atomic.LoadUint64(&old.Start) {
			if s.n == 1 {
				// 如果在跳转数组中 n == 1，则在并发情况下，这种情况是可能的
				return old, nil
			}
			return nil, ErrTimeBehindStart
		}
	}
}

// GetBucket 获取桶,封装 getBucketOfTime
func (s *LeapArray) GetBucket(builder BucketBuilder) (*Bucket, error) {
	return s.getBucketOfTime(clock.GetTimeMillis(), builder)
}

// isDisable 判断当前桶是否被弃用
func (s *LeapArray) isDisable(now uint64, b *Bucket) bool {
	ws := atomic.LoadUint64(&b.Start)
	return (now - ws) > s.intervalSize
}

// getValueOfTime 通过 now 为基点 获取array所有桶
func (s *LeapArray) getValueOfTime(now uint64) []*Bucket {
	ret := make([]*Bucket, 0, s.array.length)
	for i := (uint64)(0); i < s.array.length; i++ {
		b := s.array.getBucket(i)
		if b == nil || s.isDisable(now, b) {
			continue
		}
		ret = append(ret, b)
	}
	return ret
}

// Values getValueOfTime 封装
func (s *LeapArray) Values() []*Bucket {
	return s.getValueOfTime(clock.GetTimeMillis())
}

// ValuesChick 加入自定义 TimeChick 过滤value
func (s *LeapArray) ValuesChick(now uint64, chick TimeChick) []*Bucket {
	ret := make([]*Bucket, 0, s.array.length)
	for i := (uint64)(0); i < s.array.length; i++ {
		b := s.array.getBucket(i)
		if b == nil || s.isDisable(now, b) || !chick(atomic.LoadUint64(&b.Start)) {
			continue
		}
		ret = append(ret, b)
	}
	return ret
}

// NewLeapArray 初始化 leap array
func NewLeapArray(n uint64, intervalSize uint64, builder BucketBuilder) (*LeapArray, error) {
	if builder == nil {
		return nil, ErrBucketBuilderIsNil
	}
	if intervalSize%n != 0 {
		return nil, ErrWindowNotSegmentation
	}
	bucketSize := intervalSize / n
	return &LeapArray{
		bucketSize:   bucketSize,
		intervalSize: intervalSize,
		n:            n,
		array:        NewAtomicArray(n, bucketSize, builder),
	}, nil
}
