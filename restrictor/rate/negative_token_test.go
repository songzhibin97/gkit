package rate

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/songzhibin97/gkit/restrictor"
	xrate "golang.org/x/time/rate"
)

func TestNewRateRejectsNegativeAllowWithoutMintingTokens(t *testing.T) {
	limiter := xrate.NewLimiter(0, 1)
	allow, _ := NewRate(limiter)
	now := time.Now()

	if !allow(now, 1) {
		t.Fatal("initial AllowN(1) = false, want true")
	}
	negativeAllowed := allow(now, -1)
	if allow(now, 1) {
		t.Error("AllowN(1) succeeded after negative request minted a token")
	}
	if negativeAllowed {
		t.Error("direct AllowFunc(-1) = true, want false")
	}
	if !allow(now, 0) {
		t.Fatal("AllowN(0) = false, want true")
	}
	if allow(now, 1) {
		t.Fatal("AllowN(0) consumed or minted a token")
	}
}

func TestNewRateRejectsNegativeWaitWithoutMintingTokens(t *testing.T) {
	limiter := xrate.NewLimiter(0, 1)
	allow, wait := NewRate(limiter)
	now := time.Now()

	if !allow(now, 1) {
		t.Fatal("initial AllowN(1) = false, want true")
	}
	err := wait(context.Background(), -1)
	if allow(now, 1) {
		t.Error("AllowN(1) succeeded after negative wait minted a token")
	}
	if !errors.Is(err, restrictor.ErrInvalidTokenCount) {
		t.Errorf("direct WaitFunc(-1) error = %v, want ErrInvalidTokenCount", err)
	}
	if err := wait(context.Background(), 0); err != nil {
		t.Fatalf("WaitN(0) error = %v, want nil", err)
	}
	if allow(now, 1) {
		t.Fatal("WaitN(0) consumed or minted a token")
	}
}
