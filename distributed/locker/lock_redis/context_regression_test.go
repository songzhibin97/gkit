package lock_redis

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/go-redis/redis/v8"
)

type issue104BlockingEvalHook struct {
	entered chan struct{}
}

func (h *issue104BlockingEvalHook) BeforeProcess(ctx context.Context, cmd redis.Cmder) (context.Context, error) {
	if cmd.Name() != "eval" {
		return ctx, nil
	}
	select {
	case h.entered <- struct{}{}:
	default:
	}
	<-ctx.Done()
	return ctx, ctx.Err()
}

func (*issue104BlockingEvalHook) AfterProcess(context.Context, redis.Cmder) error { return nil }
func (*issue104BlockingEvalHook) BeforeProcessPipeline(ctx context.Context, _ []redis.Cmder) (context.Context, error) {
	return ctx, nil
}
func (*issue104BlockingEvalHook) AfterProcessPipeline(context.Context, []redis.Cmder) error {
	return nil
}

type issue104ContextLocker interface {
	LockContext(context.Context, string, int, string) error
	UnlockContext(context.Context, string, string) error
}

func TestRedisLockerContextCancelsBlockedEval(t *testing.T) {
	for _, tt := range []struct {
		name string
		call func(issue104ContextLocker, context.Context) error
	}{
		{name: "lock", call: func(lock issue104ContextLocker, ctx context.Context) error {
			return lock.LockContext(ctx, "key", 1000, "mark")
		}},
		{name: "unlock", call: func(lock issue104ContextLocker, ctx context.Context) error {
			return lock.UnlockContext(ctx, "key", "mark")
		}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			mr := miniredis.RunT(t)
			client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
			t.Cleanup(func() { _ = client.Close() })
			entered := make(chan struct{}, 1)
			client.AddHook(&issue104BlockingEvalHook{entered: entered})
			lock := NewRedisLock(client).(issue104ContextLocker)
			ctx, cancel := context.WithCancel(context.Background())
			done := make(chan error, 1)
			go func() { done <- tt.call(lock, ctx) }()
			select {
			case <-entered:
			case <-time.After(time.Second):
				t.Fatal("Eval did not start")
			}
			cancel()
			select {
			case err := <-done:
				if !errors.Is(err, context.Canceled) {
					t.Fatalf("operation error = %v, want context.Canceled", err)
				}
			case <-time.After(time.Second):
				t.Fatal("context cancellation did not stop Eval")
			}
		})
	}
}

func TestRedisLockerContextCancelsRetryWait(t *testing.T) {
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	if err := client.Set(context.Background(), "key", "other", time.Minute).Err(); err != nil {
		t.Fatal(err)
	}
	lock := NewRedisLock(client, SetRetries(10), SetInterval(time.Second)).(issue104ContextLocker)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	started := time.Now()
	err := lock.LockContext(ctx, "key", 1000, "mark")
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("LockContext error = %v, want context deadline", err)
	}
	if elapsed := time.Since(started); elapsed > 250*time.Millisecond {
		t.Fatalf("retry wait ignored context: %s", elapsed)
	}
}

func TestRedisLockerAdvertisesContextCapability(t *testing.T) {
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = client.Close() })

	lock := NewRedisLock(client)
	if _, ok := lock.(issue104ContextLocker); !ok {
		t.Fatalf("NewRedisLock returned %T, want context-aware lock capability", lock)
	}
}
