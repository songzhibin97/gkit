package controller_redis

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"runtime"
	"sync"
	"time"

	json "github.com/json-iterator/go"

	"github.com/songzhibin97/gkit/distributed/broker"
	"github.com/songzhibin97/gkit/distributed/controller"
	"github.com/songzhibin97/gkit/distributed/task"
	"github.com/songzhibin97/gkit/log"

	"github.com/go-redis/redis/v8"
	"github.com/go-redsync/redsync/v4"
	"github.com/go-redsync/redsync/v4/redis/goredis/v8"
)

type ControllerRedis struct {
	*broker.Broker
	// client redis客户端
	client redis.UniversalClient
	// lock 分布式锁
	lock *redsync.Redsync

	// helper drives structured logs from the consumer / popDelayedTask
	// paths. Previously these printed errors via fmt.Println, which made
	// operational issues invisible to anyone consuming structured logs.
	helper *log.Helper

	// consumingWg 确保消费组并发完成
	consumingWg sync.WaitGroup
	// consumingQueue 消费队列名称
	consumingQueue string
	// delayedQueue  延迟队列名称
	delayedQueue string

	deliveryMu            sync.Mutex
	stopping              bool
	deliveryLease         time.Duration
	finalizationTimeout   time.Duration
	ackConfirmationWindow time.Duration
	// delayedRecoveryInterval paces the periodic delayed-transit recovery
	// scan started by produceDelayedTasks. It defaults to
	// delayedTransitRecoveryTimeout but is distinct from the Lua-side
	// staleness threshold (always delayedTransitRecoveryTimeout): shortening
	// the scan pace never makes transit entries stale sooner. Unexported on
	// purpose; only same-package tests inject shorter intervals.
	delayedRecoveryInterval time.Duration
	tokenSource             *deliveryTokenGenerator
}

const consumerRestoreTimeout = 5 * time.Second

type consumerAttemptErrors struct {
	mu   sync.Mutex
	errs []error
}

func (e *consumerAttemptErrors) add(err error) {
	if err == nil {
		return
	}
	e.mu.Lock()
	e.errs = append(e.errs, err)
	e.mu.Unlock()
}

func (e *consumerAttemptErrors) joined() error {
	e.mu.Lock()
	defer e.mu.Unlock()
	return errors.Join(e.errs...)
}

// redisOperationError preserves the underlying error for errors.Is/errors.As,
// but does not expose Redis connection addresses or credentials in Error().
type redisOperationError struct {
	operation string
	err       error
}

func (e *redisOperationError) Error() string {
	return e.operation + ": redis command failed"
}

func (e *redisOperationError) Unwrap() error {
	return e.err
}

func wrapRedisOperation(operation string, err error) error {
	if err == nil {
		return nil
	}
	return &redisOperationError{operation: operation, err: err}
}

// SetHelper installs the structured-log helper used for operational
// messages. If not called, the default helper backed by log.DefaultLogger
// is used.
func (c *ControllerRedis) SetHelper(h *log.Helper) {
	if h != nil {
		c.helper = h
	}
}

// SetConsumingQueue 设置消费队列名称
func (c *ControllerRedis) SetConsumingQueue(consumingQueue string) {
	c.consumingQueue = consumingQueue
}

// SetDelayedQueue 设置延迟队列名称
func (c *ControllerRedis) SetDelayedQueue(delayedQueue string) {
	c.delayedQueue = delayedQueue
}

func (c *ControllerRedis) RegisterTask(name ...string) {
	c.RegisterList(name...)
}

func (c *ControllerRedis) IsRegisterTask(name string) bool {
	return c.IsRegister(name)
}

func (c *ControllerRedis) StartConsuming(concurrency int, handler task.Processor) (bool, error) {
	c.consumingWg.Add(1)
	defer c.consumingWg.Done()

	// 设置阈值,如果并发数 < 1, 默认设置成 2*cpu
	if concurrency < 1 {
		concurrency = runtime.NumCPU() * 2
	}
	_, err := c.client.Ping(c.Broker.GetRetryCtx()).Result()
	if err != nil {
		if c.Broker.GetRetry() {
			if retryFn := c.Broker.GetRetryFn(); retryFn != nil {
				retryFn(c.Broker.GetRetryCtx())
			}
			return true, wrapRedisOperation("check queue connection", err)
		}
		return false, controller.ErrorConnectClose
	}

	attemptCtx, cancelAttempt := context.WithCancel(c.GetStopCtx())
	defer cancelAttempt()

	c.helper.Info("[*] Waiting for messages. To exit press CTRL+C")
	reliableQueue := newReliableQueue(c.client, c.consumingQueue, c.deliveryLease, c.tokenSource)
	handoff := make(chan *reliableDelivery)
	processorSlots := make(chan struct{}, concurrency)
	failures := &consumerAttemptErrors{}
	reportFailure := func(err error) {
		if err == nil {
			return
		}
		failures.add(err)
		cancelAttempt()
	}

	var producerWg sync.WaitGroup
	producerWg.Add(2)
	go func() {
		defer producerWg.Done()
		c.produceQueuedTasks(attemptCtx, c.consumingQueue, handoff, processorSlots, reportFailure)
	}()
	go func() {
		defer producerWg.Done()
		c.produceDelayedTasks(attemptCtx, c.delayedQueue, reportFailure)
	}()

	var processorWg sync.WaitGroup
	for {
		select {
		case delivery := <-handoff:
			if attemptCtx.Err() != nil {
				<-processorSlots
				if releaseErr := c.releaseReliableDelivery(reliableQueue, delivery); releaseErr != nil {
					reportFailure(releaseErr)
				}
				continue
			}
			processorWg.Add(1)
			go func(claimed *reliableDelivery) {
				defer processorWg.Done()
				defer func() { <-processorSlots }()
				if processErr := c.consumeReliableDelivery(attemptCtx, reliableQueue, claimed, handler); processErr != nil {
					reportFailure(processErr)
				}
			}(delivery)
		case <-attemptCtx.Done():
			cancelAttempt()
			producerWg.Wait()
			processorWg.Wait()
			if attemptErr := failures.joined(); attemptErr != nil {
				return c.GetRetry(), attemptErr
			}
			return c.GetRetry(), attemptCtx.Err()
		}
	}
}

// popTask 弹出任务
func (c *ControllerRedis) popTask(queue string, blockTime int64) ([]byte, error) {
	return c.popTaskWithContext(c.GetStopCtx(), queue, blockTime)
}

func (c *ControllerRedis) popTaskWithContext(ctx context.Context, queue string, blockTime int64) ([]byte, error) {
	if blockTime <= 0 {
		blockTime = int64(1000 * time.Millisecond)
	}
	items, err := c.client.BLPop(ctx, time.Duration(blockTime), queue).Result()
	if err != nil {
		return nil, err
	}
	// items[0] - the name of the key where an element was popped
	// items[1] - the value of the popped element
	if len(items) != 2 {
		return nil, redis.Nil
	}
	result := []byte(items[1])
	return result, nil
}

// popDelayedTask pulls the oldest task that is due (smallest ETA-unixnano
// score in [0, now], FIFO by due time). The claimed task is retained in the
// delayed transit zset until the caller finalizes or restores it, so a crash
// before the republish never loses it (see delayed_transit.go).
func (c *ControllerRedis) popDelayedTask(queue string, blockTime int64) ([]byte, error) {
	result, _, err := c.popDelayedTaskWithContext(c.GetStopCtx(), queue, blockTime)
	return result, err
}

func (c *ControllerRedis) popDelayedTaskWithContext(ctx context.Context, queue string, blockTime int64) ([]byte, float64, error) {
	if blockTime <= 0 {
		blockTime = int64(1000 * time.Millisecond)
	}
	for {
		timer := time.NewTimer(time.Duration(blockTime))
		select {
		case <-ctx.Done():
			if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
			return nil, 0, ctx.Err()
		case <-timer.C:
		}
		result, score, err := c.claimDelayedTask(ctx, queue)
		switch {
		case err == nil:
			return result, score, nil
		case errors.Is(err, redis.Nil):
			// No task ready; try again on the next tick.
			continue
		case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
			return nil, 0, err
		default:
			// Real error — propagate. Previously this was silently swallowed
			// via `break` + `return result, nil`, making caller's hot-loop
			// indistinguishable from "no task ready".
			return nil, 0, err
		}
	}
}

func (c *ControllerRedis) produceQueuedTasks(
	ctx context.Context,
	queue string,
	handoff chan<- *reliableDelivery,
	processorSlots chan struct{},
	reportFailure func(error),
) {
	reliableQueue := newReliableQueue(c.client, queue, c.deliveryLease, c.tokenSource)
	idleInterval := 25 * time.Millisecond
	emptyCount := uint64(0)
	for {
		select {
		case processorSlots <- struct{}{}:
		case <-ctx.Done():
			return
		}

		delivery, err := reliableQueue.claim(ctx)
		if err != nil {
			<-processorSlots
			switch {
			case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
				return
			default:
				reportFailure(err)
				return
			}
		}
		if delivery == nil {
			<-processorSlots
			emptyCount++
			if !waitReliablePoll(ctx, reliableIdleDelay(idleInterval, queue, emptyCount)) {
				return
			}
			if idleInterval < time.Second {
				idleInterval *= 2
				if idleInterval > time.Second {
					idleInterval = time.Second
				}
			}
			continue
		}
		emptyCount = 0
		idleInterval = 25 * time.Millisecond

		select {
		case handoff <- delivery:
		case <-ctx.Done():
			<-processorSlots
			if releaseErr := c.releaseReliableDelivery(reliableQueue, delivery); releaseErr != nil {
				reportFailure(releaseErr)
			}
			return
		}
	}
}

func (c *ControllerRedis) produceDelayedTasks(ctx context.Context, queue string, reportFailure func(error)) {
	// Republish transit entries stranded by a crashed producer before the
	// first claim, so a restart never waits a full recovery interval.
	if err := c.recoverDelayedTransit(ctx, queue); err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return
		}
		reportFailure(err)
		return
	}

	// Periodic recovery cannot live in the claim loop below:
	// popDelayedTaskWithContext polls internally while the delayed zset is
	// empty (redis.Nil -> retry), so the loop head is only reached again
	// after a successful claim. With a live producer and no due tasks,
	// entries stranded by a crashed peer would then never be recovered. A
	// dedicated ticker goroutine scans regardless of claim traffic; it is
	// joined before returning so StopConsuming's producer wait converges
	// without leaking a goroutine.
	recoveryCtx, cancelRecovery := context.WithCancel(ctx)
	recoveryDone := make(chan struct{})
	go func() {
		defer close(recoveryDone)
		ticker := time.NewTicker(c.delayedRecoveryInterval)
		defer ticker.Stop()
		for {
			select {
			case <-recoveryCtx.Done():
				return
			case <-ticker.C:
			}
			if err := c.recoverDelayedTransit(recoveryCtx, queue); err != nil {
				if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
					return
				}
				reportFailure(err)
				return
			}
		}
	}()
	defer func() {
		cancelRecovery()
		<-recoveryDone
	}()

	for {
		taskBody, score, err := c.popDelayedTaskWithContext(ctx, queue, 0)
		if err != nil {
			switch {
			case errors.Is(err, redis.Nil):
				continue
			case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
				return
			default:
				reportFailure(wrapRedisOperation("pop delayed task", err))
				return
			}
		}
		if len(taskBody) == 0 {
			continue
		}

		var signature task.Signature
		if err = json.Unmarshal(taskBody, &signature); err == nil {
			err = c.Publish(ctx, &signature)
		} else {
			err = fmt.Errorf("decode delayed task: %w", err)
		}
		if err == nil {
			if finalizeErr := c.finalizeDelayedTransit(queue, taskBody); finalizeErr != nil {
				// The task is already published; a leftover transit entry only
				// risks a duplicate republish after recovery, never a loss.
				reportFailure(finalizeErr)
				return
			}
			continue
		}

		restoreErr := c.restoreDelayedTask(queue, taskBody, score)
		if restoreErr != nil {
			reportFailure(errors.Join(err, restoreErr))
			return
		}
		if ctx.Err() != nil && (errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)) {
			return
		}
		reportFailure(fmt.Errorf("republish delayed task: %w", err))
		return
	}
}

func (c *ControllerRedis) consumeOne(taskBody []byte, queue string, handler task.Processor) error {
	t := task.Signature{}
	decoder := json.NewDecoder(bytes.NewReader(taskBody))
	decoder.UseNumber()
	if err := decoder.Decode(&t); err != nil {
		return fmt.Errorf("decode queued task: %w", err)
	}

	if !c.IsRegisterTask(t.Name) {
		if t.IgnoreNotRegisteredTask {
			return nil
		}
		return c.requeueUnregisteredTask(queue, taskBody)
	}
	if err := handler.Process(&t); err != nil {
		return fmt.Errorf("process task %q: %w", t.ID, err)
	}
	return nil
}

func (c *ControllerRedis) restorePendingTask(queue string, taskBody []byte) error {
	ctx, cancel := context.WithTimeout(context.Background(), consumerRestoreTimeout)
	defer cancel()
	if err := c.client.LPush(ctx, queue, taskBody).Err(); err != nil {
		return wrapRedisOperation("restore queued task", err)
	}
	return nil
}

func (c *ControllerRedis) requeueUnregisteredTask(queue string, taskBody []byte) error {
	ctx, cancel := context.WithTimeout(context.Background(), consumerRestoreTimeout)
	defer cancel()
	if err := c.client.RPush(ctx, queue, taskBody).Err(); err != nil {
		return wrapRedisOperation("requeue unregistered task", err)
	}
	return nil
}

func (c *ControllerRedis) restoreDelayedTask(queue string, taskBody []byte, score float64) error {
	ctx, cancel := context.WithTimeout(context.Background(), consumerRestoreTimeout)
	defer cancel()
	// Atomically restore the original ETA schedule and clear the transit
	// entry, so recovery cannot republish an already-restored task.
	err := delayedRestoreScript.Run(ctx, c.client, []string{queue, deriveDelayedTransitKey(queue)}, taskBody, score).Err()
	if err != nil {
		return wrapRedisOperation("restore delayed task", err)
	}
	return nil
}

func (c *ControllerRedis) StopConsuming() {
	c.deliveryMu.Lock()
	c.stopping = true
	c.deliveryMu.Unlock()
	c.Broker.StopConsuming()
	c.consumingWg.Wait()
}

func (c *ControllerRedis) Publish(ctx context.Context, t *task.Signature) error {
	tBody, err := json.Marshal(t)
	if err != nil {
		return fmt.Errorf("marshal task for publish: %w", err)
	}
	if t.ETA != nil {
		now := time.Now().Local()
		if t.ETA.After(now) {
			score := t.ETA.UnixNano()
			return wrapRedisOperation("publish delayed task", c.client.ZAdd(ctx, c.delayedQueue, &redis.Z{Score: float64(score), Member: tBody}).Err())
		}
	}
	return wrapRedisOperation("publish queued task", c.client.RPush(ctx, t.Router, tBody).Err())
}

func (c *ControllerRedis) GetPendingTasks(queue string) ([]*task.Signature, error) {
	results, err := c.client.LRange(c.GetStopCtx(), queue, 0, -1).Result()
	if err != nil {
		return nil, err
	}
	taskSlice := make([]*task.Signature, 0, len(results))
	for _, result := range results {
		var t task.Signature
		err = json.Unmarshal([]byte(result), &t)
		if err != nil {
			return nil, err
		}
		taskSlice = append(taskSlice, &t)
	}
	return taskSlice, nil
}

func (c *ControllerRedis) GetDelayedTasks() ([]*task.Signature, error) {
	results, err := c.client.ZRange(c.GetStopCtx(), c.delayedQueue, 0, -1).Result()
	if err != nil {
		return nil, err
	}
	taskSlice := make([]*task.Signature, 0, len(results))
	for _, result := range results {
		var t task.Signature
		err = json.Unmarshal([]byte(result), &t)
		if err != nil {
			return nil, err
		}
		taskSlice = append(taskSlice, &t)
	}
	return taskSlice, nil
}

// NewControllerRedis borrows client. The caller remains responsible for
// closing it after every component sharing the client has stopped using it.
func NewControllerRedis(broker *broker.Broker, client redis.UniversalClient, consumingQueue, delayedQueue string) controller.Controller {
	return &ControllerRedis{
		Broker:                  broker,
		client:                  client,
		lock:                    redsync.New(goredis.NewPool(client)),
		helper:                  log.NewHelper(log.DefaultLogger),
		consumingQueue:          consumingQueue,
		delayedQueue:            delayedQueue,
		deliveryLease:           defaultDeliveryLease,
		finalizationTimeout:     consumerRestoreTimeout,
		ackConfirmationWindow:   consumerRestoreTimeout,
		delayedRecoveryInterval: delayedTransitRecoveryTimeout,
		tokenSource:             newDeliveryTokenGenerator(nil),
	}
}
