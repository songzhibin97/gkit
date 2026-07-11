package ratelimite

import (
	"context"
	"errors"
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

// TestWait_RespectsCtxCancel covers I-l: WaitFunc must abandon its wait when
// the ctx is cancelled. The bucket refills 1 token / 200ms; after draining, a
// wait for 1 token needs ~200ms, below the ctx's 5s deadline, so
// TakeMaxDuration returns (d>0, true) and the wait ENTERS the select (the code
// under test). The old test used a no-deadline ctx and a 1/sec bucket, so
// TakeMaxDuration(1,100ms) returned (0,false) and the function returned
// ErrTimeOut before reaching the select — cancellation was never exercised,
// and replacing the select with a ctx-ignoring sleep still passed.
func TestWait_RespectsCtxCancel(t *testing.T) {
	b := ratelimit.NewBucket(200*time.Millisecond, 1)
	b.TakeMaxDuration(1, 0) // drain the initial token

	_, wait := NewRateLimit(b)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	go func() {
		time.Sleep(20 * time.Millisecond)
		cancel()
	}()

	start := time.Now()
	err := wait(ctx, 1)
	elapsed := time.Since(start)

	if !errors.Is(err, context.Canceled) {
		t.Fatalf("wait err = %v, want context.Canceled (the select must observe ctx.Done)", err)
	}
	if elapsed > 500*time.Millisecond {
		t.Fatalf("wait took %v — did not honour ctx.Done", elapsed)
	}
}

func TestWait_PreCanceledContextDoesNotConsumeToken(t *testing.T) {
	b := ratelimit.NewBucket(time.Hour, 1)
	allow, wait := NewRateLimit(b)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := wait(ctx, 1)
	if !errors.Is(err, context.Canceled) {
		t.Errorf("wait err = %v, want context.Canceled", err)
	}
	if !allow(time.Now(), 1) {
		t.Error("pre-cancelled wait consumed the immediately available token")
	}
}

func TestWait_ValidContextConsumesImmediateToken(t *testing.T) {
	b := ratelimit.NewBucket(time.Hour, 1)
	allow, wait := NewRateLimit(b)

	if err := wait(context.Background(), 1); err != nil {
		t.Fatalf("wait err = %v, want nil", err)
	}
	if allow(time.Now(), 1) {
		t.Error("valid wait did not consume the immediately available token")
	}
}
