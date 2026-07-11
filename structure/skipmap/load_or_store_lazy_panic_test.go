package skipmap

import (
	"sync/atomic"
	"testing"
	"time"
)

const lazyPanicOperationTimeout = time.Second

func TestInt64MapLoadOrStoreLazyPanicReleasesLocks(t *testing.T) {
	for _, test := range []struct {
		name  string
		store int64
	}{
		{name: "same key", store: 10},
		{name: "neighbor key", store: 11},
	} {
		t.Run(test.name, func(t *testing.T) {
			m := NewInt64()
			panicValue := &struct{ name string }{name: "int64 lazy panic"}
			assertLazyPanicValue(t, panicValue, func() {
				m.LoadOrStoreLazy(10, func() interface{} { panic(panicValue) })
			})
			assertCompletes(t, "Int64Map Store after lazy panic", func() {
				m.Store(test.store, "stored")
			})
			if value, ok := m.Load(test.store); !ok || value != "stored" {
				t.Fatalf("Load(%d) = %v, %t; want stored, true", test.store, value, ok)
			}
		})
	}
}

func TestStringMapLoadOrStoreLazyPanicReleasesLocks(t *testing.T) {
	for _, test := range []struct {
		name  string
		store string
	}{
		{name: "same key", store: "key"},
		{name: "neighbor key", store: "neighbor"},
	} {
		t.Run(test.name, func(t *testing.T) {
			m := NewString()
			panicValue := &struct{ name string }{name: "string lazy panic"}
			assertLazyPanicValue(t, panicValue, func() {
				m.LoadOrStoreLazy("key", func() interface{} { panic(panicValue) })
			})
			assertCompletes(t, "StringMap Store after lazy panic", func() {
				m.Store(test.store, "stored")
			})
			if value, ok := m.Load(test.store); !ok || value != "stored" {
				t.Fatalf("Load(%q) = %v, %t; want stored, true", test.store, value, ok)
			}
		})
	}
}

func TestLoadOrStoreLazyCallsCallbackOnce(t *testing.T) {
	t.Run("Int64Map", func(t *testing.T) {
		m := NewInt64()
		var calls int32
		callback := func() interface{} {
			atomic.AddInt32(&calls, 1)
			return "created"
		}
		if value, loaded := m.LoadOrStoreLazy(10, callback); loaded || value != "created" {
			t.Fatalf("first LoadOrStoreLazy = %v, %t", value, loaded)
		}
		if value, loaded := m.LoadOrStoreLazy(10, callback); !loaded || value != "created" {
			t.Fatalf("second LoadOrStoreLazy = %v, %t", value, loaded)
		}
		if got := atomic.LoadInt32(&calls); got != 1 {
			t.Fatalf("callback calls = %d, want 1", got)
		}
	})

	t.Run("StringMap", func(t *testing.T) {
		m := NewString()
		var calls int32
		callback := func() interface{} {
			atomic.AddInt32(&calls, 1)
			return "created"
		}
		if value, loaded := m.LoadOrStoreLazy("key", callback); loaded || value != "created" {
			t.Fatalf("first LoadOrStoreLazy = %v, %t", value, loaded)
		}
		if value, loaded := m.LoadOrStoreLazy("key", callback); !loaded || value != "created" {
			t.Fatalf("second LoadOrStoreLazy = %v, %t", value, loaded)
		}
		if got := atomic.LoadInt32(&calls); got != 1 {
			t.Fatalf("callback calls = %d, want 1", got)
		}
	})
}

func TestLoadOrStoreLazyHoldsInsertionLocksDuringCallback(t *testing.T) {
	t.Run("Int64Map", func(t *testing.T) {
		m := NewInt64()
		m.LoadOrStoreLazy(10, func() interface{} {
			if m.header.mu.TryLock() {
				m.header.mu.Unlock()
				t.Fatal("lazy callback ran without the predecessor lock")
			}
			return "created"
		})
	})
	t.Run("StringMap", func(t *testing.T) {
		m := NewString()
		m.LoadOrStoreLazy("key", func() interface{} {
			if m.header.mu.TryLock() {
				m.header.mu.Unlock()
				t.Fatal("lazy callback ran without the predecessor lock")
			}
			return "created"
		})
	})
}

func assertLazyPanicValue(t *testing.T, want interface{}, call func()) {
	t.Helper()
	defer func() {
		if got := recover(); got != want {
			t.Fatalf("recovered panic = %#v, want original %#v", got, want)
		}
	}()
	call()
	t.Fatal("LoadOrStoreLazy did not panic")
}

func assertCompletes(t *testing.T, operation string, call func()) {
	t.Helper()
	done := make(chan struct{})
	go func() {
		defer close(done)
		call()
	}()
	select {
	case <-done:
	case <-time.After(lazyPanicOperationTimeout):
		t.Fatalf("%s did not complete within %s", operation, lazyPanicOperationTimeout)
	}
}
