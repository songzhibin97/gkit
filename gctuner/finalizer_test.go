package gctuner

import (
	"runtime"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
)

type testState struct {
	count int32
}

func TestFinalizer(t *testing.T) {
	maxCount := int32(16)
	is := assert.New(t)
	state := &testState{}
	f := newFinalizer(func() {
		n := atomic.AddInt32(&state.count, 1)
		if n > maxCount {
			t.Fatalf("cannot exec finalizer callback after f has been gc")
		}
	})
	for i := int32(1); i <= maxCount; i++ {
		runtime.GC()
		is.Equal(atomic.LoadInt32(&state.count), i)
	}
	is.Nil(f.ref)

	f.stop()
	is.Equal(atomic.LoadInt32(&state.count), maxCount)
	runtime.GC()
	is.Equal(atomic.LoadInt32(&state.count), maxCount)
	runtime.GC()
	is.Equal(atomic.LoadInt32(&state.count), maxCount)
}
