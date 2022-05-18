package runtimex

import (
	"runtime"
	"sync"
	"testing"
)

func TestPin(t *testing.T) {
	n := Pin()
	Unpin()
	t.Log(n)
	var wg sync.WaitGroup
	for i := 0; i < runtime.GOMAXPROCS(0); i++ {
		wg.Add(1)
		go func() {
			n := Pin()
			Unpin()
			t.Log(n)
			wg.Done()
		}()
	}
	wg.Wait()
}
