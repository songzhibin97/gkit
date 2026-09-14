package backend_redis

import (
	"fmt"
	"github.com/go-redis/redis/v8"
	"github.com/songzhibin97/gkit/distributed/task"
	"os"
	"sync"
	"testing"
)

func TestIssue167RuntimeRetentionRace(t *testing.T) {
	addr := os.Getenv("GKIT_REDIS_ADDR")
	if addr == "" {
		t.Skip("#167 live race regression requires GKIT_REDIS_ADDR; owned fixture required before acceptance")
	}
	client := redis.NewClient(&redis.Options{Addr: addr})
	t.Cleanup(func() {
		if err := client.Close(); err != nil {
			t.Error(err)
		}
	})
	b := NewBackendRedis(client, 60)
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 100; i++ {
			b.SetResultExpire(int64(60 + i%2))
		}
	}()
	defer wg.Wait()
	for i := 0; i < 20; i++ {
		sig := &task.Signature{ID: fmt.Sprintf("issue167-retention-%d", i)}
		if err := b.SetStatePending(sig); err != nil {
			t.Fatal(err)
		}
		status, err := b.GetStatus(sig.ID)
		if err != nil || status.Status != task.StatePending {
			t.Fatalf("pending status %v %v", status, err)
		}
		if err := b.ResetTask(sig.ID); err != nil {
			t.Fatal(err)
		}
	}
}
