package lscq_test

import (
	"runtime"
	"testing"
	"unsafe"

	"github.com/songzhibin97/gkit/structure/lscq"
)

//go:noinline
func enqueueLocal(q *lscq.PointerQueue, value uint64) {
	local := value
	q.Enqueue(unsafe.Pointer(&local))
}

// The caller must be able to return after handing ownership of a local to the queue.
func TestPointerQueueLocalLifetime(t *testing.T) {
	q := lscq.NewPointer()
	enqueueLocal(q, 101)
	enqueueLocal(q, 202)
	assertPointerValues(t, q)
}

func TestPointerQueueLocalLifetimeAfterGC(t *testing.T) {
	q := lscq.NewPointer()
	enqueueLocal(q, 101)
	enqueueLocal(q, 202)
	runtime.GC()
	runtime.GC()
	assertPointerValues(t, q)
}

var heapControl *uint64

//go:noinline
func enqueueHeapControl(q *lscq.PointerQueue, value uint64) {
	local := value
	heapControl = &local // Force escape independently of Enqueue's contract.
	q.Enqueue(unsafe.Pointer(&local))
}

func TestPointerQueueHeapControl(t *testing.T) {
	q := lscq.NewPointer()
	enqueueHeapControl(q, 101)
	enqueueHeapControl(q, 202)
	heapControl = nil // The queue is now the only owner of both values.
	runtime.GC()
	assertPointerValues(t, q)
}

func assertPointerValues(t *testing.T, q *lscq.PointerQueue) {
	t.Helper()
	for _, want := range []uint64{101, 202} {
		data, ok := q.Dequeue()
		if !ok || data == nil {
			t.Fatalf("Dequeue = %p, %t; want pointer to %d", data, ok, want)
		}
		if got := *(*uint64)(data); got != want {
			t.Errorf("Dequeue value = %d, want %d", got, want)
		}
	}
	if data, ok := q.Dequeue(); ok || data != nil {
		t.Fatalf("empty Dequeue = %p, %t", data, ok)
	}
}
