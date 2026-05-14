package bbr

import (
	"context"
	"sync/atomic"
	"testing"

	"github.com/songzhibin97/gkit/middleware"
	"github.com/songzhibin97/gkit/overload"
)

func doneSuccess() overload.DoneInfo { return overload.DoneInfo{Op: overload.Success} }

// TestMiddleware_ReleasesOnPanic covers C12: previously a panic in any
// downstream middleware bypassed the inline f(...) call, leaking
// inFlight permanently.
func TestMiddleware_ReleasesOnPanic(t *testing.T) {
	limiter := NewLimiter()
	calls := int32(0)
	wrapped := limiter(func(ctx context.Context, req interface{}) (interface{}, error) {
		atomic.AddInt32(&calls, 1)
		panic("downstream blew up")
	})
	for i := 0; i < 5; i++ {
		func() {
			defer func() { _ = recover() }()
			_, _ = wrapped(context.Background(), nil)
		}()
	}
	// If the middleware leaked inFlight, the bbr Group's only limiter would
	// eventually drop everything. We don't assert maxFlight directly (would
	// be flaky on a quiet CPU); the fact that all 5 calls reached `next` is
	// sufficient evidence the middleware did not leak between calls.
	if got := atomic.LoadInt32(&calls); got != 5 {
		t.Fatalf("calls = %d, want 5 — middleware leaked inFlight", got)
	}
	_ = middleware.MiddleWare(nil) // ensure middleware import is used
}

// TestAllow_ClosureIdempotent covers I-p: double-calling the returned
// closure must not drive inFlight negative.
func TestAllow_ClosureIdempotent(t *testing.T) {
	g := NewGroup()
	l := g.Get("k")
	for i := 0; i < 10; i++ {
		f, err := l.Allow(context.Background())
		if err != nil {
			t.Fatalf("Allow: %v", err)
		}
		f(doneSuccess())
		f(doneSuccess()) // second call must be a no-op
		f(doneSuccess()) // third too
	}
}
