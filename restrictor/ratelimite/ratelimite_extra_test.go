package ratelimite

import (
	"context"
	"testing"
	"time"

	"github.com/juju/ratelimit"
)

// TestAllow_NoTokenLeakOnDenial covers C13: TakeAvailable(n) used to
// consume up to n tokens whether or not n were available. After M denials
// the bucket would be empty even though no caller succeeded.
func TestAllow_NoTokenLeakOnDenial(t *testing.T) {
	// 5 tokens capacity, 1/sec refill so the bucket stays nearly empty for
	// the duration of the test.
	b := ratelimit.NewBucketWithRate(1, 5)
	// Drain to 3 tokens.
	for i := 0; i < 2; i++ {
		if _, ok := b.TakeMaxDuration(1, 0); !ok {
			t.Fatal("initial drain failed unexpectedly")
		}
	}
	allow, _ := NewRateLimit(b)
	// Request 10 (more than available) repeatedly; each must be denied
	// AND must not drain the bucket.
	for i := 0; i < 50; i++ {
		if allow(time.Now(), 10) {
			t.Fatalf("over-request unexpectedly allowed at i=%d", i)
		}
	}
	// 3 tokens should still be there (modulo refill). Requesting 3 must
	// succeed; if denials leaked tokens, only 1 or 2 would be available.
	if _, _ = NewRateLimit(b); !allow(time.Now(), 3) {
		t.Fatal("tokens leaked: denials consumed the bucket")
	}
}

// TestWait_RespectsCtxCancel covers I-l: WaitFunc previously called
// juju/ratelimit's WaitMaxDuration which doesn't watch ctx; cancellation
// mid-wait was invisible.
func TestWait_RespectsCtxCancel(t *testing.T) {
	b := ratelimit.NewBucketWithRate(1, 1) // very slow refill
	// Drain the single token.
	b.TakeMaxDuration(1, 0)
	_, wait := NewRateLimit(b)
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(20 * time.Millisecond)
		cancel()
	}()
	start := time.Now()
	err := wait(ctx, 1)
	elapsed := time.Since(start)
	if err == nil {
		t.Fatal("expected wait to return error after ctx cancel")
	}
	if elapsed > 500*time.Millisecond {
		t.Fatalf("wait took %v — did not honour ctx.Done", elapsed)
	}
}
