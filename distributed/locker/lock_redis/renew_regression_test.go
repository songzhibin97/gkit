package lock_redis

import (
	"context"
	"errors"
	"fmt"
	"math"
	"testing"
	"time"

	"github.com/go-redis/redis/v8"
)

func TestIssue168RenewRejectsNonpositiveTTL(t *testing.T) {
	issue168RedisFixtures(t, func(t *testing.T, client *redis.Client, hook *issue168CommandHook) {
		ctx := context.Background()
		lock := NewRedisLock(client)
		for _, expire := range []int{math.MinInt, -1, 0} {
			for _, mark := range []string{"owner", "wrong-owner"} {
				t.Run(fmt.Sprintf("ttl-%d-%s", expire, mark), func(t *testing.T) {
					key := issue168Key(t, client)
					if err := lock.Lock(key, 60000, "owner"); err != nil {
						t.Fatal(err)
					}
					beforeTTL, err := client.PTTL(ctx, key).Result()
					if err != nil {
						t.Fatal(err)
					}
					beforeCalls := hook.calls.Load()
					if err := lock.Renew(key, expire, mark); !errors.Is(err, ErrRenewFailed) {
						t.Errorf("invalid renewal=%v want ErrRenewFailed", err)
					}
					if calls := hook.calls.Load() - beforeCalls; calls != 0 {
						t.Errorf("invalid renewal sent %d Redis commands", calls)
					}
					if stored, err := client.Get(ctx, key).Result(); err != nil || stored != "owner" {
						t.Errorf("invalid renewal changed lock mark=%q err=%v", stored, err)
					}
					if ttl, err := client.PTTL(ctx, key).Result(); err != nil || ttl <= 0 || ttl > beforeTTL {
						t.Errorf("invalid renewal changed TTL: before=%v after=%v err=%v", beforeTTL, ttl, err)
					}
				})
			}
		}
		t.Run("positive-and-ownership", func(t *testing.T) {
			key := issue168Key(t, client)
			if err := lock.Lock(key, 60000, "owner"); err != nil {
				t.Fatal(err)
			}
			before, err := client.PTTL(ctx, key).Result()
			if err != nil {
				t.Fatal(err)
			}
			if err := lock.Renew(key, 120000, "owner"); err != nil {
				t.Fatalf("positive renewal: %v", err)
			}
			extended, err := client.PTTL(ctx, key).Result()
			if err != nil || extended <= before || extended > 2*time.Minute {
				t.Fatalf("positive renewal TTL before=%v after=%v err=%v", before, extended, err)
			}
			if err := lock.Renew(key, 1, "wrong-owner"); !errors.Is(err, ErrRenewFailed) {
				t.Errorf("wrong-owner renewal=%v", err)
			}
			if stored, err := client.Get(ctx, key).Result(); err != nil || stored != "owner" {
				t.Errorf("wrong-owner changed mark=%q err=%v", stored, err)
			}
			if ttl, err := client.PTTL(ctx, key).Result(); err != nil || ttl <= time.Minute || ttl > extended {
				t.Errorf("wrong-owner changed TTL=%v err=%v", ttl, err)
			}
			absent := issue168Key(t, client)
			if err := lock.Renew(absent, 1000, "owner"); !errors.Is(err, ErrRenewFailed) {
				t.Errorf("missing lock renewal=%v", err)
			}
		})
	})
}
