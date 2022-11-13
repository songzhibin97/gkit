package gctuner

import (
	"runtime"
	"runtime/debug"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFinalizer(t *testing.T) {
	// disable gc
	debug.SetGCPercent(-1)
	defer debug.SetGCPercent(100)

	maxCount := int32(16)
	is := assert.New(t)
	var count int32
	f := newFinalizer(func() {
		n := atomic.AddInt32(&count, 1)
		if n > maxCount {
			t.Fatalf("cannot exec finalizer callback after f has been gc")
		}
	})
	for atomic.LoadInt32(&count) < maxCount {
		runtime.GC()
	}
	is.Nil(f.ref)
	f.stop()
	// when f stopped, finalizer callback will not be called
	lastCount := atomic.LoadInt32(&count)
	for i := 0; i < 10; i++ {
		runtime.GC()
		is.Equal(lastCount, atomic.LoadInt32(&count))
	}
}
