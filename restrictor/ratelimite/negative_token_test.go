package ratelimite

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/juju/ratelimit"
	"github.com/songzhibin97/gkit/restrictor"
)

func TestNewRateLimitRejectsNegativeCounts(t *testing.T) {
	bucket := ratelimit.NewBucket(time.Hour, 2)
	allow, wait := NewRateLimit(bucket)

	if allow(time.Now(), -1) {
		t.Fatal("direct AllowFunc(-1) = true, want false")
	}
	if err := wait(context.Background(), -1); !errors.Is(err, restrictor.ErrInvalidTokenCount) {
		t.Fatalf("direct WaitFunc(-1) error = %v, want ErrInvalidTokenCount", err)
	}
	if !allow(time.Now(), 0) {
		t.Fatal("AllowN(0) = false, want true")
	}
	if err := wait(context.Background(), 0); err != nil {
		t.Fatalf("WaitN(0) error = %v, want nil", err)
	}
	if !allow(time.Now(), 2) {
		t.Fatal("zero or negative requests consumed bucket tokens")
	}
}
