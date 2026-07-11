package bbr

import (
	"context"
	"sync/atomic"
	"testing"

	"github.com/songzhibin97/gkit/internal/stat"
	"github.com/songzhibin97/gkit/overload"
)

func TestMiddlewareLegacyContextValues(t *testing.T) {
	g := NewGroup()
	const key = "legacy"
	called := false
	wrapped := newLimiterWithGroup(g)(func(context.Context, interface{}) (interface{}, error) {
		called = true
		if got := atomic.LoadInt64(&g.Get(key).(*BBR).inFlight); got != 1 {
			t.Fatalf("legacy limiter inFlight = %d, want 1 while endpoint runs", got)
		}
		return nil, nil
	})
	ctx := context.WithValue(context.Background(), LimitKey, key)
	ctx = context.WithValue(ctx, LimitOp, overload.Drop)

	if _, err := wrapped(ctx, nil); err != nil {
		t.Fatalf("wrapped endpoint: %v", err)
	}
	if !called {
		t.Fatal("wrapped endpoint was not called")
	}
	limiter := g.Get(key).(*BBR)
	if got := limiter.passStat.Reduce(stat.Count); got != 0 {
		t.Fatalf("legacy Drop pass count = %v, want 0", got)
	}
}

func TestMiddlewareWrongLegacyTypesFallBackWithoutPanic(t *testing.T) {
	g := NewGroup()
	called := false
	wrapped := newLimiterWithGroup(g)(func(context.Context, interface{}) (interface{}, error) {
		called = true
		if got := atomic.LoadInt64(&g.Get("default").(*BBR).inFlight); got != 1 {
			t.Fatalf("default limiter inFlight = %d, want 1 while endpoint runs", got)
		}
		return nil, nil
	})
	ctx := context.WithValue(context.Background(), LimitKey, 42)
	ctx = context.WithValue(ctx, LimitOp, "drop")

	var panicValue interface{}
	var err error
	func() {
		defer func() { panicValue = recover() }()
		_, err = wrapped(ctx, nil)
	}()
	if panicValue != nil {
		t.Fatalf("middleware panicked on colliding legacy keys: %v", panicValue)
	}
	if err != nil {
		t.Fatalf("wrapped endpoint: %v", err)
	}
	if !called {
		t.Fatal("wrapped endpoint was not called")
	}
}

func TestMiddlewareTypedContextHelpers(t *testing.T) {
	g := NewGroup()
	const key = "typed"
	wrapped := newLimiterWithGroup(g)(func(context.Context, interface{}) (interface{}, error) {
		if got := atomic.LoadInt64(&g.Get(key).(*BBR).inFlight); got != 1 {
			t.Fatalf("typed limiter inFlight = %d, want 1 while endpoint runs", got)
		}
		return nil, nil
	})
	ctx := WithLimitKey(context.Background(), key)
	ctx = WithLimitOp(ctx, overload.Drop)

	if _, err := wrapped(ctx, nil); err != nil {
		t.Fatalf("wrapped endpoint: %v", err)
	}
	limiter := g.Get(key).(*BBR)
	if got := limiter.passStat.Reduce(stat.Count); got != 0 {
		t.Fatalf("typed Drop pass count = %v, want 0", got)
	}
}

func TestMiddlewareTypedContextTakesPriorityOverLegacy(t *testing.T) {
	g := NewGroup()
	const typedKey = "typed-priority"
	const legacyKey = "legacy-shadowed"
	wrapped := newLimiterWithGroup(g)(func(context.Context, interface{}) (interface{}, error) {
		if got := atomic.LoadInt64(&g.Get(typedKey).(*BBR).inFlight); got != 1 {
			t.Fatalf("typed limiter inFlight = %d, want 1 while endpoint runs", got)
		}
		if got := atomic.LoadInt64(&g.Get(legacyKey).(*BBR).inFlight); got != 0 {
			t.Fatalf("legacy limiter inFlight = %d, want 0 when typed key is present", got)
		}
		return nil, nil
	})
	ctx := context.WithValue(context.Background(), LimitKey, legacyKey)
	ctx = context.WithValue(ctx, LimitOp, overload.Success)
	ctx = WithLimitKey(ctx, typedKey)
	ctx = WithLimitOp(ctx, overload.Drop)

	if _, err := wrapped(ctx, nil); err != nil {
		t.Fatalf("wrapped endpoint: %v", err)
	}
	typedLimiter := g.Get(typedKey).(*BBR)
	if got := typedLimiter.passStat.Reduce(stat.Count); got != 0 {
		t.Fatalf("typed Drop pass count = %v, want 0", got)
	}
}
