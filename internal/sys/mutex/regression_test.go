package mutex

import (
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"
	"unsafe"
)

func TestIssue81MutexCountIncludesOwnerAndWaiters(t *testing.T) {
	var m Mutex
	m.Lock()
	if got := m.Count(); got != 1 {
		m.Unlock()
		t.Fatalf("locked mutex count = %d, want 1", got)
	}

	const waiters = 2
	started := make(chan struct{}, waiters)
	var wg sync.WaitGroup
	wg.Add(waiters)
	for i := 0; i < waiters; i++ {
		go func() {
			started <- struct{}{}
			m.Lock()
			m.Unlock()
			wg.Done()
		}()
	}
	for i := 0; i < waiters; i++ {
		<-started
	}

	got := 0
	for i := 0; i < 10000; i++ {
		got = m.Count()
		if got == waiters+1 {
			break
		}
		runtime.Gosched()
	}
	m.Unlock()
	wg.Wait()
	if got != waiters+1 {
		t.Fatalf("contended mutex count = %d, want %d", got, waiters+1)
	}
}

func TestIssue81TokenRecursiveMutexAcceptsZeroToken(t *testing.T) {
	var m TokenRecursiveMutex
	m.Lock(0)
	if m.Mutex.TryLock() {
		m.Mutex.Unlock()
		t.Fatal("token 0 did not acquire the underlying mutex")
	}

	m.Lock(0)
	m.Unlock(0)
	if m.Mutex.TryLock() {
		m.Mutex.Unlock()
		t.Fatal("recursive token 0 unlock released the underlying mutex too early")
	}

	m.Unlock(0)
	if !m.Mutex.TryLock() {
		t.Fatal("final token 0 unlock did not release the underlying mutex")
	}
	m.Mutex.Unlock()
}

func TestIssue81TokenRecursiveMutexSerializesDifferentTokens(t *testing.T) {
	var ownership TokenRecursiveMutex
	ownership.Lock(7)
	ownership.Unlock(7)

	var m TokenRecursiveMutex
	m.Lock(7)
	m.Lock(7)
	done := make(chan interface{}, 1)
	go func() {
		defer func() {
			if recovered := recover(); recovered != nil {
				done <- recovered
			}
		}()
		m.Lock(8)
		m.Unlock(8)
		done <- nil
	}()

	deadline := time.Now().Add(time.Second)
	for {
		state := atomic.LoadInt32((*int32)(unsafe.Pointer(&m.Mutex)))
		if state>>mutexWaiterShift > 0 {
			break
		}
		select {
		case result := <-done:
			t.Fatalf("token 8 completed while token 7 held the mutex: %v", result)
		default:
		}
		if time.Now().After(deadline) {
			t.Fatal("token 8 did not queue on the underlying mutex")
		}
		runtime.Gosched()
	}

	m.Unlock(7)
	select {
	case result := <-done:
		t.Fatalf("one recursive unlock released token 8 early: %v", result)
	default:
	}
	m.Unlock(7)
	select {
	case result := <-done:
		if result != nil {
			t.Fatalf("token 8 failed after final release: %v", result)
		}
	case <-time.After(time.Second):
		t.Fatal("token 8 did not acquire after final release")
	}
}
