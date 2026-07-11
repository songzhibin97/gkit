package backend_redis

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/go-redis/redis/v8"
	"github.com/songzhibin97/gkit/distributed/backend"
	"github.com/songzhibin97/gkit/distributed/task"
)

type issue104CommandCountHook struct {
	mu   sync.Mutex
	gets int
}

type issue104FailCommandHook struct {
	mu      sync.Mutex
	command string
	fail    bool
}

type issue104SetNXBarrierHook struct {
	once    sync.Once
	entered chan struct{}
	release chan struct{}
}

func (h *issue104SetNXBarrierHook) BeforeProcess(ctx context.Context, cmd redis.Cmder) (context.Context, error) {
	if strings.EqualFold(cmd.Name(), "setnx") {
		h.once.Do(func() {
			close(h.entered)
			<-h.release
		})
	}
	return ctx, nil
}

func (*issue104SetNXBarrierHook) AfterProcess(context.Context, redis.Cmder) error { return nil }
func (h *issue104SetNXBarrierHook) BeforeProcessPipeline(ctx context.Context, _ []redis.Cmder) (context.Context, error) {
	return ctx, nil
}
func (*issue104SetNXBarrierHook) AfterProcessPipeline(context.Context, []redis.Cmder) error {
	return nil
}

type issue104DeleteBarrierHook struct {
	recordKey       string
	deleted         chan struct{}
	release         chan struct{}
	sawDeleting     chan struct{}
	deleteOnce      sync.Once
	registrationOne sync.Once
}

type issue104PreDeleteBarrierHook struct {
	fenced      chan struct{}
	deleteReady chan struct{}
	release     chan struct{}
	fenceOnce   sync.Once
	deleteOnce  sync.Once
}

type issue104StaleScannerBarrierHook struct {
	ensureReady   chan struct{}
	releaseEnsure chan struct{}
	setNXReady    chan struct{}
	releaseSetNX  chan struct{}
	ensureOnce    sync.Once
	setNXOnce     sync.Once
}

func (h *issue104StaleScannerBarrierHook) BeforeProcess(ctx context.Context, cmd redis.Cmder) (context.Context, error) {
	args := cmd.Args()
	if strings.EqualFold(cmd.Name(), "eval") && len(args) > 1 && fmt.Sprint(args[1]) == redisChordEnsureIndexScript {
		h.ensureOnce.Do(func() {
			close(h.ensureReady)
			<-h.releaseEnsure
		})
	}
	if strings.EqualFold(cmd.Name(), "setnx") {
		h.setNXOnce.Do(func() {
			close(h.setNXReady)
			<-h.releaseSetNX
		})
	}
	return ctx, nil
}

func (*issue104StaleScannerBarrierHook) AfterProcess(context.Context, redis.Cmder) error { return nil }
func (h *issue104StaleScannerBarrierHook) BeforeProcessPipeline(ctx context.Context, cmds []redis.Cmder) (context.Context, error) {
	for _, cmd := range cmds {
		if _, err := h.BeforeProcess(ctx, cmd); err != nil {
			return ctx, err
		}
	}
	return ctx, nil
}
func (*issue104StaleScannerBarrierHook) AfterProcessPipeline(context.Context, []redis.Cmder) error {
	return nil
}

func (h *issue104PreDeleteBarrierHook) BeforeProcess(ctx context.Context, cmd redis.Cmder) (context.Context, error) {
	args := cmd.Args()
	if strings.EqualFold(cmd.Name(), "eval") && len(args) > 1 && fmt.Sprint(args[1]) == redisChordDeleteRecordScript {
		h.deleteOnce.Do(func() {
			close(h.deleteReady)
			<-h.release
		})
	}
	return ctx, nil
}

func (h *issue104PreDeleteBarrierHook) AfterProcess(_ context.Context, cmd redis.Cmder) error {
	args := cmd.Args()
	if strings.EqualFold(cmd.Name(), "eval") && len(args) > 1 && fmt.Sprint(args[1]) == redisChordBeginDeleteScript {
		if value, err := cmd.(*redis.Cmd).Int(); err == nil && value == 1 {
			h.fenceOnce.Do(func() { close(h.fenced) })
		}
	}
	return nil
}

func (h *issue104PreDeleteBarrierHook) BeforeProcessPipeline(ctx context.Context, cmds []redis.Cmder) (context.Context, error) {
	for _, cmd := range cmds {
		if _, err := h.BeforeProcess(ctx, cmd); err != nil {
			return ctx, err
		}
	}
	return ctx, nil
}
func (*issue104PreDeleteBarrierHook) AfterProcessPipeline(context.Context, []redis.Cmder) error {
	return nil
}

func (h *issue104DeleteBarrierHook) BeforeProcess(ctx context.Context, _ redis.Cmder) (context.Context, error) {
	return ctx, nil
}

func (h *issue104DeleteBarrierHook) AfterProcess(_ context.Context, cmd redis.Cmder) error {
	args := cmd.Args()
	if strings.EqualFold(cmd.Name(), "eval") && len(args) > 3 && fmt.Sprint(args[1]) == redisChordDeleteRecordScript && fmt.Sprint(args[3]) == h.recordKey {
		h.deleteOnce.Do(func() {
			close(h.deleted)
			<-h.release
		})
	}
	if strings.EqualFold(cmd.Name(), "eval") && len(args) > 1 && fmt.Sprint(args[1]) == redisChordPrepareIndexScript {
		if value, err := cmd.(*redis.Cmd).Text(); err == nil && strings.HasPrefix(value, "deleting:") {
			h.registrationOne.Do(func() { close(h.sawDeleting) })
		}
	}
	return nil
}

func (h *issue104DeleteBarrierHook) BeforeProcessPipeline(ctx context.Context, _ []redis.Cmder) (context.Context, error) {
	return ctx, nil
}
func (h *issue104DeleteBarrierHook) AfterProcessPipeline(ctx context.Context, cmds []redis.Cmder) error {
	for _, cmd := range cmds {
		if err := h.AfterProcess(ctx, cmd); err != nil {
			return err
		}
	}
	return nil
}

func (h *issue104FailCommandHook) BeforeProcess(ctx context.Context, cmd redis.Cmder) (context.Context, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.fail && strings.EqualFold(cmd.Name(), h.command) {
		h.fail = false
		return ctx, errors.New("injected redis registration failure")
	}
	return ctx, nil
}

func (*issue104FailCommandHook) AfterProcess(context.Context, redis.Cmder) error { return nil }
func (h *issue104FailCommandHook) BeforeProcessPipeline(ctx context.Context, _ []redis.Cmder) (context.Context, error) {
	return ctx, nil
}
func (*issue104FailCommandHook) AfterProcessPipeline(context.Context, []redis.Cmder) error {
	return nil
}

func (h *issue104CommandCountHook) BeforeProcess(ctx context.Context, _ redis.Cmder) (context.Context, error) {
	return ctx, nil
}

func (h *issue104CommandCountHook) AfterProcess(_ context.Context, cmd redis.Cmder) error {
	if strings.EqualFold(cmd.Name(), "get") {
		h.mu.Lock()
		h.gets++
		h.mu.Unlock()
	}
	return nil
}

func (h *issue104CommandCountHook) BeforeProcessPipeline(ctx context.Context, _ []redis.Cmder) (context.Context, error) {
	return ctx, nil
}

func (h *issue104CommandCountHook) AfterProcessPipeline(_ context.Context, cmds []redis.Cmder) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	for _, cmd := range cmds {
		if strings.EqualFold(cmd.Name(), "get") {
			h.gets++
		}
	}
	return nil
}

func (h *issue104CommandCountHook) reset() {
	h.mu.Lock()
	h.gets = 0
	h.mu.Unlock()
}

func (h *issue104CommandCountHook) count() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.gets
}

func TestDurableChordRedisBoundedPagination(t *testing.T) {
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	runDurableChordRedisBoundedPagination(t, client)
}

func TestDurableChordRedisPaginationRepairsHolesAndPartialRegistration(t *testing.T) {
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	b := NewBackendRedis(client, -1).(*BackendRedis)
	ctx := context.Background()

	for index := 0; index < 25; index++ {
		registration := issue104PaginationRegistration(t, fmt.Sprintf("hole-group-%02d", index))
		if _, err := b.RegisterChord(ctx, registration); err != nil {
			t.Fatal(err)
		}
	}
	indexed, err := client.ZRange(ctx, redisChordDeliveryIndexKey, 0, -1).Result()
	if err != nil {
		t.Fatal(err)
	}
	for _, deliveryKey := range indexed[:7] {
		recordKey, err := redisChordRecordKey(deliveryKey)
		if err != nil {
			t.Fatal(err)
		}
		if err := client.Del(ctx, recordKey).Err(); err != nil {
			t.Fatal(err)
		}
	}
	seen := make(map[string]struct{})
	cursor := ""
	for {
		page, err := b.ScanChordDeliveries(ctx, backend.ChordScan{Cursor: cursor, Limit: 5})
		if err != nil {
			t.Fatal(err)
		}
		for _, delivery := range page.Deliveries {
			if _, duplicate := seen[delivery.DeliveryKey]; duplicate {
				t.Fatalf("duplicate delivery after holes: %s", delivery.DeliveryKey)
			}
			seen[delivery.DeliveryKey] = struct{}{}
		}
		if page.NextCursor == "" {
			break
		}
		cursor = page.NextCursor
	}
	if len(seen) != 18 {
		t.Fatalf("deliveries after holes = %d, want 18", len(seen))
	}
	if got := client.ZCard(ctx, redisChordDeliveryIndexKey).Val(); got != 18 {
		t.Fatalf("repaired delivery index size = %d, want 18", got)
	}

	missingIndexRegistration := issue104PaginationRegistration(t, "missing-index-group")
	missingRef, err := b.RegisterChord(ctx, missingIndexRegistration)
	if err != nil {
		t.Fatal(err)
	}
	if err := client.ZRem(ctx, redisChordDeliveryIndexKey, missingRef.DeliveryKey).Err(); err != nil {
		t.Fatal(err)
	}
	attached, err := b.RegisterChord(ctx, missingIndexRegistration)
	if err != nil || attached.Created {
		t.Fatalf("repair registration = %#v, %v", attached, err)
	}
	if scoreErr := client.ZScore(ctx, redisChordDeliveryIndexKey, missingRef.DeliveryKey).Err(); scoreErr != nil {
		t.Fatalf("missing index was not repaired: %v", scoreErr)
	}

	failHook := &issue104FailCommandHook{command: "setnx", fail: true}
	client.AddHook(failHook)
	partial := issue104PaginationRegistration(t, "partial-index-group")
	if _, err := b.RegisterChord(ctx, partial); err == nil || !strings.Contains(err.Error(), "injected redis registration failure") {
		t.Fatalf("partial registration error = %v", err)
	}
	if scoreErr := client.ZScore(ctx, redisChordDeliveryIndexKey, partial.DeliveryKey).Err(); scoreErr != nil {
		t.Fatalf("index-first partial registration left no repairable hole: %v", scoreErr)
	}
	page, err := b.ScanChordDeliveries(ctx, backend.ChordScan{Limit: 100})
	if err != nil {
		t.Fatal(err)
	}
	for _, delivery := range page.Deliveries {
		if delivery.DeliveryKey == partial.DeliveryKey {
			t.Fatal("index hole surfaced as a delivery")
		}
	}
	if state := client.HGet(ctx, redisChordIndexStateKey, partial.DeliveryKey).Val(); !strings.HasPrefix(state, "pending:") {
		t.Fatalf("partial registration state = %q, want fenced pending generation", state)
	}
	retried, err := b.RegisterChord(ctx, partial)
	if err != nil || !retried.Created {
		t.Fatalf("partial registration retry = %#v, %v", retried, err)
	}
	if state := client.HGet(ctx, redisChordIndexStateKey, partial.DeliveryKey).Val(); state != "committed:"+retried.Owner {
		t.Fatalf("retried registration state = %q, want committed owner", state)
	}

	if _, err := b.ScanChordDeliveries(ctx, backend.ChordScan{Cursor: "raw-delivery-key", Limit: 5}); err == nil {
		t.Fatal("raw/invalid Redis chord cursor was accepted")
	}
}

func TestDurableChordRedisPendingRegistrationFencesHoleCleanup(t *testing.T) {
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	barrier := &issue104SetNXBarrierHook{entered: make(chan struct{}), release: make(chan struct{})}
	client.AddHook(barrier)
	b := NewBackendRedis(client, -1).(*BackendRedis)
	registration := issue104PaginationRegistration(t, "pending-registration-race")

	type registerResult struct {
		ref backend.ChordRegistrationRef
		err error
	}
	result := make(chan registerResult, 1)
	go func() {
		ref, err := b.RegisterChord(context.Background(), registration)
		result <- registerResult{ref: ref, err: err}
	}()
	<-barrier.entered
	page, err := b.ScanChordDeliveries(context.Background(), backend.ChordScan{Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Deliveries) != 0 {
		t.Fatalf("pending registration surfaced before SETNX: %#v", page.Deliveries)
	}
	if scoreErr := client.ZScore(context.Background(), redisChordDeliveryIndexKey, registration.DeliveryKey).Err(); scoreErr != nil {
		t.Fatalf("scanner removed pending index generation: %v", scoreErr)
	}
	close(barrier.release)
	registered := <-result
	if registered.err != nil || !registered.ref.Created {
		t.Fatalf("registration result = %#v, %v", registered.ref, registered.err)
	}
	if state := client.HGet(context.Background(), redisChordIndexStateKey, registration.DeliveryKey).Val(); state != "committed:"+registered.ref.Owner {
		t.Fatalf("registration state = %q, want committed owner", state)
	}
}

func TestDurableChordRedisAttachPrepareDoesNotPoisonCommittedOwner(t *testing.T) {
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	b := NewBackendRedis(client, -1).(*BackendRedis)
	ctx := context.Background()
	registration := issue104PaginationRegistration(t, "attach-prepare-crash")
	oldRef, err := b.RegisterChord(ctx, registration)
	if err != nil {
		t.Fatal(err)
	}
	barrier := &issue104SetNXBarrierHook{entered: make(chan struct{}), release: make(chan struct{})}
	client.AddHook(barrier)

	type registerResult struct {
		ref backend.ChordRegistrationRef
		err error
	}
	attachDone := make(chan registerResult, 1)
	go func() {
		ref, attachErr := b.RegisterChord(ctx, registration)
		attachDone <- registerResult{ref: ref, err: attachErr}
	}()
	<-barrier.entered
	if state := client.HGet(ctx, redisChordIndexStateKey, oldRef.DeliveryKey).Val(); state != "committed:"+oldRef.Owner {
		t.Fatalf("attach prepare changed committed owner to %q", state)
	}
	if err := b.AbortRegistration(ctx, oldRef); err != nil {
		t.Fatalf("original owner could not abort after attach stopped at SETNX: %v", err)
	}
	close(barrier.release)
	replacement := <-attachDone
	if replacement.err != nil || !replacement.ref.Created {
		t.Fatalf("replacement after abort = %#v, %v", replacement.ref, replacement.err)
	}
	if state := client.HGet(ctx, redisChordIndexStateKey, replacement.ref.DeliveryKey).Val(); state != "committed:"+replacement.ref.Owner {
		t.Fatalf("replacement state = %q, want committed owner", state)
	}
}

func TestDurableChordRedisStaleScannerCannotOverwritePendingGeneration(t *testing.T) {
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	b := NewBackendRedis(client, -1).(*BackendRedis)
	ctx := context.Background()
	registration := issue104PaginationRegistration(t, "stale-scanner-generation-race")
	oldRef, err := b.RegisterChord(ctx, registration)
	if err != nil {
		t.Fatal(err)
	}
	barrier := &issue104StaleScannerBarrierHook{
		ensureReady:   make(chan struct{}),
		releaseEnsure: make(chan struct{}),
		setNXReady:    make(chan struct{}),
		releaseSetNX:  make(chan struct{}),
	}
	client.AddHook(barrier)
	scanDone := make(chan error, 1)
	go func() {
		_, scanErr := b.ScanChordDeliveries(ctx, backend.ChordScan{Limit: 10})
		scanDone <- scanErr
	}()
	<-barrier.ensureReady
	if err := b.AbortRegistration(ctx, oldRef); err != nil {
		t.Fatal(err)
	}

	type registerResult struct {
		ref backend.ChordRegistrationRef
		err error
	}
	registerDone := make(chan registerResult, 1)
	go func() {
		ref, registerErr := b.RegisterChord(ctx, registration)
		registerDone <- registerResult{ref: ref, err: registerErr}
	}()
	<-barrier.setNXReady
	pendingState := client.HGet(ctx, redisChordIndexStateKey, oldRef.DeliveryKey).Val()
	if !strings.HasPrefix(pendingState, "pending:") {
		t.Fatalf("replacement state = %q, want pending generation", pendingState)
	}
	close(barrier.releaseEnsure)
	if err := <-scanDone; err != nil {
		t.Fatal(err)
	}
	if state := client.HGet(ctx, redisChordIndexStateKey, oldRef.DeliveryKey).Val(); state != pendingState {
		t.Fatalf("stale scanner changed pending generation from %q to %q", pendingState, state)
	}
	close(barrier.releaseSetNX)
	registered := <-registerDone
	if registered.err != nil || !registered.ref.Created {
		t.Fatalf("replacement registration = %#v, %v", registered.ref, registered.err)
	}
	if state := client.HGet(ctx, redisChordIndexStateKey, registered.ref.DeliveryKey).Val(); state != "committed:"+registered.ref.Owner {
		t.Fatalf("replacement state = %q, want committed owner", state)
	}
	if err := b.AbortRegistration(ctx, registered.ref); err != nil {
		t.Fatalf("replacement owner could not acquire abort fence: %v", err)
	}
}

func TestDurableChordRedisCleanupFencesConcurrentReregistration(t *testing.T) {
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	b := NewBackendRedis(client, -1).(*BackendRedis)
	ctx := context.Background()
	registration := issue104PaginationRegistration(t, "cleanup-reregistration-race")
	oldRef, err := b.RegisterChord(ctx, registration)
	if err != nil {
		t.Fatal(err)
	}
	recordKey, err := redisChordRecordKey(oldRef.DeliveryKey)
	if err != nil {
		t.Fatal(err)
	}
	delivery, err := b.loadChordDelivery(ctx, recordKey)
	if err != nil {
		t.Fatal(err)
	}
	past := time.Now().Add(-time.Minute)
	delivery.TerminalExpireAt = &past
	body, err := json.Marshal(delivery)
	if err != nil {
		t.Fatal(err)
	}
	if err := client.Set(ctx, recordKey, body, 0).Err(); err != nil {
		t.Fatal(err)
	}
	if err := client.ZAdd(ctx, redisChordTerminalIndexKey, &redis.Z{Score: float64(past.UnixMilli()), Member: oldRef.DeliveryKey}).Err(); err != nil {
		t.Fatal(err)
	}

	barrier := &issue104DeleteBarrierHook{
		recordKey:   recordKey,
		deleted:     make(chan struct{}),
		release:     make(chan struct{}),
		sawDeleting: make(chan struct{}),
	}
	client.AddHook(barrier)
	cleanupResult := make(chan error, 1)
	go func() {
		_, cleanupErr := b.CleanupTerminalChordDeliveries(ctx, time.Now(), 1)
		cleanupResult <- cleanupErr
	}()
	<-barrier.deleted
	oldDeletingState := client.HGet(ctx, redisChordIndexStateKey, oldRef.DeliveryKey).Val()
	if !strings.HasPrefix(oldDeletingState, "deleting:") {
		t.Fatalf("cleanup state = %q, want deleting generation", oldDeletingState)
	}

	type registerResult struct {
		ref backend.ChordRegistrationRef
		err error
	}
	registerDone := make(chan registerResult, 1)
	go func() {
		ref, registerErr := b.RegisterChord(ctx, registration)
		registerDone <- registerResult{ref: ref, err: registerErr}
	}()
	<-barrier.sawDeleting
	close(barrier.release)
	if err := <-cleanupResult; err != nil {
		t.Fatal(err)
	}
	registered := <-registerDone
	if registered.err != nil || !registered.ref.Created {
		t.Fatalf("concurrent registration = %#v, %v", registered.ref, registered.err)
	}
	if err := b.removeChordIndexesIfState(ctx, registered.ref.DeliveryKey, oldDeletingState); err != nil {
		t.Fatal(err)
	}
	if scoreErr := client.ZScore(ctx, redisChordDeliveryIndexKey, registered.ref.DeliveryKey).Err(); scoreErr != nil {
		t.Fatalf("stale cleanup removed new discovery index: %v", scoreErr)
	}
	page, err := b.ScanChordDeliveries(ctx, backend.ChordScan{Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Deliveries) != 1 || page.Deliveries[0].RegistrationOwner != registered.ref.Owner {
		t.Fatalf("new generation is not discoverable: %#v", page.Deliveries)
	}
}

func TestDurableChordRedisAbortFencesConcurrentReregistration(t *testing.T) {
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	b := NewBackendRedis(client, -1).(*BackendRedis)
	ctx := context.Background()
	registration := issue104PaginationRegistration(t, "abort-reregistration-race")
	oldRef, err := b.RegisterChord(ctx, registration)
	if err != nil {
		t.Fatal(err)
	}
	recordKey, err := redisChordRecordKey(oldRef.DeliveryKey)
	if err != nil {
		t.Fatal(err)
	}
	barrier := &issue104DeleteBarrierHook{
		recordKey:   recordKey,
		deleted:     make(chan struct{}),
		release:     make(chan struct{}),
		sawDeleting: make(chan struct{}),
	}
	client.AddHook(barrier)
	abortDone := make(chan error, 1)
	go func() { abortDone <- b.AbortRegistration(ctx, oldRef) }()
	<-barrier.deleted
	oldDeletingState := client.HGet(ctx, redisChordIndexStateKey, oldRef.DeliveryKey).Val()
	if !strings.HasPrefix(oldDeletingState, "deleting:") {
		t.Fatalf("abort state = %q, want deleting generation", oldDeletingState)
	}

	type registerResult struct {
		ref backend.ChordRegistrationRef
		err error
	}
	registerDone := make(chan registerResult, 1)
	go func() {
		ref, registerErr := b.RegisterChord(ctx, registration)
		registerDone <- registerResult{ref: ref, err: registerErr}
	}()
	<-barrier.sawDeleting
	close(barrier.release)
	if err := <-abortDone; err != nil {
		t.Fatal(err)
	}
	registered := <-registerDone
	if registered.err != nil || !registered.ref.Created {
		t.Fatalf("concurrent registration = %#v, %v", registered.ref, registered.err)
	}
	if err := b.removeChordIndexesIfState(ctx, registered.ref.DeliveryKey, oldDeletingState); err != nil {
		t.Fatal(err)
	}
	if scoreErr := client.ZScore(ctx, redisChordDeliveryIndexKey, registered.ref.DeliveryKey).Err(); scoreErr != nil {
		t.Fatalf("stale abort removed new discovery index: %v", scoreErr)
	}
}

func TestDurableChordRedisCleanupOwnerFenceSurvivesTTLAndReregistration(t *testing.T) {
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	b := NewBackendRedis(client, -1).(*BackendRedis)
	ctx := context.Background()
	registration := issue104PaginationRegistration(t, "cleanup-ttl-owner-race")
	oldRef, err := b.RegisterChord(ctx, registration)
	if err != nil {
		t.Fatal(err)
	}
	recordKey, err := redisChordRecordKey(oldRef.DeliveryKey)
	if err != nil {
		t.Fatal(err)
	}
	delivery, err := b.loadChordDelivery(ctx, recordKey)
	if err != nil {
		t.Fatal(err)
	}
	past := time.Now().Add(-time.Minute)
	delivery.TerminalExpireAt = &past
	body, err := json.Marshal(delivery)
	if err != nil {
		t.Fatal(err)
	}
	if err := client.Set(ctx, recordKey, body, time.Second).Err(); err != nil {
		t.Fatal(err)
	}
	if err := client.ZAdd(ctx, redisChordTerminalIndexKey, &redis.Z{Score: float64(past.UnixMilli()), Member: oldRef.DeliveryKey}).Err(); err != nil {
		t.Fatal(err)
	}
	barrier := &issue104PreDeleteBarrierHook{
		fenced:      make(chan struct{}),
		deleteReady: make(chan struct{}),
		release:     make(chan struct{}),
	}
	client.AddHook(barrier)
	cleanupDone := make(chan error, 1)
	go func() {
		_, cleanupErr := b.CleanupTerminalChordDeliveries(ctx, time.Now(), 1)
		cleanupDone <- cleanupErr
	}()
	<-barrier.fenced
	<-barrier.deleteReady
	mr.FastForward(2 * time.Second)
	if exists := client.Exists(ctx, recordKey).Val(); exists != 0 {
		t.Fatalf("old terminal record still exists after TTL: %d", exists)
	}
	newRef, err := b.RegisterChord(ctx, registration)
	if err != nil || !newRef.Created {
		t.Fatalf("replacement registration = %#v, %v", newRef, err)
	}
	close(barrier.release)
	if err := <-cleanupDone; err != nil {
		t.Fatal(err)
	}
	current, err := b.loadChordDelivery(ctx, recordKey)
	if err != nil {
		t.Fatalf("old cleanup deleted replacement record: %v", err)
	}
	if current.RegistrationOwner != newRef.Owner {
		t.Fatalf("replacement owner = %q, want %q", current.RegistrationOwner, newRef.Owner)
	}
	if scoreErr := client.ZScore(ctx, redisChordDeliveryIndexKey, newRef.DeliveryKey).Err(); scoreErr != nil {
		t.Fatalf("replacement discovery index missing: %v", scoreErr)
	}
}

func TestDurableChordRedisBoundedPaginationLive(t *testing.T) {
	if addr := os.Getenv("GKIT_REDIS_ADDR"); addr != "" {
		t.Run("standalone", func(t *testing.T) {
			client := redis.NewClient(&redis.Options{Addr: addr})
			t.Cleanup(func() { _ = client.Close() })
			runDurableChordRedisBoundedPagination(t, client)
		})
	}
	if raw := os.Getenv("GKIT_REDIS_CLUSTER_ADDRS"); raw != "" {
		t.Run("cluster", func(t *testing.T) {
			addrs := strings.Split(raw, ",")
			if len(addrs) != 3 {
				t.Fatalf("GKIT_REDIS_CLUSTER_ADDRS has %d nodes, want 3", len(addrs))
			}
			client := redis.NewClusterClient(&redis.ClusterOptions{
				Addrs: addrs,
				// Redis 7 adds endpoint metadata to CLUSTER SLOTS that go-redis v8
				// predates. The test topology is fixed, so provide the authoritative
				// slot map and still exercise real Redis 7 cluster routing/MOVED
				// semantics for every indexed scan and record operation.
				ClusterSlots: func(context.Context) ([]redis.ClusterSlot, error) {
					return []redis.ClusterSlot{
						{Start: 0, End: 5460, Nodes: []redis.ClusterNode{{Addr: addrs[0]}}},
						{Start: 5461, End: 10922, Nodes: []redis.ClusterNode{{Addr: addrs[1]}}},
						{Start: 10923, End: 16383, Nodes: []redis.ClusterNode{{Addr: addrs[2]}}},
					}, nil
				},
			})
			t.Cleanup(func() { _ = client.Close() })
			runDurableChordRedisBoundedPagination(t, client)
		})
	}
	if os.Getenv("GKIT_REDIS_ADDR") == "" && os.Getenv("GKIT_REDIS_CLUSTER_ADDRS") == "" {
		t.Skip("GKIT_REDIS_ADDR or GKIT_REDIS_CLUSTER_ADDRS is required")
	}
}

func runDurableChordRedisBoundedPagination(t *testing.T, client redis.UniversalClient) {
	t.Helper()
	ctx := context.Background()
	flushIssue104Redis(t, client)
	b := NewBackendRedis(client, -1).(*BackendRedis)
	const records = 240
	states := make(map[string]backend.ChordCallbackState)
	for index := 0; index < records; index++ {
		registration := issue104PaginationRegistration(t, fmt.Sprintf("pagination-group-%03d", index))
		ref, err := b.RegisterChord(ctx, registration)
		if err != nil {
			t.Fatalf("register %d: %v", index, err)
		}
		switch index {
		case 1:
			if err := b.ReconcileChord(ctx, ref.DeliveryKey); err != nil {
				t.Fatal(err)
			}
		case 2:
			if err := b.ReconcileChord(ctx, ref.DeliveryKey); err != nil {
				t.Fatal(err)
			}
			lease, claimed, err := b.ClaimMemberPublication(ctx, backend.ChordMemberClaim{DeliveryKey: ref.DeliveryKey, Ordinal: 0, Owner: "pagination", Now: time.Now()})
			if err != nil || !claimed {
				t.Fatalf("claim unknown fixture = %t, %v", claimed, err)
			}
			if err := b.RecordMemberPublishOutcome(ctx, lease, backend.ChordPublishOutcome{Kind: backend.ChordPublishOutcomeUnknown, Now: time.Now(), ConfirmationDeadline: time.Now().Add(time.Hour)}); err != nil {
				t.Fatal(err)
			}
		case 3:
			if err := b.ReconcileChord(ctx, ref.DeliveryKey); err != nil {
				t.Fatal(err)
			}
			if err := b.RecordMemberTerminal(ctx, ref.DeliveryKey, 0, registration.Members[0].TaskID, backend.MemberTerminalFailure, nil); err != nil {
				t.Fatal(err)
			}
		}
	}

	hook := &issue104CommandCountHook{}
	client.AddHook(hook)
	cursor := ""
	totalGets := 0
	seen := make(map[string]backend.ChordDelivery)
	pages := 0
	for {
		hook.reset()
		page, err := b.ScanChordDeliveries(ctx, backend.ChordScan{Cursor: cursor, Limit: 12, Now: time.Now()})
		if err != nil {
			t.Fatal(err)
		}
		pages++
		pageGets := hook.count()
		totalGets += pageGets
		if pageGets > 13 {
			t.Fatalf("page %d GET count = %d, want <= limit+1", pages, pageGets)
		}
		for _, delivery := range page.Deliveries {
			if _, duplicate := seen[delivery.DeliveryKey]; duplicate {
				t.Fatalf("duplicate delivery %s", delivery.DeliveryKey)
			}
			seen[delivery.DeliveryKey] = delivery
			states[delivery.DeliveryKey] = delivery.CallbackState
		}
		if page.NextCursor == "" {
			break
		}
		if strings.Contains(page.NextCursor, "chord:v1:") {
			t.Fatalf("cursor exposes raw delivery key: %q", page.NextCursor)
		}
		cursor = page.NextCursor
	}
	if len(seen) != records {
		t.Fatalf("scanned deliveries = %d, want %d", len(seen), records)
	}
	if pages < 2 {
		t.Fatalf("pages = %d, want multiple pages", pages)
	}
	if totalGets > records+pages {
		t.Fatalf("total GET count = %d, want O(N) <= %d", totalGets, records+pages)
	}
	selected := map[backend.ChordCallbackState]bool{}
	for _, state := range states {
		selected[state] = true
	}
	if !selected[backend.ChordWaiting] || !selected[backend.ChordSuppressed] {
		t.Fatalf("selected states = %#v, want WAITING and SUPPRESSED", selected)
	}
}

func issue104PaginationRegistration(t *testing.T, groupID string) backend.ChordRegistration {
	t.Helper()
	callbackID := "pagination-callback"
	deliveryKey := backend.ChordDeliveryKey(groupID, callbackID)
	callback := task.NewSignature(callbackID, "callback")
	callback.Meta.Set(backend.DurableChordDeliveryKeyMeta, deliveryKey)
	callbackPayload, err := json.Marshal(callback)
	if err != nil {
		t.Fatal(err)
	}
	member := task.NewSignature(groupID+"-member", "member")
	member.GroupID = groupID
	member.Meta.Set(backend.DurableChordDeliveryKeyMeta, deliveryKey)
	member.Meta.Set(backend.DurableChordMemberMeta, true)
	member.Meta.Set(backend.DurableChordMemberOrdinal, 0)
	memberPayload, err := json.Marshal(member)
	if err != nil {
		t.Fatal(err)
	}
	registration := backend.ChordRegistration{
		GroupID:   groupID,
		GroupName: groupID,
		Retention: -1,
		Callback:  callbackPayload,
		Members:   []backend.ChordMemberRegistration{{Ordinal: 0, TaskID: member.ID, Payload: memberPayload}},
	}
	if err := backend.FinalizeChordRegistration(&registration); err != nil {
		t.Fatal(err)
	}
	return registration
}

func flushIssue104Redis(t *testing.T, client redis.UniversalClient) {
	t.Helper()
	ctx := context.Background()
	if cluster, ok := client.(*redis.ClusterClient); ok {
		if err := cluster.ForEachMaster(ctx, func(ctx context.Context, master *redis.Client) error {
			return master.FlushDB(ctx).Err()
		}); err != nil {
			t.Fatal(err)
		}
		return
	}
	if err := client.FlushDB(ctx).Err(); err != nil {
		t.Fatal(err)
	}
}
