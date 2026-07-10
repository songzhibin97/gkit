package controller_redis

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/go-redis/redis/v8"

	"github.com/songzhibin97/gkit/distributed/broker"
	"github.com/songzhibin97/gkit/distributed/controller"
	"github.com/songzhibin97/gkit/distributed/task"
)

type issue79Processor struct{}

func (issue79Processor) Process(*task.Signature) error { return nil }
func (issue79Processor) ConsumeQueue() string          { return "consume_queue" }
func (issue79Processor) PreConsumeHandler() bool       { return true }

func newClosedRedisClient(t *testing.T) redis.UniversalClient {
	t.Helper()
	mr := miniredis.RunT(t)
	addr := mr.Addr()
	mr.Close()
	client := redis.NewUniversalClient(&redis.UniversalOptions{
		Addrs:        []string{addr},
		MaxRetries:   -1,
		DialTimeout:  50 * time.Millisecond,
		ReadTimeout:  50 * time.Millisecond,
		WriteTimeout: 50 * time.Millisecond,
	})
	t.Cleanup(func() { _ = client.Close() })
	return client
}

func TestGetDelayedTasksReturnsETAOrder(t *testing.T) {
	c, _, _ := newMiniController(t)
	now := time.Now()
	earlyETA := now.Add(time.Hour)
	lateETA := now.Add(2 * time.Hour)

	late := task.NewSignature("late", "task", task.SetETATime(&lateETA))
	early := task.NewSignature("early", "task", task.SetETATime(&earlyETA))
	if err := c.Publish(context.Background(), late); err != nil {
		t.Fatalf("publish late task: %v", err)
	}
	if err := c.Publish(context.Background(), early); err != nil {
		t.Fatalf("publish early task: %v", err)
	}

	tasks, err := c.GetDelayedTasks()
	if err != nil {
		t.Fatalf("GetDelayedTasks returned error: %v", err)
	}
	if len(tasks) != 2 {
		t.Fatalf("GetDelayedTasks returned %d tasks, want 2", len(tasks))
	}
	if tasks[0].ID != "early" || tasks[1].ID != "late" {
		t.Fatalf("task order = [%s %s], want [early late]", tasks[0].ID, tasks[1].ID)
	}
}

func TestStartConsumingPingFailureWithoutRetryDoesNotPanic(t *testing.T) {
	client := newClosedRedisClient(t)
	b := broker.NewBroker(broker.NewRegisteredTask(), context.Background())
	c := NewControllerRedis(b, client, "consume_queue", "delayed_queue")

	retryAllowed, err := c.StartConsuming(1, issue79Processor{})
	if retryAllowed {
		t.Fatal("retryAllowed = true, want false")
	}
	if !errors.Is(err, controller.ErrorConnectClose) {
		t.Fatalf("error = %v, want ErrorConnectClose", err)
	}
}

func TestStartConsumingPingFailureWithRetryCallsRetryFnOnce(t *testing.T) {
	client := newClosedRedisClient(t)
	var calls atomic.Int32
	b := broker.NewBroker(
		broker.NewRegisteredTask(),
		context.Background(),
		broker.SetRetry(true),
		broker.SetRetryFn(func(context.Context) { calls.Add(1) }),
	)
	c := NewControllerRedis(b, client, "consume_queue", "delayed_queue")

	retryAllowed, err := c.StartConsuming(1, issue79Processor{})
	if !retryAllowed {
		t.Fatal("retryAllowed = false, want true")
	}
	if err == nil {
		t.Fatal("error = nil, want Ping error")
	}
	if got := calls.Load(); got != 1 {
		t.Fatalf("retry callback calls = %d, want 1", got)
	}
}
