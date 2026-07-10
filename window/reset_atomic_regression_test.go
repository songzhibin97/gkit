package window

import (
	"sync/atomic"
	"testing"
	"time"
	"unsafe"
)

type stagedResetValue struct {
	name string
}

type stagedResetBuilder struct {
	resetStarted chan struct{}
	finishReset  chan struct{}
}

func (b *stagedResetBuilder) NewEmptyBucket() interface{} {
	return &stagedResetValue{name: "old"}
}

func (b *stagedResetBuilder) Reset(bucket *Bucket, startTime uint64) *Bucket {
	atomic.StoreUint64(&bucket.Start, startTime)
	close(b.resetStarted)
	<-b.finishReset
	bucket.Value.Store(&stagedResetValue{name: "new"})
	return bucket
}

func TestLeapArrayPublishesResetBucketAtomically(t *testing.T) {
	const (
		oldTime    = uint64(100)
		bucketSize = uint64(10)
	)
	builder := &stagedResetBuilder{
		resetStarted: make(chan struct{}),
		finishReset:  make(chan struct{}),
	}
	array := NewAtomicArrayWithTime(1, bucketSize, oldTime, builder)
	leap := &LeapArray{
		bucketSize:   bucketSize,
		intervalSize: bucketSize,
		n:            1,
		array:        array,
	}
	oldBucket := array.getBucket(0)

	type result struct {
		bucket *Bucket
		err    error
	}
	resultCh := make(chan result, 1)
	go func() {
		bucket, err := leap.getBucketOfTime(oldTime+bucketSize+1, builder)
		resultCh <- result{bucket: bucket, err: err}
	}()

	select {
	case <-builder.resetStarted:
	case <-time.After(time.Second):
		t.Fatal("Reset did not start")
	}

	publishedDuringReset := array.getBucket(0)
	startDuringReset := atomic.LoadUint64(&publishedDuringReset.Start)
	valueDuringReset := publishedDuringReset.Value.Load().(*stagedResetValue).name
	close(builder.finishReset)

	var got result
	select {
	case got = <-resultCh:
	case <-time.After(time.Second):
		t.Fatal("getBucketOfTime did not return")
	}
	if got.err != nil {
		t.Fatalf("getBucketOfTime() error = %v", got.err)
	}
	if publishedDuringReset != oldBucket {
		t.Fatal("partially reset candidate was published before Reset completed")
	}
	if startDuringReset != oldTime || valueDuringReset != "old" {
		t.Fatalf("published bucket changed during Reset: start=%d value=%q", startDuringReset, valueDuringReset)
	}

	publishedAfterReset := array.getBucket(0)
	if publishedAfterReset == oldBucket {
		t.Fatal("reset reused the previously published bucket")
	}
	if got.bucket != publishedAfterReset {
		t.Fatal("getBucketOfTime returned a bucket that was not published")
	}
	if start := atomic.LoadUint64(&publishedAfterReset.Start); start != oldTime+bucketSize {
		t.Fatalf("published start = %d, want %d", start, oldTime+bucketSize)
	}
	if value := publishedAfterReset.Value.Load().(*stagedResetValue).name; value != "new" {
		t.Fatalf("published value = %q, want new", value)
	}
}

func TestAtomicArrayUsesNativePointerStride(t *testing.T) {
	if want := uint64(unsafe.Sizeof(uintptr(0))); PtrOffSize != want {
		t.Fatalf("PtrOffSize = %d, want native pointer size %d", PtrOffSize, want)
	}
	const length = uint64(4)
	array := NewAtomicArrayWithTime(length, 10, 100, &Mock{})
	for index := uint64(0); index < length; index++ {
		if got, want := array.getBucket(index), array.data[index]; got != want {
			t.Fatalf("getBucket(%d) = %p, want data[%d] = %p", index, got, index, want)
		}
		if !array.compareAndSwap(index, array.data[index], array.data[index]) {
			t.Fatalf("compareAndSwap(%d) addressed the wrong slot", index)
		}
	}
}
