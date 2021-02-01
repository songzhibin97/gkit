package atomicArray

import (
	"sync/atomic"
	"unsafe"
)

const (
	// PtrOffSize: 指针偏移量大小
	PtrOffSize = int64(8)
)

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
// bucketSize: 单位 ms
func (b *Bucket) isHit(now uint64, bucketSize uint64) bool {
	return b.Start <= now && now < b.Start+bucketSize
}

// calculateStartTime: 计算各个桶的开始时间
func calculateStartTime(now uint64, bucketSize uint64) uint64 {
	return now - (now % bucketSize)
}

//   B0       B1      B2     B3
//   |_______|_______|_______|
//  1000    1200    1400    1600

// AtomicArray: 原子数组
type AtomicArray struct {
	// length: array长度
	length int64

	// base: 数据基地址
	base unsafe.Pointer
	data []*Bucket
}

// offset: 偏移量
func (a *AtomicArray) offset(index int64) (unsafe.Pointer, bool) {
	if index < 0 || index >= a.length {
		return nil, false
	}
	base := a.base
	return unsafe.Pointer(uintptr(base) + uintptr(index*PtrOffSize)), true
}

// getBucket: 根据偏移量获取bucket
func (a *AtomicArray) getBucket(index int64) *Bucket {
	if offset, ok := a.offset(index); ok {
		return (*Bucket)(atomic.LoadPointer((*unsafe.Pointer)(offset)))
	}
	return nil
}

// cas: compare and swap
// 交换
func (a *AtomicArray) cas(index int64, except, update *Bucket) bool {
	if offset, ok := a.offset(index); ok {
		return atomic.CompareAndSwapPointer((*unsafe.Pointer)(offset), unsafe.Pointer(except), unsafe.Pointer(update))
	}
	return false
}
