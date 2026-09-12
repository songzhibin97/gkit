package buffer

import (
	"errors"
	"io"
	"sync"
	"testing"
	"time"
)

// Cond.Wait registers the waiter before calling L.Unlock. Only Cond.Wait uses
// this wrapper; normal pipe operations continue to lock and unlock the same mu.
type pipeWaitObserver struct {
	mu     *sync.Mutex
	waited chan struct{}
}

func (o *pipeWaitObserver) Lock() { o.mu.Lock() }

func (o *pipeWaitObserver) Unlock() {
	o.mu.Unlock()
	o.waited <- struct{}{}
}

type pipeReadResult struct {
	n    int
	data byte
	err  error
}

func startWaitingPipeReaders(t *testing.T, count int) (*pipe, <-chan pipeReadResult) {
	t.Helper()
	p := NewPipe(16).(*pipe)
	observer := &pipeWaitObserver{mu: &p.mu, waited: make(chan struct{}, count*4)}
	p.c.L = observer
	results := make(chan pipeReadResult, count)
	var readers sync.WaitGroup
	readers.Add(count)
	t.Cleanup(func() {
		p.CloseWithError(nil)
		joined := make(chan struct{})
		go func() {
			readers.Wait()
			close(joined)
		}()
		select {
		case <-joined:
			if err := PutIoPool(p); err != nil {
				t.Error(err)
			}
		case <-time.After(2 * time.Second):
			t.Error("pipe readers did not exit after cleanup close")
		}
	})
	for i := 0; i < count; i++ {
		go func() {
			defer readers.Done()
			var data [1]byte
			n, err := p.Read(data[:])
			results <- pipeReadResult{n: n, data: data[0], err: err}
		}()
	}
	timer := time.NewTimer(2 * time.Second)
	defer timer.Stop()
	for i := 0; i < count; i++ {
		select {
		case <-observer.waited:
		case <-timer.C:
			t.Fatalf("only %d/%d readers registered with Cond.Wait", i, count)
		}
	}
	return p, results
}

func receivePipeRead(t *testing.T, results <-chan pipeReadResult) pipeReadResult {
	t.Helper()
	select {
	case result := <-results:
		return result
	case <-time.After(2 * time.Second):
		t.Fatal("reader remained blocked after pipe notification")
		return pipeReadResult{}
	}
}

// Regression for #164, item 01-02: one write must let all readers consume its
// buffered data, even when each reader requests only one byte.
func TestPipeWriteWakesReadersForAllBufferedData(t *testing.T) {
	p, results := startWaitingPipeReaders(t, 2)
	if n, err := p.Write([]byte("ab")); n != 2 || err != nil {
		t.Fatalf("Write = (%d, %v), want (2, nil)", n, err)
	}
	counts := make(map[byte]int)
	for i := 0; i < 2; i++ {
		result := receivePipeRead(t, results)
		if result.n != 1 || result.err != nil {
			t.Fatalf("Read = (%d, %v), want (1, nil)", result.n, result.err)
		}
		counts[result.data]++
	}
	if len(counts) != 2 || counts['a'] != 1 || counts['b'] != 1 {
		t.Fatalf("read byte counts = %v, want one a and one b", counts)
	}
	if got := p.Len(); got != 0 {
		t.Fatalf("Len = %d, want 0", got)
	}
}

func TestPipeExtraReadersWaitForWriteOrClose(t *testing.T) {
	closeErr := errors.New("pipe closed by test")
	for _, test := range []struct {
		name     string
		write    bool
		closeErr error
		wantErr  error
	}{
		{name: "next_write", write: true},
		{name: "close_eof", wantErr: io.EOF},
		{name: "close_error", closeErr: closeErr, wantErr: closeErr},
	} {
		t.Run(test.name, func(t *testing.T) {
			p, results := startWaitingPipeReaders(t, 3)
			if n, err := p.Write([]byte("a")); n != 1 || err != nil {
				t.Fatalf("first Write = (%d, %v), want (1, nil)", n, err)
			}
			first := receivePipeRead(t, results)
			if first.n != 1 || first.data != 'a' || first.err != nil {
				t.Fatalf("first Read = %+v, want one a and nil error", first)
			}
			select {
			case result := <-results:
				t.Fatalf("extra reader returned without data or close: %+v", result)
			default:
			}
			if test.write {
				if n, err := p.Write([]byte("bb")); n != 2 || err != nil {
					t.Fatalf("next Write = (%d, %v), want (2, nil)", n, err)
				}
			} else {
				p.CloseWithError(test.closeErr)
			}
			for i := 0; i < 2; i++ {
				result := receivePipeRead(t, results)
				if test.write {
					if result.n != 1 || result.data != 'b' || result.err != nil {
						t.Fatalf("later Read = %+v, want one b and nil error", result)
					}
				} else if result.n != 0 || result.err != test.wantErr {
					t.Fatalf("closed Read = (%d, %v), want (0, %v)", result.n, result.err, test.wantErr)
				}
			}
			if got := p.Len(); got != 0 {
				t.Fatalf("Len = %d, want 0", got)
			}
		})
	}
}
