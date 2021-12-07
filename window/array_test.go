package window

import (
	"sync/atomic"
	"testing"
	"unsafe"

	"github.com/songzhibin97/gkit/internal/clock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

const (
	// BucketSize 桶大小
	BucketSize uint64 = 500
	// N: 长度
	N uint64 = 20
	// IntervalSize 时间间隔 10s
	IntervalSize uint64 = 10 * 1000
)

// mock ArrayMock and implement BucketGenerator
type Mock struct {
	mock.Mock
}

func (bla *Mock) NewEmptyBucket() interface{} {
	return new(int64)
}

func (bla *Mock) Reset(b *Bucket, startTime uint64) *Bucket {
	b.Start = startTime
	b.Value.Store(new(int64))
	return b
}

func TestBucketSize(t *testing.T) {
	b := &Bucket{
		Start: clock.GetTimeMillis(),
		Value: atomic.Value{},
	}
	if !assert.Equal(t, int(unsafe.Sizeof(*b)), 24) {
		t.Errorf("the size of Bucket is not equal 24.\n")
	}
	if !assert.Equal(t, int(unsafe.Sizeof(b)), 8) {
		t.Errorf("the size of Bucket pointer is not equal 8.\n")
	}
}

func TestGetBucket(t *testing.T) {
	n := (uint64)(5)
	array := NewAtomicArray(n, BucketSize, &Mock{})
	for i := (uint64)(0); i < n; i++ {
		b := array.getBucket(i)
		if !assert.Equal(t, b, array.data[i]) {
			t.Error("getBucket not equal")
		}
	}
}

func TestCompareAndSwap(t *testing.T) {
	n := (uint64)(5)
	array := NewAtomicArray(n, BucketSize, &Mock{})
	for i := (uint64)(0); i < n; i++ {
		if !array.compareAndSwap(i, array.getBucket(i), array.getBucket(i)) {
			t.Error("compareAndSwap not equal")
		}
	}
}
