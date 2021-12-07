package window

import (
	"sync/atomic"
	"unsafe"

	"github.com/songzhibin97/gkit/internal/clock"
	"github.com/songzhibin97/gkit/internal/sys/safe"
)

// AtomicArray 封装原子操作, 底层维护 []*Bucket
type AtomicArray struct {
	// length: array长度
	length uint64

	// base: 数据基地址
	base unsafe.Pointer
	data []*Bucket
}

// offset 根据index获取底层的桶
func (a *AtomicArray) offset(index uint64) (unsafe.Pointer, bool) {
	if index < 0 {
		return nil, false
	}
	index = index % a.length
	base := a.base
	return unsafe.Pointer(uintptr(base) + uintptr(index*PtrOffSize)), true
}

// getBucket 根据index获取底层bucket
func (a *AtomicArray) getBucket(index uint64) *Bucket {
	if ptr, ok := a.offset(index); ok {
		return (*Bucket)(atomic.LoadPointer((*unsafe.Pointer)(ptr)))
	}
	return nil
}

// compareAndSwap 比较交换
func (a *AtomicArray) compareAndSwap(index uint64, old, new *Bucket) bool {
	if ptr, ok := a.offset(index); ok {
		return atomic.CompareAndSwapPointer((*unsafe.Pointer)(ptr), unsafe.Pointer(old), unsafe.Pointer(new))
	}
	return false
}

// NewAtomicArrayWithTime 初始化 AtomicArray, 需要手动传入 startTime 作为时间戳
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

// NewAtomicArray 初始化 AtomicArray, startTime 为当前时间
func NewAtomicArray(length uint64, bucketSize uint64, Builder BucketBuilder) *AtomicArray {
	return NewAtomicArrayWithTime(length, bucketSize, clock.GetTimeMillis(), Builder)
}
