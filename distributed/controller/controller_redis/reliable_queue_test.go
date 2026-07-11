package controller_redis

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/go-redis/redis/v8"
)

func newReliableTestQueue(t *testing.T, queue string, lease time.Duration, source io.Reader) (*reliableQueue, redis.UniversalClient) {
	t.Helper()
	_, _, client := newMiniController(t)
	return newReliableQueue(client, queue, lease, newDeliveryTokenGenerator(source)), client
}

type failingTokenReader struct{ err error }

func (r failingTokenReader) Read([]byte) (int, error) { return 0, r.err }

type fixedTokenReader struct {
	token [16]byte
	reads atomic.Int32
}

func (r *fixedTokenReader) Read(p []byte) (int, error) {
	r.reads.Add(1)
	for index := range p {
		p[index] = r.token[index%len(r.token)]
	}
	return len(p), nil
}

type loseScriptResponseHook struct {
	hash  string
	err   error
	armed atomic.Bool
}

func (h *loseScriptResponseHook) BeforeProcess(ctx context.Context, _ redis.Cmder) (context.Context, error) {
	return ctx, nil
}

func (h *loseScriptResponseHook) AfterProcess(_ context.Context, cmd redis.Cmder) error {
	if !strings.EqualFold(cmd.Name(), "evalsha") || !h.armed.Load() {
		return nil
	}
	args := cmd.Args()
	if len(args) < 2 || fmt.Sprint(args[1]) != h.hash || !h.armed.CompareAndSwap(true, false) {
		return nil
	}
	return h.err
}

func (*loseScriptResponseHook) BeforeProcessPipeline(ctx context.Context, _ []redis.Cmder) (context.Context, error) {
	return ctx, nil
}

func (*loseScriptResponseHook) AfterProcessPipeline(context.Context, []redis.Cmder) error { return nil }

func TestConcurrentClaimHasSingleLeaseOwner(t *testing.T) {
	queue := "queue:{single-owner}"
	q, client := newReliableTestQueue(t, queue, time.Second, nil)
	payload := []byte("serialized-task")
	if err := client.RPush(context.Background(), queue, payload).Err(); err != nil {
		t.Fatalf("enqueue: %v", err)
	}

	results := make(chan *reliableDelivery, 2)
	errs := make(chan error, 2)
	var wg sync.WaitGroup
	for index := 0; index < 2; index++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			delivery, err := q.claim(context.Background())
			results <- delivery
			errs <- err
		}()
	}
	wg.Wait()
	close(results)
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatalf("claim: %v", err)
		}
	}
	owners := 0
	for delivery := range results {
		if delivery != nil {
			owners++
		}
	}
	if owners != 1 {
		t.Fatalf("claim owners = %d, want 1", owners)
	}
	if got := client.HLen(context.Background(), q.keys.inflight).Val(); got != 1 {
		t.Fatalf("inflight = %d, want 1", got)
	}
}

func TestExpiredTokenCannotFinalizeDelivery(t *testing.T) {
	queue := "queue:{expiry-fence}"
	q, client := newReliableTestQueue(t, queue, 20*time.Millisecond, nil)
	payload := []byte("expiry-task")
	if err := client.RPush(context.Background(), queue, payload).Err(); err != nil {
		t.Fatalf("enqueue: %v", err)
	}
	delivery, err := q.claim(context.Background())
	if err != nil || delivery == nil {
		t.Fatalf("claim = (%v, %v), want delivery", delivery, err)
	}
	time.Sleep(30 * time.Millisecond)

	beforeEnvelope := client.HGet(context.Background(), q.keys.inflight, delivery.token).Val()
	beforeScore := client.ZScore(context.Background(), q.keys.visibility, delivery.token).Val()
	for name, operation := range map[string]func() error{
		"renew":   func() error { return q.renew(context.Background(), delivery) },
		"ack":     func() error { return q.acknowledge(context.Background(), delivery) },
		"release": func() error { return q.release(context.Background(), delivery) },
	} {
		if err := operation(); !errors.Is(err, ErrDeliveryLeaseLost) {
			t.Fatalf("%s error = %v, want ErrDeliveryLeaseLost", name, err)
		}
		if got := client.HGet(context.Background(), q.keys.inflight, delivery.token).Val(); got != beforeEnvelope {
			t.Fatalf("%s changed inflight envelope", name)
		}
		if got := client.ZScore(context.Background(), q.keys.visibility, delivery.token).Val(); got != beforeScore {
			t.Fatalf("%s changed visibility score", name)
		}
	}
}

func TestAbandonedDeliveryReclaimedAfterVisibility(t *testing.T) {
	queue := "queue:{reclaim}"
	q, client := newReliableTestQueue(t, queue, 25*time.Millisecond, nil)
	payload := []byte{0, 1, 2, 3, 255}
	if err := client.RPush(context.Background(), queue, payload).Err(); err != nil {
		t.Fatalf("enqueue: %v", err)
	}
	first, err := q.claim(context.Background())
	if err != nil || first == nil {
		t.Fatalf("first claim = (%v, %v)", first, err)
	}
	if early, err := q.claim(context.Background()); err != nil || early != nil {
		t.Fatalf("early reclaim = (%v, %v), want nil", early, err)
	}
	time.Sleep(35 * time.Millisecond)
	second, err := q.claim(context.Background())
	if err != nil || second == nil {
		t.Fatalf("reclaim = (%v, %v)", second, err)
	}
	if second.token == first.token {
		t.Fatal("reclaim reused delivery token")
	}
	if !bytes.Equal(second.payload, payload) {
		t.Fatalf("reclaimed payload = %v, want %v", second.payload, payload)
	}
}

func TestCrashAfterProcessBeforeAckPreservesIdentity(t *testing.T) {
	queue := "queue:{crash-before-ack}"
	q, client := newReliableTestQueue(t, queue, 25*time.Millisecond, nil)
	payload := []byte(`{"id":"stable-id","name":"task","args":["exact"]}`)
	if err := client.RPush(context.Background(), queue, payload).Err(); err != nil {
		t.Fatalf("enqueue: %v", err)
	}
	beforeCrash, err := q.claim(context.Background())
	if err != nil || beforeCrash == nil {
		t.Fatalf("claim before crash = (%v, %v)", beforeCrash, err)
	}
	time.Sleep(35 * time.Millisecond)
	afterCrash, err := q.claim(context.Background())
	if err != nil || afterCrash == nil {
		t.Fatalf("claim after crash = (%v, %v)", afterCrash, err)
	}
	if !bytes.Equal(afterCrash.payload, beforeCrash.payload) || !bytes.Equal(afterCrash.payload, payload) {
		t.Fatalf("redelivered bytes = %q, want exact original %q", afterCrash.payload, payload)
	}
	var decoded struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(afterCrash.payload, &decoded); err != nil || decoded.ID != "stable-id" {
		t.Fatalf("redelivered Signature.ID = %q, error = %v", decoded.ID, err)
	}
}

func TestAcknowledgementOutcomeConfirmsRetry(t *testing.T) {
	queue := "queue:{ack-outcome}"
	q, client := newReliableTestQueue(t, queue, 50*time.Millisecond, nil)
	if err := client.RPush(context.Background(), queue, "ack-task").Err(); err != nil {
		t.Fatalf("enqueue: %v", err)
	}
	delivery, err := q.claim(context.Background())
	if err != nil || delivery == nil {
		t.Fatalf("claim = (%v, %v)", delivery, err)
	}
	if err := q.acknowledge(context.Background(), delivery); err != nil {
		t.Fatalf("first ack: %v", err)
	}
	time.Sleep(60 * time.Millisecond)
	if err := q.acknowledge(context.Background(), delivery); err != nil {
		t.Fatalf("confirmation ack after lease: %v", err)
	}
	if score := client.ZScore(context.Background(), q.keys.outcomes, delivery.token).Val(); score == 0 {
		t.Fatal("ack outcome missing")
	}
	if ttl := client.TTL(context.Background(), q.keys.outcomes).Val(); ttl <= 0 || ttl > ackOutcomeKeyTTL {
		t.Fatalf("ack outcome TTL = %v, want (0, %v]", ttl, ackOutcomeKeyTTL)
	}
}

func TestReliableDeliveryRedisFailuresRemainRecoverable(t *testing.T) {
	queue := "queue:{lost-ack-response}"
	q, client := newReliableTestQueue(t, queue, time.Second, nil)
	lostResponse := errors.New("ack response lost")
	hook := &loseScriptResponseHook{hash: reliableAckScript.Hash(), err: lostResponse}
	client.AddHook(hook)
	if err := reliableAckScript.Load(context.Background(), client).Err(); err != nil {
		t.Fatalf("preload ack script: %v", err)
	}
	if err := client.RPush(context.Background(), queue, "ack-lost-task").Err(); err != nil {
		t.Fatalf("enqueue: %v", err)
	}
	delivery, err := q.claim(context.Background())
	if err != nil || delivery == nil {
		t.Fatalf("claim = (%v, %v)", delivery, err)
	}
	hook.armed.Store(true)
	c := &ControllerRedis{finalizationTimeout: 100 * time.Millisecond, ackConfirmationWindow: 500 * time.Millisecond}
	if err := c.acknowledgeReliableDelivery(q, delivery); err != nil {
		t.Fatalf("bounded ack confirmation: %v", err)
	}
	if hook.armed.Load() {
		t.Fatal("test hook did not lose the committed ACK response")
	}
	if got := client.ZScore(context.Background(), q.keys.outcomes, delivery.token).Val(); got == 0 {
		t.Fatal("committed ACK outcome missing")
	}
	if got := client.HLen(context.Background(), q.keys.inflight).Val(); got != 0 {
		t.Fatalf("inflight after confirmed ACK = %d, want 0", got)
	}
}

func TestLuaOrphanReconciliationRetainsPayload(t *testing.T) {
	queue := "queue:{orphan}"
	q, client := newReliableTestQueue(t, queue, time.Second, nil)
	payload := []byte("orphan-payload")
	if err := client.HSet(context.Background(), q.keys.inflight, "orphan-token", append([]byte("0000000000:"), payload...)).Err(); err != nil {
		t.Fatalf("seed orphan: %v", err)
	}
	delivery, err := q.claim(context.Background())
	if err != nil || delivery == nil {
		t.Fatalf("claim reconciled payload = (%v, %v)", delivery, err)
	}
	if !bytes.Equal(delivery.payload, payload) {
		t.Fatalf("reconciled payload = %q, want %q", delivery.payload, payload)
	}
}

func TestPermanentFailuresUseBoundedBackoff(t *testing.T) {
	queue := "queue:{backoff}"
	q, client := newReliableTestQueue(t, queue, time.Second, nil)
	payload := []byte("poison-task")
	if err := client.RPush(context.Background(), queue, payload).Err(); err != nil {
		t.Fatalf("enqueue: %v", err)
	}
	delivery, err := q.claim(context.Background())
	if err != nil || delivery == nil {
		t.Fatalf("claim = (%v, %v)", delivery, err)
	}
	next, err := q.deferRetry(context.Background(), delivery)
	if err != nil {
		t.Fatalf("defer retry: %v", err)
	}
	if next.failures != 1 {
		t.Fatalf("failure count = %d, want 1", next.failures)
	}
	delay := next.deadline.Sub(next.serverTime)
	if delay < 800*time.Millisecond || delay > 1200*time.Millisecond {
		t.Fatalf("first retry delay = %v, want [800ms, 1.2s]", delay)
	}
	for failures := uint64(0); failures < 20; failures++ {
		capped := reliableRetryDelay(failures, payload)
		if capped < 800*time.Millisecond || capped > 60*time.Second {
			t.Fatalf("failure %d delay = %v, outside bounded range", failures, capped)
		}
	}
}

func TestDeliveryTokenGenerationIsBounded(t *testing.T) {
	t.Run("entropy failure", func(t *testing.T) {
		queue := "queue:{entropy}"
		wantErr := errors.New("entropy unavailable")
		q, client := newReliableTestQueue(t, queue, time.Second, failingTokenReader{err: wantErr})
		if err := client.RPush(context.Background(), queue, "task").Err(); err != nil {
			t.Fatalf("enqueue: %v", err)
		}
		if _, err := q.claim(context.Background()); !errors.Is(err, ErrDeliveryTokenUnavailable) || !errors.Is(err, wantErr) {
			t.Fatalf("claim error = %v, want token and source errors", err)
		}
		if client.LLen(context.Background(), queue).Val() != 1 || client.HLen(context.Background(), q.keys.inflight).Val() != 0 {
			t.Fatal("entropy failure mutated task")
		}
	})

	t.Run("collision exhaustion", func(t *testing.T) {
		queue := "queue:{collision}"
		reader := &fixedTokenReader{}
		q, client := newReliableTestQueue(t, queue, time.Second, reader)
		if err := client.RPush(context.Background(), queue, "first").Err(); err != nil {
			t.Fatalf("enqueue first: %v", err)
		}
		first, err := q.claim(context.Background())
		if err != nil || first == nil {
			t.Fatalf("first claim = (%v, %v)", first, err)
		}
		if err := client.RPush(context.Background(), queue, "second").Err(); err != nil {
			t.Fatalf("enqueue second: %v", err)
		}
		if _, err := q.claim(context.Background()); !errors.Is(err, ErrDeliveryTokenCollision) {
			t.Fatalf("collision claim error = %v, want ErrDeliveryTokenCollision", err)
		}
		if got := reader.reads.Load(); got != 5 {
			t.Fatalf("token reads = %d, want initial claim + 4 bounded collision attempts", got)
		}
		if got := client.LIndex(context.Background(), queue, 0).Val(); got != "second" {
			t.Fatalf("collision changed ready task to %q", got)
		}
		if got := client.HGet(context.Background(), q.keys.inflight, first.token).Val(); got == "" {
			t.Fatal("collision overwrote existing reservation")
		}
	})
}

func TestLegacyQueueAndClusterKeyCompatibility(t *testing.T) {
	var firstTags [16384]string
	remaining := len(firstTags)
	maxCandidate := -1
	for candidate := 0; candidate <= 131071 && remaining > 0; candidate++ {
		tag := fmt.Sprintf("gkit-%x", candidate)
		slot := int(redisCRC16([]byte(tag)) % 16384)
		if firstTags[slot] == "" {
			firstTags[slot] = tag
			remaining--
			maxCandidate = candidate
		}
	}
	if remaining != 0 || maxCandidate > 131071 {
		t.Fatalf("offline candidate traversal left %d slots, max candidate %d", remaining, maxCandidate)
	}
	for slot, tag := range firstTags {
		if got := redisClusterSlot("{" + tag + "}"); got != slot {
			t.Fatalf("slot %d candidate %q hashes to %d", slot, tag, got)
		}
	}

	reliableSlotTags.Lock()
	reliableSlotTags.tags = make(map[int]string)
	reliableSlotTags.Unlock()
	for _, slot := range []int{0, 1, 8192, 16383} {
		if tag := reliableTagForSlot(slot); tag != firstTags[slot] {
			t.Fatalf("slot %d lazy candidate = %q, want first %q", slot, tag, firstTags[slot])
		}
		if again := reliableTagForSlot(slot); again != firstTags[slot] {
			t.Fatalf("slot %d cached candidate = %q, want %q", slot, again, firstTags[slot])
		}
	}
	reliableSlotTags.RLock()
	if cached := len(reliableSlotTags.tags); cached != 4 {
		reliableSlotTags.RUnlock()
		t.Fatalf("lazy slot cache entries = %d, want 4", cached)
	}
	reliableSlotTags.RUnlock()

	for _, queue := range []string{"legacy-queue", "queue:{tagged}", "queue:{broken"} {
		keys := deriveReliableQueueKeys(queue)
		wantSlot := redisClusterSlot(queue)
		for _, key := range []string{keys.inflight, keys.visibility, keys.outcomes, keys.repairCursor, keys.repairBacklog} {
			if got := redisClusterSlot(key); got != wantSlot {
				t.Fatalf("queue %q internal key %q slot = %d, want %d", queue, key, got, wantSlot)
			}
		}
	}
	first := deriveReliableQueueKeys("first:{shared}")
	second := deriveReliableQueueKeys("second:{shared}")
	if first.prefix == second.prefix {
		t.Fatal("queue digest did not distinguish same-slot queue names")
	}
	reliableSlotTags.RLock()
	if cached := len(reliableSlotTags.tags); cached >= 16384 {
		reliableSlotTags.RUnlock()
		t.Fatalf("key derivation eagerly populated %d slot tags", cached)
	}
	reliableSlotTags.RUnlock()
}
