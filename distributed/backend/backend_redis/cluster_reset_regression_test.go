package backend_redis

import (
	"context"
	"github.com/go-redis/redis/v8"
	"os"
	"strings"
	"testing"
)

func TestIssue167ClusterBatchReset(t *testing.T) {
	raw := os.Getenv("GKIT_REDIS_CLUSTER_ADDRS")
	if raw == "" {
		t.Skip("#167 live regression requires GKIT_REDIS_CLUSTER_ADDRS; owned fixture required before acceptance")
	}
	addrs := strings.Split(raw, ",")
	if len(addrs) != 3 {
		t.Fatal("three cluster addresses required")
	}
	client := redis.NewClusterClient(&redis.ClusterOptions{Addrs: addrs, ClusterSlots: func(context.Context) ([]redis.ClusterSlot, error) {
		return []redis.ClusterSlot{
			{Start: 0, End: 5460, Nodes: []redis.ClusterNode{{Addr: addrs[0]}}},
			{Start: 5461, End: 10922, Nodes: []redis.ClusterNode{{Addr: addrs[1]}}},
			{Start: 10923, End: 16383, Nodes: []redis.ClusterNode{{Addr: addrs[2]}}},
		}, nil
	}})
	t.Cleanup(func() {
		if err := client.Close(); err != nil {
			t.Error(err)
		}
	})
	b := NewBackendRedis(client, -1)
	ctx := context.Background()
	for name, reset := range map[string]func(...string) error{"task": b.ResetTask, "group": b.ResetGroup} {
		t.Run(name, func(t *testing.T) {
			keys := []string{"issue167:" + name + "{a}", "issue167:" + name + "{b}"}
			for _, key := range keys {
				if err := client.Set(ctx, key, "present", 0).Err(); err != nil {
					t.Fatal(err)
				}
			}
			t.Cleanup(func() {
				for _, key := range keys {
					if err := client.Del(ctx, key).Err(); err != nil {
						t.Error(err)
					}
				}
			})
			if err := reset(keys...); err != nil {
				t.Fatal(err)
			}
			for _, key := range keys {
				if n, err := client.Exists(ctx, key).Result(); err != nil || n != 0 {
					t.Fatalf("key remains: %d %v", n, err)
				}
			}
			if err := reset(keys...); err != nil {
				t.Fatalf("idempotent reset: %v", err)
			}
			if err := reset(); err != nil {
				t.Fatalf("empty reset: %v", err)
			}
		})
	}
}
