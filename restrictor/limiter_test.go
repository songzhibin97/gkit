package restrictor

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestAllowFuncRejectsNegativeCount(t *testing.T) {
	called := false
	allow := AllowFunc(func(time.Time, int) bool {
		called = true
		return true
	})

	if allow.AllowN(time.Now(), -1) {
		t.Fatal("AllowN(-1) = true, want false")
	}
	if called {
		t.Fatal("AllowN(-1) called the wrapped limiter")
	}
}

func TestWaitFuncRejectsNegativeCount(t *testing.T) {
	called := false
	wait := WaitFunc(func(context.Context, int) error {
		called = true
		return nil
	})

	if err := wait.WaitN(context.Background(), -1); !errors.Is(err, ErrInvalidTokenCount) {
		t.Fatalf("WaitN(-1) error = %v, want ErrInvalidTokenCount", err)
	}
	if called {
		t.Fatal("WaitN(-1) called the wrapped limiter")
	}
}

func TestFuncAdaptersPreserveZeroCount(t *testing.T) {
	allowCalls := 0
	allow := AllowFunc(func(_ time.Time, n int) bool {
		allowCalls++
		return n == 0
	})
	if !allow.AllowN(time.Now(), 0) {
		t.Fatal("AllowN(0) = false, want true")
	}
	if allowCalls != 1 {
		t.Fatalf("AllowN(0) calls = %d, want 1", allowCalls)
	}

	waitCalls := 0
	wait := WaitFunc(func(_ context.Context, n int) error {
		waitCalls++
		if n != 0 {
			t.Fatalf("WaitN forwarded n = %d, want 0", n)
		}
		return nil
	})
	if err := wait.WaitN(context.Background(), 0); err != nil {
		t.Fatalf("WaitN(0) error = %v, want nil", err)
	}
	if waitCalls != 1 {
		t.Fatalf("WaitN(0) calls = %d, want 1", waitCalls)
	}
}
