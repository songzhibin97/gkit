package lock_redis

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"math"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/go-redis/redis/v8"
)

type issue168CommandHook struct {
	calls     atomic.Int64
	afterEval func()
}

func (h *issue168CommandHook) BeforeProcess(ctx context.Context, _ redis.Cmder) (context.Context, error) {
	h.calls.Add(1)
	return ctx, nil
}
func (h *issue168CommandHook) AfterProcess(_ context.Context, cmd redis.Cmder) error {
	if cmd.Name() == "eval" && h.afterEval != nil {
		h.afterEval()
	}
	return nil
}
func (h *issue168CommandHook) BeforeProcessPipeline(ctx context.Context, cmds []redis.Cmder) (context.Context, error) {
	h.calls.Add(int64(len(cmds)))
	return ctx, nil
}
func (*issue168CommandHook) AfterProcessPipeline(context.Context, []redis.Cmder) error { return nil }

func issue168RedisFixtures(t *testing.T, run func(*testing.T, *redis.Client, *issue168CommandHook)) {
	t.Helper()
	for _, name := range []string{"miniredis", "live"} {
		t.Run(name, func(t *testing.T) {
			addr := os.Getenv("GKIT_REDIS_ADDR")
			if name == "miniredis" {
				addr = miniredis.RunT(t).Addr()
			} else if addr == "" {
				t.Skip("#168 live Redis regression requires GKIT_REDIS_ADDR; owned fixture required before acceptance")
			}
			client := redis.NewClient(&redis.Options{Addr: addr})
			t.Cleanup(func() {
				if err := client.Close(); err != nil {
					t.Error(err)
				}
			})
			hook := &issue168CommandHook{}
			client.AddHook(hook)
			run(t, client, hook)
		})
	}
}

func issue168Key(t *testing.T, client *redis.Client) string {
	t.Helper()
	var value [16]byte
	if _, err := rand.Read(value[:]); err != nil {
		t.Fatal(err)
	}
	key := "gkit-issue168-" + hex.EncodeToString(value[:])
	t.Cleanup(func() {
		if err := client.Del(context.Background(), key).Err(); err != nil {
			t.Error(err)
		}
	})
	return key
}

func TestIssue168RetriesAlwaysAttemptAcquisition(t *testing.T) {
	issue168RedisFixtures(t, func(t *testing.T, client *redis.Client, hook *issue168CommandHook) {
		for _, retries := range []int{math.MinInt, -1, 0, 1, math.MaxInt} {
			t.Run(fmt.Sprintf("retries-%d", retries), func(t *testing.T) {
				ctx := context.Background()
				lock := NewRedisLock(client, SetRetries(retries), SetInterval(time.Millisecond))
				key := issue168Key(t, client)
				before := hook.calls.Load()
				if err := lock.Lock(key, 60000, "owner"); err != nil {
					t.Fatal(err)
				}
				if calls := hook.calls.Load() - before; calls != 1 {
					t.Errorf("free key acquisition commands=%d want 1", calls)
				}
				if mark, err := client.Get(ctx, key).Result(); err != nil || mark != "owner" {
					t.Errorf("reported acquisition but stored mark=%q err=%v", mark, err)
				}
				if ttl, err := client.PTTL(ctx, key).Result(); err != nil || ttl <= 0 || ttl > time.Minute {
					t.Errorf("acquired lock TTL=%v err=%v", ttl, err)
				}
				canceled, cancel := context.WithCancel(ctx)
				cancel()
				fresh := issue168Key(t, client)
				before = hook.calls.Load()
				if err := lock.(issue104ContextLocker).LockContext(canceled, fresh, 60000, "owner"); !errors.Is(err, context.Canceled) {
					t.Errorf("canceled acquisition=%v", err)
				}
				if hook.calls.Load() != before {
					t.Error("canceled acquisition accessed Redis")
				}
				if exists, err := client.Exists(ctx, fresh).Result(); err != nil || exists != 0 {
					t.Errorf("canceled acquisition key exists=%d err=%v", exists, err)
				}
				occupied := issue168Key(t, client)
				if err := client.Set(ctx, occupied, "other-owner", time.Minute).Err(); err != nil {
					t.Fatal(err)
				}
				before = hook.calls.Load()
				if retries == math.MaxInt {
					attemptCtx, stop := context.WithCancel(ctx)
					hook.afterEval = stop
					err := lock.(issue104ContextLocker).LockContext(attemptCtx, occupied, 60000, "owner")
					hook.afterEval = nil
					stop()
					if !errors.Is(err, context.Canceled) {
						t.Errorf("large retry cancellation=%v", err)
					}
					if calls := hook.calls.Load() - before; calls != 1 {
						t.Errorf("large retry attempts=%d want 1 before cancellation", calls)
					}
				} else {
					if err := lock.Lock(occupied, 60000, "owner"); !errors.Is(err, ErrLockFailed) {
						t.Errorf("occupied acquisition=%v", err)
					}
					expected := int64(1)
					if retries == 1 {
						expected = 2
					}
					if calls := hook.calls.Load() - before; calls != expected {
						t.Errorf("occupied attempts=%d want %d", calls, expected)
					}
				}
				if mark, err := client.Get(ctx, occupied).Result(); err != nil || mark != "other-owner" {
					t.Errorf("occupied mark changed=%q err=%v", mark, err)
				}
			})
		}
	})
}
