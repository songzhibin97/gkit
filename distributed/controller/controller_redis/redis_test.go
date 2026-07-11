package controller_redis

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/songzhibin97/gkit/distributed/task"

	"github.com/go-redis/redis/v8"
	"github.com/songzhibin97/gkit/distributed/broker"
	"github.com/songzhibin97/gkit/distributed/controller"
)

func initLiveController(t *testing.T) controller.Controller {
	t.Helper()

	opt := redis.UniversalOptions{
		Addrs: []string{"127.0.0.1:6379"},
	}
	client := redis.NewUniversalClient(&opt)
	if client == nil {
		return nil
	}
	// Without a live Redis the client is created lazily; skip this integration
	// test. Ping-failure behavior is covered deterministically by regression_test.go,
	// while FIFO and watch-error behavior is covered by fifo_test.go.
	if err := client.Ping(context.Background()).Err(); err != nil {
		_ = client.Close()
		return nil
	}
	queue := fmt.Sprintf("gkit:test:controller:%d", time.Now().UnixNano())
	delayedQueue := queue + ":delayed"
	reliableKeys := deriveReliableQueueKeys(queue)
	producerCtx, cancelProducer := context.WithCancel(context.Background())
	producerDone := make(chan struct{})
	t.Cleanup(func() {
		cancelProducer()
		<-producerDone

		cleanupCtx, cancelCleanup := context.WithTimeout(context.Background(), time.Second)
		defer cancelCleanup()
		if err := client.Del(cleanupCtx, queue, delayedQueue, reliableKeys.inflight,
			reliableKeys.visibility, reliableKeys.outcomes, reliableKeys.repairCursor, reliableKeys.repairBacklog).Err(); err != nil {
			t.Errorf("clean live Redis queues: %v", err)
		}
		if err := client.Close(); err != nil {
			t.Errorf("close live Redis client: %v", err)
		}
	})

	bk := broker.NewBroker(broker.NewRegisteredTask(), context.Background())
	go func() {
		defer close(producerDone)
		ticker := time.NewTicker(10 * time.Millisecond)
		defer ticker.Stop()

		n := 0
		for {
			select {
			case <-producerCtx.Done():
				return
			case <-ticker.C:
			}
			if err := client.LPush(producerCtx, queue, fmt.Sprintf(`{"id":"%d","name":"test_task"}`, n)).Err(); err != nil {
				if producerCtx.Err() != nil {
					return
				}
				t.Errorf("produce live Redis task: %v", err)
				return
			}
			n++
		}
	}()
	return NewControllerRedis(bk, client, queue, delayedQueue)
}

type processor struct {
	processed chan<- struct{}
}

func (p processor) Process(t *task.Signature) error {
	select {
	case p.processed <- struct{}{}:
	default:
	}
	return nil
}

func (p processor) ConsumeQueue() string {
	return "test_task"
}

func (p processor) PreConsumeHandler() bool {
	return false
}

func TestControllerRedis_StartConsuming(t *testing.T) {
	ct := initLiveController(t)
	if ct == nil {
		t.Skip("no live Redis at 127.0.0.1:6379")
	}
	t.Cleanup(ct.StopConsuming)
	ct.RegisterTask("test_task")
	processed := make(chan struct{}, 1)
	type consumeResult struct {
		retry bool
		err   error
	}
	result := make(chan consumeResult, 1)
	go func() {
		retry, err := ct.StartConsuming(1, processor{processed: processed})
		result <- consumeResult{retry: retry, err: err}
	}()
	select {
	case <-processed:
	case <-time.After(3 * time.Second):
		t.Fatal("live Redis task was not consumed")
	}
	ct.StopConsuming()
	outcome := <-result
	if !errors.Is(outcome.err, context.Canceled) {
		t.Fatalf("StartConsuming error = %v, want context.Canceled after StopConsuming", outcome.err)
	}
	if outcome.retry {
		t.Fatal("StartConsuming retry = true after an intentional stop, want false")
	}
}
