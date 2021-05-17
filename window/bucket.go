package window

import (
	"sync/atomic"
)

const (
	// PtrOffSize 指针偏移量大小
	PtrOffSize = uint64(8)
)

// BucketBuilder Bucket 生成器
type BucketBuilder interface {
	// NewEmptyBucket 生成一个空桶
	NewEmptyBucket() interface{}

	// Reset 重置桶
	Reset(b *Bucket, startTime uint64) *Bucket
}

// Bucket 滑动窗口的承载的最小元素
type Bucket struct {
	// Start 存储了这个桶的起始时间
	Start uint64

	// Value 实际挂载对象
	Value atomic.Value
}

// reset 重置 Bucket.Start 属性
func (b *Bucket) reset(start uint64) {
	b.Start = start
}

// isHit 判断 now 是否命中了该桶
func (b *Bucket) isHit(now uint64, bucketSize uint64) bool {
	return b.Start <= now && now < b.Start+bucketSize
}

// calculateStartTime 计算初始桶的时间
func calculateStartTime(now uint64, bucketSize uint64) uint64 {
	return now - (now % bucketSize)
}
