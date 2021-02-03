package array

import (
	"Songzhibin/GKit/internal/clock"
	"Songzhibin/GKit/internal/sys/safe"
	"sync/atomic"
	"unsafe"
)

const (
	// PtrOffSize: 指针偏移量大小
	PtrOffSize = uint64(8)
)

// BucketBuilder: Bucket 生成器
type BucketBuilder interface {
	// NewEmptyBucket: 生成一个空桶
	NewEmptyBucket() interface{}

	// Reset: 重置桶
	Reset(bw *Bucket, startTime uint64) *Bucket
}

// Bucket:
type Bucket struct {
	// Start: 存储开始时间的时间戳
	Start uint64

	// Value: 实际挂载对象
	Value atomic.Value
}

// reset: 重置开始时间戳
func (b *Bucket) reset(start uint64) {
	b.Start = start
}

// isHit: 判断是否命中该桶
// bucketSize: 桶的长度 单位 ms
func (b *Bucket) isHit(now uint64, bucketSize uint64) bool {
	return b.Start <= now && now < b.Start+bucketSize
}

// calculateStartTime: 计算初始桶的时间
func calculateStartTime(now uint64, bucketSize uint64) uint64 {
	return now - (now % bucketSize)
}

//   B0       B1      B2     B3
//   |_______|_______|_______|
//  1000    1200    1400    1600

// AtomicArray: 原子数组
type AtomicArray struct {
	// length: array长度
	length uint64

	// base: 数据基地址
	base unsafe.Pointer
	data []*Bucket
}

// offset: 偏移量
func (a *AtomicArray) offset(index uint64) (unsafe.Pointer, bool) {
	if index < 0 || index >= a.length {
		return nil, false
	}
	base := a.base
	return unsafe.Pointer(uintptr(base) + uintptr(index*PtrOffSize)), true
}

// getBucket: 根据偏移量获取bucket
func (a *AtomicArray) getBucket(index uint64) *Bucket {
	if offset, ok := a.offset(index); ok {
		return (*Bucket)(atomic.LoadPointer((*unsafe.Pointer)(offset)))
	}
	return nil
}

// cas: compare and swap
// 交换
func (a *AtomicArray) cas(index uint64, except, update *Bucket) bool {
	if offset, ok := a.offset(index); ok {
		return atomic.CompareAndSwapPointer((*unsafe.Pointer)(offset), unsafe.Pointer(except), unsafe.Pointer(update))
	}
	return false
}

// NewAtomicArrayWithTime: 初始化 AtomicArray, 需要手动传入 startTime 作为时间戳
func NewAtomicArrayWithTime(length uint64, bucketSize uint64, now uint64, Builder BucketBuilder) *AtomicArray {
	array := &AtomicArray{
		length: length,
		data:   make([]*Bucket, length),
	}
	id := now / bucketSize
	index := id % length
	startTime := calculateStartTime(now, bucketSize)
	for i := index; i < length; i++ {
		b := &Bucket{
			Start: startTime,
			Value: atomic.Value{},
		}
		b.Value.Store(Builder.NewEmptyBucket())
		array.data[i] = b
		startTime += bucketSize
	}
	for i := (uint64)(0); i < index; i++ {
		b := &Bucket{
			Start: startTime,
			Value: atomic.Value{},
		}
		b.Value.Store(Builder.NewEmptyBucket())
		array.data[i] = b
		startTime += bucketSize
	}
	header := (*safe.SliceModel)(unsafe.Pointer(&array.data))
	array.base = header.Data
	return array
}

// NewAtomicArray: 初始化 AtomicArray, startTime 为当前时间
func NewAtomicArray(length uint64, bucketSize uint64, Builder BucketBuilder) *AtomicArray {
	return NewAtomicArrayWithTime(length, bucketSize, clock.GetTimeMillis(), Builder)
}
