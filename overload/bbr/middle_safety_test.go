package bbr

import (
	"context"
	"sync/atomic"
	"testing"

	"github.com/songzhibin97/gkit/middleware"
	"github.com/songzhibin97/gkit/overload"
)

func doneSuccess() overload.DoneInfo { return overload.DoneInfo{Op: overload.Success} }

// TestMiddleware_ReleasesOnPanic covers C12: a panic in any downstream
// middleware must still release the inFlight slot. We drive the middleware over
// a Group we control (newLimiterWithGroup) so we can read the same limiter's
// inFlight afterwards: a leak leaves it elevated, the defer's f(Drop) returns it
// to zero. Asserting calls==5 (the old test) proved nothing — the slot could
// leak on every call and `next` would still be reached.
func TestMiddleware_ReleasesOnPanic(t *testing.T) {
	g := NewGroup()
	mw := newLimiterWithGroup(g)
	wrapped := mw(func(ctx context.Context, req interface{}) (interface{}, error) {
		panic("downstream blew up")
	})
	ctx := context.WithValue(context.Background(), LimitKey, "k")
	for i := 0; i < 5; i++ {
		func() {
			defer func() { _ = recover() }()
			_, _ = wrapped(ctx, nil)
		}()
	}
	l := g.Get("k").(*BBR)
	if got := atomic.LoadInt64(&l.inFlight); got != 0 {
		t.Fatalf("inFlight = %d, want 0 — middleware leaked the slot on panic", got)
	}
	_ = middleware.MiddleWare(nil) // ensure middleware import is used
}

// TestAllow_ClosureIdempotent covers I-p: double-calling the returned closure
// must not drive inFlight negative. Asserting only that Allow returned no error
// (the old test) missed the bug; assert inFlight returns to zero.
func TestAllow_ClosureIdempotent(t *testing.T) {
	g := NewGroup()
	l := g.Get("k").(*BBR)
	for i := 0; i < 10; i++ {
		f, err := l.Allow(context.Background())
		if err != nil {
			t.Fatalf("Allow: %v", err)
		}
		f(doneSuccess())
		f(doneSuccess()) // second call must be a no-op
		f(doneSuccess()) // third too
	}
	if got := atomic.LoadInt64(&l.inFlight); got != 0 {
		t.Fatalf("inFlight = %d, want 0 — non-idempotent release drove it negative", got)
	}
}
