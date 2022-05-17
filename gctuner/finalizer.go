package gctuner

import (
	"runtime"
	"sync/atomic"
)

type finalizerCallback func()

type finalizer struct {
	ref      *finalizerRef
	callback finalizerCallback
	stopped  int32
}

func (f *finalizer) stop() {
	atomic.StoreInt32(&f.stopped, 1)
}

type finalizerRef struct {
	parent *finalizer
}

func finalizerHandler(f *finalizerRef) {
	// stop calling callback

	if atomic.LoadInt32(&f.parent.stopped) == 1 {
		return
	}
	f.parent.callback()
	runtime.SetFinalizer(f, finalizerHandler)
}

// newFinalizer return a finalizer object and caller should save it to make sure it will not be gc.
// the go runtime promise the callback function should be called every gc time.
func newFinalizer(callback finalizerCallback) *finalizer {
	f := &finalizer{
		callback: callback,
	}
	f.ref = &finalizerRef{parent: f}
	runtime.SetFinalizer(f.ref, finalizerHandler)
	f.ref = nil // trigger gc
	return f
}
