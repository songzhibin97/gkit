package bbr

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/songzhibin97/gkit/internal/stat"
	"github.com/songzhibin97/gkit/overload"
)

func TestLongBucketKeepsPositiveCapacityFactor(t *testing.T) {
	limiter := newLimiter(SetWindow(5*time.Minute), SetWinBucket(100)).(*BBR)
	if limiter.winBucketPerSec != 1 {
		t.Fatalf("winBucketPerSec = %d, want 1 for a three-second bucket", limiter.winBucketPerSec)
	}

	atomic.StoreInt64(&limiter.rawMaxPASS, 1000)
	atomic.StoreInt64(&limiter.rawMinRt, 1)
	if got := limiter.maxFlight(); got <= 0 {
		t.Fatalf("maxFlight() = %d, want positive capacity for observed traffic", got)
	}
}

func TestMiddlewareCountsCompletedBusinessErrorAsSuccess(t *testing.T) {
	g := NewGroup()
	wantErr := errors.New("business validation failed")
	wrapped := newLimiterWithGroup(g)(func(context.Context, interface{}) (interface{}, error) {
		time.Sleep(2 * time.Millisecond)
		return "response", wantErr
	})
	ctx := context.WithValue(context.Background(), LimitKey, "business-error")

	resp, err := wrapped(ctx, nil)
	if resp != "response" {
		t.Fatalf("response = %#v, want response", resp)
	}
	if !errors.Is(err, wantErr) {
		t.Fatalf("error = %v, want %v", err, wantErr)
	}
	limiter := g.Get("business-error").(*BBR)
	if got := limiter.passStat.Reduce(stat.Count); got != 1 {
		t.Fatalf("passStat count = %v, want 1 for a completed handler", got)
	}
	if got := limiter.rtStat.Reduce(stat.Sum); got <= 0 {
		t.Fatalf("rtStat sum = %v, want > 0 for a completed handler", got)
	}
}

func TestMiddlewarePreservesExplicitDropGate(t *testing.T) {
	g := NewGroup()
	wrapped := newLimiterWithGroup(g)(func(context.Context, interface{}) (interface{}, error) {
		time.Sleep(2 * time.Millisecond)
		return nil, nil
	})
	ctx := context.WithValue(context.Background(), LimitKey, "explicit-drop")
	ctx = context.WithValue(ctx, LimitOp, overload.Drop)

	if _, err := wrapped(ctx, nil); err != nil {
		t.Fatalf("wrapped endpoint: %v", err)
	}
	limiter := g.Get("explicit-drop").(*BBR)
	if got := limiter.passStat.Reduce(stat.Count); got != 0 {
		t.Fatalf("passStat count = %v, want 0 for an explicit Drop", got)
	}
	if got := limiter.rtStat.Reduce(stat.Sum); got != 0 {
		t.Fatalf("rtStat sum = %v, want 0 for an explicit Drop", got)
	}
}

func TestMiddlewarePropagatesNilPanicAsDrop(t *testing.T) {
	g := NewGroup()
	wrapped := newLimiterWithGroup(g)(func(context.Context, interface{}) (interface{}, error) {
		time.Sleep(2 * time.Millisecond)
		panic(nil)
	})
	ctx := context.WithValue(context.Background(), LimitKey, "nil-panic")

	returned := false
	func() {
		defer func() { _ = recover() }()
		_, _ = wrapped(ctx, nil)
		returned = true
	}()
	if returned {
		t.Fatal("panic(nil) was swallowed by the middleware")
	}

	limiter := g.Get("nil-panic").(*BBR)
	if got := atomic.LoadInt64(&limiter.inFlight); got != 0 {
		t.Fatalf("inFlight = %d, want 0 after panic(nil)", got)
	}
	if got := limiter.passStat.Reduce(stat.Count); got != 0 {
		t.Fatalf("passStat count = %v, want 0 after panic(nil)", got)
	}
	if got := limiter.rtStat.Reduce(stat.Sum); got != 0 {
		t.Fatalf("rtStat sum = %v, want 0 after panic(nil)", got)
	}
}
