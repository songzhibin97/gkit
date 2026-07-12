package controller_redis

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/go-redis/redis/v8"
	json "github.com/json-iterator/go"

	"github.com/songzhibin97/gkit/distributed/broker"
	"github.com/songzhibin97/gkit/distributed/task"
)

type reliableTestProcessor struct {
	started  chan struct{}
	release  chan struct{}
	returned chan struct{}
	err      error
	once     sync.Once
	calls    atomic.Int32
}

func (p *reliableTestProcessor) Process(*task.Signature) error {
	p.calls.Add(1)
	p.once.Do(func() { close(p.started) })
	if p.release != nil {
		<-p.release
	}
	if p.returned != nil {
		close(p.returned)
	}
	return p.err
}

func (*reliableTestProcessor) ConsumeQueue() string    { return "task" }
func (*reliableTestProcessor) PreConsumeHandler() bool { return true }

type reliableConsumeResult struct {
	retry bool
	err   error
}

type claimTimingHook struct {
	hash  string
	times chan time.Time
}

func (h *claimTimingHook) BeforeProcess(ctx context.Context, _ redis.Cmder) (context.Context, error) {
	return ctx, nil
}

func (h *claimTimingHook) AfterProcess(_ context.Context, cmd redis.Cmder) error {
	if !strings.EqualFold(cmd.Name(), "evalsha") {
		return nil
	}
	args := cmd.Args()
	if len(args) >= 2 && fmt.Sprint(args[1]) == h.hash {
		select {
		case h.times <- time.Now():
		default:
		}
	}
	return nil
}

func (*claimTimingHook) BeforeProcessPipeline(ctx context.Context, _ []redis.Cmder) (context.Context, error) {
	return ctx, nil
}

func (*claimTimingHook) AfterProcessPipeline(context.Context, []redis.Cmder) error { return nil }

func newReliableTestController(t *testing.T, queue string) (*ControllerRedis, redis.UniversalClient) {
	t.Helper()
	mr := miniredis.RunT(t)
	client := redis.NewUniversalClient(&redis.UniversalOptions{Addrs: []string{mr.Addr()}})
	t.Cleanup(func() { _ = client.Close() })
	b := broker.NewBroker(broker.NewRegisteredTask(), context.Background())
	c := NewControllerRedis(b, client, queue, queue+":delayed").(*ControllerRedis)
	c.RegisterTask("task")
	return c, client
}

func reliableTaggedPrefix(queue, tag string) string {
	digest := sha256.Sum256([]byte(queue))
	return fmt.Sprintf("{%s}:gkit:%x", tag, digest)
}

func reliableTaskBody(t *testing.T, id string) []byte {
	t.Helper()
	signature := task.NewSignature(id, "task")
	signature.Router = "queue:{reliable}"
	body, err := json.Marshal(signature)
	if err != nil {
		t.Fatalf("marshal task: %v", err)
	}
	return body
}

func TestClaimPersistsUntilSuccessfulAck(t *testing.T) {
	const queue = "queue:{reliable}"
	c, client := newReliableTestController(t, queue)
	body := reliableTaskBody(t, "persistent-claim")
	if err := client.RPush(context.Background(), queue, body).Err(); err != nil {
		t.Fatalf("enqueue task: %v", err)
	}
	processor := &reliableTestProcessor{started: make(chan struct{}), release: make(chan struct{})}
	result := make(chan reliableConsumeResult, 1)
	go func() {
		retry, err := c.StartConsuming(1, processor)
		result <- reliableConsumeResult{retry: retry, err: err}
	}()
	select {
	case <-processor.started:
	case <-time.After(3 * time.Second):
		c.StopConsuming()
		t.Fatal("processor did not start")
	}

	prefix := reliableTaggedPrefix(queue, "reliable")
	if got := client.HLen(context.Background(), prefix+":inflight").Val(); got != 1 {
		close(processor.release)
		c.StopConsuming()
		t.Fatalf("inflight reservations while processing = %d, want 1", got)
	}
	if got := client.ZCard(context.Background(), prefix+":ack-outcomes").Val(); got != 0 {
		close(processor.release)
		c.StopConsuming()
		t.Fatalf("ack outcomes before processor success = %d, want 0", got)
	}
	close(processor.release)
	deadline := time.Now().Add(3 * time.Second)
	for client.ZCard(context.Background(), prefix+":ack-outcomes").Val() != 1 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if got := client.ZCard(context.Background(), prefix+":ack-outcomes").Val(); got != 1 {
		c.StopConsuming()
		t.Fatalf("ack outcomes after processor success = %d, want 1", got)
	}
	c.StopConsuming()
	select {
	case <-result:
	case <-time.After(3 * time.Second):
		t.Fatal("consumer did not stop")
	}
}

func TestAckOccursOnlyAfterProcessorSuccess(t *testing.T) {
	t.Run("ignored unregistered task is acknowledged", func(t *testing.T) {
		const queue = "queue:{ignore-unregistered}"
		c, client := newReliableTestController(t, queue)
		signature := task.NewSignature("ignored", "missing")
		signature.Router = queue
		signature.IgnoreNotRegisteredTask = true
		body, err := json.Marshal(signature)
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		if err := client.RPush(context.Background(), queue, body).Err(); err != nil {
			t.Fatalf("enqueue: %v", err)
		}
		processor := &reliableTestProcessor{started: make(chan struct{})}
		result := make(chan reliableConsumeResult, 1)
		go func() {
			retry, err := c.StartConsuming(1, processor)
			result <- reliableConsumeResult{retry: retry, err: err}
		}()
		keys := deriveReliableQueueKeys(queue)
		deadline := time.Now().Add(3 * time.Second)
		for client.ZCard(context.Background(), keys.outcomes).Val() != 1 && time.Now().Before(deadline) {
			time.Sleep(time.Millisecond)
		}
		if got := client.ZCard(context.Background(), keys.outcomes).Val(); got != 1 {
			c.StopConsuming()
			t.Fatalf("ack outcomes = %d, want 1", got)
		}
		select {
		case <-processor.started:
			c.StopConsuming()
			t.Fatal("processor ran for ignored unregistered task")
		default:
		}
		c.StopConsuming()
		<-result
	})

	t.Run("non-ignored unregistered task is deferred", func(t *testing.T) {
		const queue = "queue:{defer-unregistered}"
		c, client := newReliableTestController(t, queue)
		signature := task.NewSignature("not-ignored", "missing")
		signature.Router = queue
		body, err := json.Marshal(signature)
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		if err := client.RPush(context.Background(), queue, body).Err(); err != nil {
			t.Fatalf("enqueue: %v", err)
		}
		processor := &reliableTestProcessor{started: make(chan struct{})}
		_, err = c.StartConsuming(1, processor)
		if err == nil || !strings.Contains(err.Error(), "not registered") {
			t.Fatalf("StartConsuming error = %v, want not-registered failure", err)
		}
		keys := deriveReliableQueueKeys(queue)
		if got := client.HLen(context.Background(), keys.inflight).Val(); got != 1 {
			t.Fatalf("deferred inflight = %d, want 1", got)
		}
		if got := client.ZCard(context.Background(), keys.outcomes).Val(); got != 0 {
			t.Fatalf("ack outcomes = %d, want 0", got)
		}
	})
}

func TestActiveProcessorRenewsWithinConfirmedDeadline(t *testing.T) {
	const queue = "queue:{renew-active}"
	c, client := newReliableTestController(t, queue)
	c.deliveryLease = 80 * time.Millisecond
	if err := reliableRenewScript.Load(context.Background(), client).Err(); err != nil {
		t.Fatalf("preload renew script: %v", err)
	}
	lostRenew := &loseScriptResponseHook{hash: reliableRenewScript.Hash(), err: errors.New("renew response lost")}
	client.AddHook(lostRenew)
	lostRenew.armed.Store(true)
	body := reliableTaskBody(t, "long-running")
	if err := client.RPush(context.Background(), queue, body).Err(); err != nil {
		t.Fatalf("enqueue: %v", err)
	}
	processor := &reliableTestProcessor{started: make(chan struct{}), release: make(chan struct{})}
	result := make(chan reliableConsumeResult, 1)
	go func() {
		retry, err := c.StartConsuming(1, processor)
		result <- reliableConsumeResult{retry: retry, err: err}
	}()
	select {
	case <-processor.started:
	case <-time.After(3 * time.Second):
		c.StopConsuming()
		t.Fatal("processor did not start")
	}
	time.Sleep(250 * time.Millisecond)
	if lostRenew.armed.Load() {
		close(processor.release)
		c.StopConsuming()
		t.Fatal("renewal transport failure was not injected")
	}
	keys := deriveReliableQueueKeys(queue)
	if got := client.HLen(context.Background(), keys.inflight).Val(); got != 1 {
		close(processor.release)
		c.StopConsuming()
		t.Fatalf("active inflight after initial lease = %d, want 1", got)
	}
	probe := newReliableQueue(client, queue, c.deliveryLease, newDeliveryTokenGenerator(nil))
	if reclaimed, err := probe.claim(context.Background()); err != nil || reclaimed != nil {
		close(processor.release)
		c.StopConsuming()
		t.Fatalf("healthy delivery reclaimed = (%v, %v)", reclaimed, err)
	}
	close(processor.release)
	deadline := time.Now().Add(3 * time.Second)
	for client.HLen(context.Background(), keys.inflight).Val() != 0 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	c.StopConsuming()
	<-result
}

func TestStopJoinsHeartbeatAndOwnedDeliveries(t *testing.T) {
	const queue = "queue:{stop-join}"
	c, client := newReliableTestController(t, queue)
	c.deliveryLease = 40 * time.Millisecond
	if err := reliableRenewScript.Load(context.Background(), client).Err(); err != nil {
		t.Fatalf("preload renew script: %v", err)
	}
	if err := reliableAckScript.Load(context.Background(), client).Err(); err != nil {
		t.Fatalf("preload ack script: %v", err)
	}
	renewEntered := make(chan struct{})
	renewCanceled := make(chan struct{})
	renewRelease := make(chan struct{})
	var renewReleaseOnce sync.Once
	releaseRenew := func() { renewReleaseOnce.Do(func() { close(renewRelease) }) }
	t.Cleanup(releaseRenew)
	renewHook := &beforeScriptHook{
		hash:     reliableRenewScript.Hash(),
		entered:  renewEntered,
		canceled: renewCanceled,
		release:  renewRelease,
	}
	renewHook.armed.Store(true)
	ackEntered := make(chan struct{})
	ackHook := &beforeScriptHook{hash: reliableAckScript.Hash(), entered: ackEntered}
	ackHook.armed.Store(true)
	client.AddHook(renewHook)
	client.AddHook(ackHook)

	body := reliableTaskBody(t, "stop-active")
	if err := client.RPush(context.Background(), queue, body).Err(); err != nil {
		t.Fatalf("enqueue: %v", err)
	}
	processor := &reliableTestProcessor{
		started:  make(chan struct{}),
		release:  make(chan struct{}),
		returned: make(chan struct{}),
	}
	var processorReleaseOnce sync.Once
	releaseProcessor := func() { processorReleaseOnce.Do(func() { close(processor.release) }) }
	t.Cleanup(releaseProcessor)
	consumeDone := make(chan reliableConsumeResult, 1)
	go func() {
		retry, err := c.StartConsuming(1, processor)
		consumeDone <- reliableConsumeResult{retry: retry, err: err}
	}()
	select {
	case <-processor.started:
	case <-time.After(3 * time.Second):
		c.StopConsuming()
		t.Fatal("processor did not start")
	}
	select {
	case <-renewEntered:
	case <-time.After(3 * time.Second):
		releaseProcessor()
		c.StopConsuming()
		t.Fatal("renew call did not enter blocking hook")
	}

	stopCtx := c.GetStopCtx()
	stopDone := make(chan struct{})
	go func() {
		c.StopConsuming()
		close(stopDone)
	}()
	select {
	case <-stopCtx.Done():
	case <-time.After(3 * time.Second):
		releaseProcessor()
		releaseRenew()
		t.Fatal("StopConsuming did not cancel the consume attempt")
	}
	releaseProcessor()
	select {
	case <-processor.returned:
	case <-time.After(3 * time.Second):
		releaseRenew()
		t.Fatal("processor did not return")
	}
	select {
	case <-renewCanceled:
	case <-time.After(3 * time.Second):
		releaseRenew()
		t.Fatal("processor return did not cancel the blocked renew operation")
	}
	select {
	case <-ackEntered:
		releaseRenew()
		t.Fatal("finalization started before the blocked renew operation was joined")
	case <-stopDone:
		releaseRenew()
		t.Fatal("StopConsuming returned before the blocked renew operation was joined")
	case <-time.After(200 * time.Millisecond):
	}

	releaseRenew()
	select {
	case <-ackEntered:
	case <-time.After(3 * time.Second):
		t.Fatal("acknowledgement did not start after renew was released")
	}
	select {
	case <-stopDone:
	case <-time.After(3 * time.Second):
		t.Fatal("StopConsuming did not finish after processor and renewal returned")
	}
	if outcome := <-consumeDone; !errors.Is(outcome.err, ErrDeliveryLeaseLost) {
		t.Fatalf("StartConsuming error = %v, want ErrDeliveryLeaseLost after blocked renew outlives lease", outcome.err)
	}
	if err := client.Ping(context.Background()).Err(); err != nil {
		t.Fatalf("shared Redis client closed: %v", err)
	}
}

func TestIdleClaimRequestsAreBounded(t *testing.T) {
	const queue = "queue:{idle-bound}"
	c, client := newReliableTestController(t, queue)
	if err := reliableClaimScript.Load(context.Background(), client).Err(); err != nil {
		t.Fatalf("preload claim script: %v", err)
	}
	timing := &claimTimingHook{hash: reliableClaimScript.Hash(), times: make(chan time.Time, 32)}
	client.AddHook(timing)
	processor := &reliableTestProcessor{started: make(chan struct{}), release: make(chan struct{})}
	consumeDone := make(chan reliableConsumeResult, 1)
	go func() {
		retry, err := c.StartConsuming(1, processor)
		consumeDone <- reliableConsumeResult{retry: retry, err: err}
	}()

	var calls []time.Time
	deadline := time.After(8 * time.Second)
	for len(calls) < 8 {
		select {
		case calledAt := <-timing.times:
			calls = append(calls, calledAt)
		case <-deadline:
			c.StopConsuming()
			t.Fatalf("claim calls = %d, want 8", len(calls))
		}
	}
	steadyInterval := calls[7].Sub(calls[6])
	if steadyInterval < 800*time.Millisecond || steadyInterval > 1300*time.Millisecond {
		c.StopConsuming()
		t.Fatalf("steady idle claim interval = %v, want [800ms, 1.3s]", steadyInterval)
	}
	for count := uint64(1); count <= 64; count++ {
		delay := reliableIdleDelay(time.Second, queue, count)
		if delay < 800*time.Millisecond || delay > 1200*time.Millisecond {
			c.StopConsuming()
			t.Fatalf("idle jitter delay = %v, want [800ms, 1.2s]", delay)
		}
	}

	publishedAt := time.Now()
	body := reliableTaskBody(t, "idle-discovery")
	if err := client.RPush(context.Background(), queue, body).Err(); err != nil {
		c.StopConsuming()
		t.Fatalf("publish ready task: %v", err)
	}
	select {
	case <-processor.started:
		if elapsed := time.Since(publishedAt); elapsed > 1200*time.Millisecond {
			close(processor.release)
			c.StopConsuming()
			t.Fatalf("idle task discovery = %v, want <= 1.2s", elapsed)
		}
	case <-time.After(1500 * time.Millisecond):
		close(processor.release)
		c.StopConsuming()
		t.Fatal("published task not discovered from steady idle")
	}
	close(processor.release)
	c.StopConsuming()
	<-consumeDone
}

func TestProcessorErrorDefersDelivery(t *testing.T) {
	const queue = "queue:{reliable}"
	c, client := newReliableTestController(t, queue)
	body := reliableTaskBody(t, "processor-error")
	if err := client.RPush(context.Background(), queue, body).Err(); err != nil {
		t.Fatalf("enqueue task: %v", err)
	}
	wantErr := errors.New("processor failed")
	processor := &reliableTestProcessor{started: make(chan struct{}), err: wantErr}
	retry, err := c.StartConsuming(1, processor)
	if retry {
		t.Fatal("retry = true, want false")
	}
	if !errors.Is(err, wantErr) {
		t.Fatalf("StartConsuming error = %v, want processor failure", err)
	}
	prefix := reliableTaggedPrefix(queue, "reliable")
	durable := client.LLen(context.Background(), queue).Val() + client.HLen(context.Background(), prefix+":inflight").Val()
	if durable != 1 {
		t.Fatalf("durable copies after processor error = %d, want 1", durable)
	}
}

func TestQueuedPayloadValidationPreservesMalformedBytes(t *testing.T) {
	valid := reliableTaskBody(t, "payload-validation")
	tests := []struct {
		name      string
		body      []byte
		malformed bool
	}{
		{name: "invalid first byte", body: append([]byte("!"), valid...), malformed: true},
		{name: "invalid trailing byte", body: append(append([]byte(nil), valid...), '!'), malformed: true},
		{name: "second JSON value", body: append(append([]byte(nil), valid...), valid...), malformed: true},
		{name: "trailing whitespace", body: append(append([]byte(nil), valid...), []byte(" \n\t\r  ")...)},
	}

	for index, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			queue := fmt.Sprintf("queue:{payload-validation-%d}", index)
			c, client := newReliableTestController(t, queue)
			if err := client.RPush(context.Background(), queue, tt.body).Err(); err != nil {
				t.Fatalf("enqueue payload: %v", err)
			}
			processor := &reliableTestProcessor{started: make(chan struct{})}
			result := make(chan reliableConsumeResult, 1)
			go func() {
				retry, err := c.StartConsuming(1, processor)
				result <- reliableConsumeResult{retry: retry, err: err}
			}()

			keys := deriveReliableQueueKeys(queue)
			if tt.malformed {
				select {
				case outcome := <-result:
					if outcome.err == nil || !strings.Contains(outcome.err.Error(), "decode queued task") {
						t.Fatalf("StartConsuming error = %v, want contextual decode failure", outcome.err)
					}
				case <-processor.started:
					c.StopConsuming()
					<-result
					t.Fatal("processor ran for malformed queued payload")
				case <-time.After(3 * time.Second):
					c.StopConsuming()
					t.Fatal("malformed queued payload did not stop the consume attempt")
				}
				if got := processor.calls.Load(); got != 0 {
					t.Fatalf("processor calls = %d, want 0", got)
				}
				if got := client.ZCard(context.Background(), keys.outcomes).Val(); got != 0 {
					t.Fatalf("ack outcomes = %d, want 0", got)
				}
				ready, err := client.LRange(context.Background(), queue, 0, -1).Result()
				if err != nil {
					t.Fatalf("read ready queue: %v", err)
				}
				inflight, err := client.HGetAll(context.Background(), keys.inflight).Result()
				if err != nil {
					t.Fatalf("read inflight reservations: %v", err)
				}
				if copies := len(ready) + len(inflight); copies != 1 {
					t.Fatalf("ready + inflight copies = %d, want exactly 1", copies)
				}
				var recovered []byte
				if len(ready) == 1 {
					recovered = []byte(ready[0])
				}
				for _, envelope := range inflight {
					_, payload, err := decodeReliableEnvelope([]byte(envelope))
					if err != nil {
						t.Fatalf("decode retained envelope: %v", err)
					}
					recovered = payload
				}
				if !bytes.Equal(recovered, tt.body) {
					t.Fatalf("retained payload = %q, want exact bytes %q", recovered, tt.body)
				}
				if got := client.ZCard(context.Background(), keys.visibility).Val(); got != int64(len(inflight)) {
					t.Fatalf("visibility entries = %d, want %d", got, len(inflight))
				}
				if got := client.ZCard(context.Background(), keys.repairBacklog).Val(); got != 0 {
					t.Fatalf("repair backlog entries = %d, want 0", got)
				}
				return
			}

			select {
			case <-processor.started:
			case outcome := <-result:
				t.Fatalf("StartConsuming returned before valid payload processing: %v", outcome.err)
			case <-time.After(3 * time.Second):
				c.StopConsuming()
				t.Fatal("processor did not receive valid payload with trailing whitespace")
			}
			deadline := time.Now().Add(3 * time.Second)
			for client.ZCard(context.Background(), keys.outcomes).Val() != 1 && time.Now().Before(deadline) {
				time.Sleep(time.Millisecond)
			}
			if got := client.ZCard(context.Background(), keys.outcomes).Val(); got != 1 {
				c.StopConsuming()
				t.Fatalf("ack outcomes = %d, want 1", got)
			}
			c.StopConsuming()
			select {
			case outcome := <-result:
				if !errors.Is(outcome.err, context.Canceled) {
					t.Fatalf("StartConsuming error = %v, want context.Canceled after stop", outcome.err)
				}
			case <-time.After(3 * time.Second):
				t.Fatal("consumer did not stop")
			}
			if got := processor.calls.Load(); got != 1 {
				t.Fatalf("processor calls = %d, want 1", got)
			}
			if ready := client.LLen(context.Background(), queue).Val(); ready != 0 {
				t.Fatalf("ready tasks after valid processing = %d, want 0", ready)
			}
			if inflight := client.HLen(context.Background(), keys.inflight).Val(); inflight != 0 {
				t.Fatalf("inflight tasks after valid processing = %d, want 0", inflight)
			}
		})
	}
}
