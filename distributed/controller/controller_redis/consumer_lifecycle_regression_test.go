package controller_redis

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	json "github.com/json-iterator/go"

	"github.com/go-redis/redis/v8"
	"github.com/songzhibin97/gkit/distributed/broker"
	"github.com/songzhibin97/gkit/distributed/task"
)

type issue79ConsumeResult struct {
	retry bool
	err   error
}

type issue79JoiningProcessor struct {
	blockedStarted chan struct{}
	releaseBlocked chan struct{}
	failErr        error
	blockedOnce    sync.Once
}

func (p *issue79JoiningProcessor) Process(signature *task.Signature) error {
	switch signature.ID {
	case "blocked":
		p.blockedOnce.Do(func() { close(p.blockedStarted) })
		<-p.releaseBlocked
		return nil
	case "failed":
		<-p.blockedStarted
		return p.failErr
	default:
		return nil
	}
}

func (*issue79JoiningProcessor) ConsumeQueue() string    { return "test_task" }
func (*issue79JoiningProcessor) PreConsumeHandler() bool { return true }

type issue79BlockingRPushHook struct {
	started chan struct{}
	once    sync.Once
}

type issue79BlockingClaimHook struct {
	hash    string
	started chan struct{}
	release <-chan struct{}
	once    sync.Once
}

func (h *issue79BlockingClaimHook) BeforeProcess(ctx context.Context, cmd redis.Cmder) (context.Context, error) {
	if strings.EqualFold(cmd.Name(), "evalsha") {
		args := cmd.Args()
		if len(args) >= 2 && fmt.Sprint(args[1]) == h.hash {
			h.once.Do(func() { close(h.started) })
			<-h.release
		}
	}
	return ctx, nil
}

func (*issue79BlockingClaimHook) AfterProcess(context.Context, redis.Cmder) error { return nil }

func (*issue79BlockingClaimHook) BeforeProcessPipeline(ctx context.Context, _ []redis.Cmder) (context.Context, error) {
	return ctx, nil
}

func (*issue79BlockingClaimHook) AfterProcessPipeline(context.Context, []redis.Cmder) error {
	return nil
}

type issue79TrackingRedisClient struct {
	redis.UniversalClient
	active         atomic.Int32
	popCalls       chan int32
	calls          atomic.Int32
	blockAfterCall int32
	popUnblocked   chan struct{}
	releasePop     <-chan struct{}
	unblockedOnce  sync.Once
}

func (c *issue79TrackingRedisClient) BLPop(ctx context.Context, timeout time.Duration, keys ...string) *redis.StringSliceCmd {
	call := c.calls.Add(1)
	c.active.Add(1)
	defer c.active.Add(-1)
	c.popCalls <- call
	result := c.UniversalClient.BLPop(ctx, timeout, keys...)
	if call == c.blockAfterCall {
		c.unblockedOnce.Do(func() { close(c.popUnblocked) })
		<-c.releasePop
	}
	return result
}

type issue79WaitForPopProcessor struct {
	popCalls <-chan int32
	wantCall int32
	failErr  error
	abort    <-chan struct{}
}

func (p *issue79WaitForPopProcessor) Process(*task.Signature) error {
	for {
		select {
		case call := <-p.popCalls:
			if call >= p.wantCall {
				return p.failErr
			}
		case <-p.abort:
			return p.failErr
		}
	}
}

func (*issue79WaitForPopProcessor) ConsumeQueue() string    { return "test_task" }
func (*issue79WaitForPopProcessor) PreConsumeHandler() bool { return true }

func (h *issue79BlockingRPushHook) BeforeProcess(ctx context.Context, cmd redis.Cmder) (context.Context, error) {
	if strings.EqualFold(cmd.Name(), "rpush") {
		h.once.Do(func() { close(h.started) })
		<-ctx.Done()
		return ctx, ctx.Err()
	}
	return ctx, nil
}

func (*issue79BlockingRPushHook) AfterProcess(context.Context, redis.Cmder) error {
	return nil
}

func (*issue79BlockingRPushHook) BeforeProcessPipeline(ctx context.Context, _ []redis.Cmder) (context.Context, error) {
	return ctx, nil
}

func (*issue79BlockingRPushHook) AfterProcessPipeline(context.Context, []redis.Cmder) error {
	return nil
}

func TestPublishHonorsSuppliedContext(t *testing.T) {
	c, _, client := newMiniController(t)

	t.Run("queued", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		signature := task.NewSignature("cancelled", "task")
		signature.Router = "test_task"
		err := c.Publish(ctx, signature)
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("Publish error = %v, want context.Canceled", err)
		}
		if length := client.LLen(context.Background(), "test_task").Val(); length != 0 {
			t.Fatalf("queue length = %d, want 0", length)
		}
	})

	t.Run("delayed", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		eta := time.Now().Add(time.Hour)
		signature := task.NewSignature("cancelled-delayed", "task", task.SetETATime(&eta))
		err := c.Publish(ctx, signature)
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("Publish error = %v, want context.Canceled", err)
		}
		if length := client.ZCard(context.Background(), "delayed").Val(); length != 0 {
			t.Fatalf("delayed queue length = %d, want 0", length)
		}
	})
}

func TestRetryPingFailureRedactsConnectionDetails(t *testing.T) {
	client := newClosedRedisClient(t)
	b := broker.NewBroker(
		broker.NewRegisteredTask(),
		context.Background(),
		broker.SetRetry(true),
		broker.SetRetryFn(func(context.Context) {}),
	)
	c := NewControllerRedis(b, client, "test_task", "delayed").(*ControllerRedis)

	retry, err := c.StartConsuming(1, issue79Processor{})
	if !retry {
		t.Fatal("retry = false, want true")
	}
	if err == nil {
		t.Fatal("StartConsuming error = nil, want connection failure")
	}
	if got := err.Error(); got != "check queue connection: redis command failed" {
		t.Fatalf("StartConsuming error = %q, want operation context without connection details", got)
	}
}

func TestQueueProducerRestoresPoppedTaskWhenHandoffIsCancelled(t *testing.T) {
	c, _, client := newMiniController(t)
	body := []byte(`{"id":"restore-me","name":"task"}`)
	if err := client.RPush(context.Background(), "test_task", body).Err(); err != nil {
		t.Fatalf("enqueue task: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	handoff := make(chan *reliableDelivery)
	processorSlots := make(chan struct{}, 1)
	failures := make(chan error, 1)
	done := make(chan struct{})
	go func() {
		defer close(done)
		c.produceQueuedTasks(ctx, "test_task", handoff, processorSlots, func(err error) {
			select {
			case failures <- err:
			default:
			}
		})
	}()

	deadline := time.Now().Add(3 * time.Second)
	for client.LLen(context.Background(), "test_task").Val() != 0 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if length := client.LLen(context.Background(), "test_task").Val(); length != 0 {
		cancel()
		t.Fatal("producer did not pop the task")
	}

	cancel()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("queue producer did not stop after cancellation")
	}
	select {
	case err := <-failures:
		t.Fatalf("queue producer reported restore failure: %v", err)
	default:
	}
	if length := client.LLen(context.Background(), "test_task").Val(); length != 1 {
		t.Fatalf("queue length = %d, want 1 restored task", length)
	}
	if restored := client.LIndex(context.Background(), "test_task", 0).Val(); restored != string(body) {
		t.Fatalf("restored task = %q, want %q", restored, body)
	}
	if slots := len(processorSlots); slots != 0 {
		t.Fatalf("processor slots still owned = %d, want 0", slots)
	}
}

func TestConsumeOnePreservesUnregisteredRequeueOrder(t *testing.T) {
	c, _, client := newMiniController(t)
	existing := []byte(`{"id":"existing","name":"task"}`)
	unregistered := []byte(`{"id":"unregistered","name":"missing"}`)
	if err := client.RPush(context.Background(), "test_task", existing).Err(); err != nil {
		t.Fatalf("enqueue existing task: %v", err)
	}
	if err := c.consumeOne(unregistered, "test_task", issue79Processor{}); err != nil {
		t.Fatalf("consume unregistered task: %v", err)
	}
	queued, err := client.LRange(context.Background(), "test_task", 0, -1).Result()
	if err != nil {
		t.Fatalf("read queue: %v", err)
	}
	if len(queued) != 2 || queued[0] != string(existing) || queued[1] != string(unregistered) {
		t.Fatalf("queue = %q, want existing task followed by unregistered task", queued)
	}
}

func TestQueueProducerSurfacesRestoreFailureWithoutClientDetails(t *testing.T) {
	c, _, client := newMiniController(t)
	body := []byte(`{"id":"restore-fails","name":"task"}`)
	if err := client.RPush(context.Background(), "test_task", body).Err(); err != nil {
		t.Fatalf("enqueue task: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	handoff := make(chan *reliableDelivery)
	processorSlots := make(chan struct{}, 1)
	failures := make(chan error, 1)
	done := make(chan struct{})
	go func() {
		defer close(done)
		c.produceQueuedTasks(ctx, "test_task", handoff, processorSlots, func(err error) {
			failures <- err
		})
	}()

	deadline := time.Now().Add(3 * time.Second)
	for client.LLen(context.Background(), "test_task").Val() != 0 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if length := client.LLen(context.Background(), "test_task").Val(); length != 0 {
		cancel()
		t.Fatal("producer did not pop the task")
	}
	if err := client.Close(); err != nil {
		cancel()
		t.Fatalf("close client: %v", err)
	}
	cancel()

	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("queue producer did not stop after cancellation")
	}
	select {
	case err := <-failures:
		if got := err.Error(); got != "release queued task: redis command failed" {
			t.Fatalf("restore error = %q, want operation context without client detail", got)
		}
		if !errors.Is(err, redis.ErrClosed) {
			t.Fatalf("restore error does not preserve redis.ErrClosed: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("restore failure was not surfaced")
	}
}

func TestStartConsumingCancelsAndJoinsAttemptBeforeReturn(t *testing.T) {
	c, _, client := newMiniController(t)
	defer c.StopConsuming()
	c.RegisterTask("task")

	failErr := errors.New("processor failed")
	processor := &issue79JoiningProcessor{
		blockedStarted: make(chan struct{}),
		releaseBlocked: make(chan struct{}),
		failErr:        failErr,
	}
	for _, id := range []string{"failed", "blocked"} {
		body, err := json.Marshal(task.NewSignature(id, "task"))
		if err != nil {
			t.Fatalf("marshal %s: %v", id, err)
		}
		if err := client.RPush(context.Background(), "test_task", body).Err(); err != nil {
			t.Fatalf("enqueue %s: %v", id, err)
		}
	}

	result := make(chan issue79ConsumeResult, 1)
	go func() {
		retry, err := c.StartConsuming(2, processor)
		result <- issue79ConsumeResult{retry: retry, err: err}
	}()

	select {
	case <-processor.blockedStarted:
	case <-time.After(3 * time.Second):
		t.Fatal("blocked processor did not start")
	}

	select {
	case early := <-result:
		close(processor.releaseBlocked)
		t.Fatalf("StartConsuming returned before the active processor joined: retry=%v err=%v", early.retry, early.err)
	case <-time.After(100 * time.Millisecond):
	}

	close(processor.releaseBlocked)
	var outcome issue79ConsumeResult
	select {
	case outcome = <-result:
	case <-time.After(3 * time.Second):
		t.Fatal("StartConsuming did not return after active processor completed")
	}
	if !errors.Is(outcome.err, failErr) {
		t.Fatalf("StartConsuming error = %v, want processor failure", outcome.err)
	}

	afterBody, err := json.Marshal(task.NewSignature("after-return", "task"))
	if err != nil {
		t.Fatalf("marshal post-return task: %v", err)
	}
	if err := client.RPush(context.Background(), "test_task", afterBody).Err(); err != nil {
		t.Fatalf("enqueue post-return task: %v", err)
	}
	time.Sleep(100 * time.Millisecond)
	if length := client.LLen(context.Background(), "test_task").Val(); length != 1 {
		t.Fatalf("queue length after StartConsuming returned = %d, want 1", length)
	}
}

func TestRetryAttemptsDoNotAccumulateQueueProducers(t *testing.T) {
	c, _, client := newMiniController(t)
	c.Broker.SetRetry(true)
	defer c.StopConsuming()
	c.RegisterTask("task")

	for attempt := int32(1); attempt <= 2; attempt++ {
		body, err := json.Marshal(task.NewSignature(fmt.Sprintf("attempt-%d", attempt), "task"))
		if err != nil {
			t.Fatalf("marshal attempt %d: %v", attempt, err)
		}
		if err := client.RPush(context.Background(), "test_task", body).Err(); err != nil {
			t.Fatalf("enqueue attempt %d: %v", attempt, err)
		}

		failErr := fmt.Errorf("attempt %d failed", attempt)
		processor := &reliableTestProcessor{
			started: make(chan struct{}),
			err:     failErr,
		}
		result := make(chan issue79ConsumeResult, 1)
		go func() {
			retry, err := c.StartConsuming(2, processor)
			result <- issue79ConsumeResult{retry: retry, err: err}
		}()

		select {
		case outcome := <-result:
			if !outcome.retry {
				t.Fatalf("attempt %d retry = false, want true", attempt)
			}
			if !errors.Is(outcome.err, failErr) {
				t.Fatalf("attempt %d error = %v, want processor failure", attempt, outcome.err)
			}
		case <-time.After(3 * time.Second):
			t.Fatalf("attempt %d did not return", attempt)
		}
		probe := fmt.Sprintf(`{"id":"probe-%d","name":"task"}`, attempt)
		if err := client.RPush(context.Background(), "test_task", probe).Err(); err != nil {
			t.Fatalf("enqueue probe %d: %v", attempt, err)
		}
		time.Sleep(100 * time.Millisecond)
		if got := client.LIndex(context.Background(), "test_task", 0).Val(); got != probe {
			t.Fatalf("queue producer survived attempt %d and claimed probe", attempt)
		}
		if err := client.LPop(context.Background(), "test_task").Err(); err != nil {
			t.Fatalf("remove probe %d: %v", attempt, err)
		}
	}
}

func TestStartConsumingJoinsQueueProducerBeforeReturn(t *testing.T) {
	c, _, client := newMiniController(t)
	defer c.StopConsuming()
	c.RegisterTask("task")
	if err := reliableClaimScript.Load(context.Background(), client).Err(); err != nil {
		t.Fatalf("preload claim script: %v", err)
	}
	releaseClaim := make(chan struct{})
	hook := &issue79BlockingClaimHook{
		hash:    reliableClaimScript.Hash(),
		started: make(chan struct{}),
		release: releaseClaim,
	}
	client.AddHook(hook)
	result := make(chan issue79ConsumeResult, 1)
	go func() {
		retry, err := c.StartConsuming(2, issue79Processor{})
		result <- issue79ConsumeResult{retry: retry, err: err}
	}()

	select {
	case <-hook.started:
	case <-time.After(3 * time.Second):
		close(releaseClaim)
		t.Fatal("queue producer did not reach its cancellation gate")
	}
	c.Broker.StopConsuming()
	select {
	case early := <-result:
		close(releaseClaim)
		t.Fatalf("StartConsuming returned before queue producer joined: retry=%v err=%v", early.retry, early.err)
	case <-time.After(100 * time.Millisecond):
	}
	close(releaseClaim)
	select {
	case outcome := <-result:
		if !errors.Is(outcome.err, context.Canceled) {
			t.Fatalf("StartConsuming error = %v, want context.Canceled", outcome.err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("StartConsuming did not return after queue producer joined")
	}
}

func TestCancelledDelayedRepublishRestoresOriginalSchedule(t *testing.T) {
	c, _, client := newMiniController(t)
	defer c.StopConsuming()
	c.RegisterTask("task")

	due := time.Now().Add(-time.Minute)
	signature := task.NewSignature("delayed", "task", task.SetETATime(&due))
	signature.Router = "test_task"
	body, err := json.Marshal(signature)
	if err != nil {
		t.Fatalf("marshal delayed task: %v", err)
	}
	originalScore := float64(due.UnixNano())
	if err := client.ZAdd(context.Background(), "delayed", &redis.Z{Score: originalScore, Member: body}).Err(); err != nil {
		t.Fatalf("enqueue delayed task: %v", err)
	}

	hook := &issue79BlockingRPushHook{started: make(chan struct{})}
	client.AddHook(hook)
	result := make(chan issue79ConsumeResult, 1)
	go func() {
		retry, err := c.StartConsuming(1, issue79Processor{})
		result <- issue79ConsumeResult{retry: retry, err: err}
	}()

	select {
	case <-hook.started:
	case <-time.After(3 * time.Second):
		c.Broker.StopConsuming()
		t.Fatal("delayed republish did not start")
	}
	c.Broker.StopConsuming()
	select {
	case <-result:
	case <-time.After(3 * time.Second):
		t.Fatal("StartConsuming did not join the cancelled delayed producer")
	}

	deadline := time.Now().Add(time.Second)
	for client.ZCard(context.Background(), "delayed").Val() != 1 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if cardinality := client.ZCard(context.Background(), "delayed").Val(); cardinality != 1 {
		t.Fatalf("delayed queue cardinality = %d, want 1 restored task", cardinality)
	}
	if score, err := client.ZScore(context.Background(), "delayed", string(body)).Result(); err != nil {
		t.Fatalf("restored task score: %v", err)
	} else if score != originalScore {
		t.Fatalf("restored score = %.0f, want %.0f", score, originalScore)
	}
	// The restore must also clear the transit claim, or recovery would later
	// republish the restored task a second time.
	if transit := client.ZCard(context.Background(), deriveDelayedTransitKey("delayed")).Val(); transit != 0 {
		t.Fatalf("transit zset still holds %d entries after restore, want 0", transit)
	}
}
