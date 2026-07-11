package singleflight

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"runtime/debug"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestDo(t *testing.T) {
	g := NewSingleFlight()
	v, err, _ := g.Do("key", func() (interface{}, error) {
		return "bar", nil
	})
	if got, want := fmt.Sprintf("%v (%T)", v, v), "bar (string)"; got != want {
		t.Errorf("Do = %v; want %v", got, want)
	}
	if err != nil {
		t.Errorf("Do error = %v", err)
	}
}

func TestDoErr(t *testing.T) {
	g := NewSingleFlight()
	someErr := errors.New("Some error")
	v, err, _ := g.Do("key", func() (interface{}, error) {
		return nil, someErr
	})
	if err != someErr {
		t.Errorf("Do error = %v; want someErr %v", err, someErr)
	}
	if v != nil {
		t.Errorf("unexpected non-nil value %#v", v)
	}
}

func TestForgetStartsNewCallWithoutCancelingOldCall(t *testing.T) {
	type result struct {
		value  interface{}
		err    error
		shared bool
	}

	g := NewSingleFlight()
	oldStarted := make(chan struct{})
	releaseOld := make(chan struct{})
	var releaseOldOnce sync.Once
	releaseOldCall := func() {
		releaseOldOnce.Do(func() { close(releaseOld) })
	}
	t.Cleanup(releaseOldCall)
	oldDone := make(chan result, 1)
	go func() {
		value, err, shared := g.Do("key", func() (interface{}, error) {
			close(oldStarted)
			<-releaseOld
			return "old", nil
		})
		oldDone <- result{value: value, err: err, shared: shared}
	}()
	waitForSignal(t, oldStarted, "old call to start")

	g.Forget("key")

	newStarted := make(chan struct{})
	newDone := make(chan result, 1)
	go func() {
		value, err, shared := g.Do("key", func() (interface{}, error) {
			close(newStarted)
			return "new", nil
		})
		newDone <- result{value: value, err: err, shared: shared}
	}()
	waitForSignal(t, newStarted, "new call to start independently")

	newResult := waitForResult(t, newDone, "new call to finish")
	if newResult.value != "new" || newResult.err != nil || newResult.shared {
		t.Fatalf("new call = (%v, %v, shared=%t), want (new, nil, shared=false)", newResult.value, newResult.err, newResult.shared)
	}
	select {
	case oldResult := <-oldDone:
		t.Fatalf("old call finished before release: (%v, %v, shared=%t)", oldResult.value, oldResult.err, oldResult.shared)
	default:
	}

	releaseOldCall()
	oldResult := waitForResult(t, oldDone, "old call to finish after release")
	if oldResult.value != "old" || oldResult.err != nil {
		t.Fatalf("old call = (%v, %v), want (old, nil)", oldResult.value, oldResult.err)
	}
}

func waitForSignal(t *testing.T, ch <-chan struct{}, operation string) {
	t.Helper()
	timer := time.NewTimer(time.Second)
	defer timer.Stop()
	select {
	case <-ch:
	case <-timer.C:
		t.Fatalf("timed out waiting for %s", operation)
	}
}

func waitForResult[T any](t *testing.T, ch <-chan T, operation string) T {
	t.Helper()
	timer := time.NewTimer(time.Second)
	defer timer.Stop()
	select {
	case result := <-ch:
		return result
	case <-timer.C:
		t.Fatalf("timed out waiting for %s", operation)
		var zero T
		return zero
	}
}

func TestDoDupSuppress(t *testing.T) {
	g := NewSingleFlight()
	var wg1, wg2 sync.WaitGroup
	c := make(chan string, 1)
	var calls int32
	fn := func() (interface{}, error) {
		if atomic.AddInt32(&calls, 1) == 1 {
			// First invocation.
			wg1.Done()
		}
		v := <-c
		c <- v // pump; make available for any future calls

		time.Sleep(10 * time.Millisecond) // let more goroutines enter Do

		return v, nil
	}

	const n = 10
	wg1.Add(1)
	for i := 0; i < n; i++ {
		wg1.Add(1)
		wg2.Add(1)
		go func() {
			defer wg2.Done()
			wg1.Done()
			v, err, _ := g.Do("key", fn)
			if err != nil {
				t.Errorf("Do error: %v", err)
				return
			}
			if s, _ := v.(string); s != "bar" {
				t.Errorf("Do = %T %v; want %q", v, v, "bar")
			}
		}()
	}
	wg1.Wait()

	c <- "bar"
	wg2.Wait()
	if got := atomic.LoadInt32(&calls); got <= 0 || got >= n {
		t.Errorf("number of calls = %d; want over 0 and less than %d", got, n)
	}
}

func TestPanicDo(t *testing.T) {
	g := NewSingleFlight()
	fn := func() (interface{}, error) {
		panic("invalid memory address or nil pointer dereference")
	}

	const n = 5
	waited := int32(n)
	panicCount := int32(0)
	done := make(chan struct{})
	for i := 0; i < n; i++ {
		go func() {
			defer func() {
				if err := recover(); err != nil {
					t.Logf("Got panic: %v\n%s", err, debug.Stack())
					atomic.AddInt32(&panicCount, 1)
				}

				if atomic.AddInt32(&waited, -1) == 0 {
					close(done)
				}
			}()

			_, _, _ = g.Do("key", fn)
		}()
	}

	select {
	case <-done:
		if panicCount != n {
			t.Errorf("Expect %d panic, but got %d", n, panicCount)
		}
	case <-time.After(time.Second):
		t.Fatalf("Do hangs")
	}
}

func TestGoexitDo(t *testing.T) {
	g := NewSingleFlight()
	fn := func() (interface{}, error) {
		runtime.Goexit()
		return nil, nil
	}

	const n = 5
	waited := int32(n)
	done := make(chan struct{})
	for i := 0; i < n; i++ {
		go func() {
			var err error
			defer func() {
				if err != nil {
					t.Errorf("Error should be nil, but got: %v", err)
				}
				if atomic.AddInt32(&waited, -1) == 0 {
					close(done)
				}
			}()
			_, err, _ = g.Do("key", fn)
		}()
	}

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatalf("Do hangs")
	}
}

func TestPanicDoChan(t *testing.T) {
	if runtime.GOOS == "js" {
		t.Skipf("js does not support exec")
	}

	if os.Getenv("TEST_PANIC_DOCHAN") != "" {
		defer func() {
			_ = recover()
		}()

		g := NewSingleFlight()
		ch := g.DoChan("", func() (interface{}, error) {
			panic("Panicking in DoChan")
		})
		t.Log(<-ch)
		t.Fatalf("DoChan unexpectedly returned")
	}

	t.Parallel()

	cmd := exec.Command(os.Args[0], "-test.run="+t.Name(), "-test.v")
	cmd.Env = append(os.Environ(), "TEST_PANIC_DOCHAN=1")
	out := new(bytes.Buffer)
	cmd.Stdout = out
	cmd.Stderr = out
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}

	err := cmd.Wait()
	t.Logf("%s:\n%s", strings.Join(cmd.Args, " "), out)
	if err == nil {
		t.Errorf("Test subprocess passed; want a crash due to panic in DoChan")
	}
	if bytes.Contains(out.Bytes(), []byte("DoChan unexpectedly")) {
		t.Errorf("Test subprocess failed with an unexpected failure mode.")
	}
	if !bytes.Contains(out.Bytes(), []byte("Panicking in DoChan")) {
		t.Errorf("Test subprocess failed, but the crash isn't caused by panicking in DoChan")
	}
}

func TestPanicDoSharedByDoChan(t *testing.T) {
	if runtime.GOOS == "js" {
		t.Skipf("js does not support exec")
	}

	if os.Getenv("TEST_PANIC_DOCHAN") != "" {
		blocked := make(chan struct{})
		unblock := make(chan struct{})

		g := NewSingleFlight()
		go func() {
			defer func() {
				_ = recover()
			}()
			_, _, _ = g.Do("", func() (interface{}, error) {
				close(blocked)
				<-unblock
				panic("Panicking in Do")
			})
		}()

		<-blocked
		ch := g.DoChan("", func() (interface{}, error) {
			panic("DoChan unexpectedly executed callback")
		})
		close(unblock)
		<-ch
		t.Fatalf("DoChan unexpectedly returned")
	}

	t.Parallel()

	cmd := exec.Command(os.Args[0], "-test.run="+t.Name(), "-test.v")
	cmd.Env = append(os.Environ(), "TEST_PANIC_DOCHAN=1")
	out := new(bytes.Buffer)
	cmd.Stdout = out
	cmd.Stderr = out
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}

	err := cmd.Wait()
	t.Logf("%s:\n%s", strings.Join(cmd.Args, " "), out)
	if err == nil {
		t.Errorf("Test subprocess passed; want a crash due to panic in Do shared by DoChan")
	}
	if bytes.Contains(out.Bytes(), []byte("DoChan unexpectedly")) {
		t.Errorf("Test subprocess failed with an unexpected failure mode.")
	}
	if !bytes.Contains(out.Bytes(), []byte("Panicking in Do")) {
		t.Errorf("Test subprocess failed, but the crash isn't caused by panicking in Do")
	}
}
