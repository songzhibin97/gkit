package distributed

import (
	"context"
	"errors"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	resultpkg "github.com/songzhibin97/gkit/distributed/backend/result"
	"github.com/songzhibin97/gkit/distributed/task"
)

type groupTestBackend struct {
	mu              sync.Mutex
	pendingErrByID  map[string]error
	pendingIDs      []string
	resetTaskCalls  [][]string
	resetGroupCalls [][]string
	resetTaskErr    error
	resetGroupErr   error
}

func (*groupTestBackend) GroupTakeOver(string, string, ...string) error  { return nil }
func (*groupTestBackend) GroupCompleted(string) (bool, error)            { return false, nil }
func (*groupTestBackend) GroupTaskStatus(string) ([]*task.Status, error) { return nil, nil }
func (*groupTestBackend) TriggerCompleted(string) (bool, error)          { return false, nil }
func (b *groupTestBackend) SetStatePending(signature *task.Signature) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if err := b.pendingErrByID[signature.ID]; err != nil {
		return err
	}
	b.pendingIDs = append(b.pendingIDs, signature.ID)
	return nil
}
func (*groupTestBackend) SetStateReceived(*task.Signature) error                { return nil }
func (*groupTestBackend) SetStateStarted(*task.Signature) error                 { return nil }
func (*groupTestBackend) SetStateRetry(*task.Signature) error                   { return nil }
func (*groupTestBackend) SetStateSuccess(*task.Signature, []*task.Result) error { return nil }
func (*groupTestBackend) SetStateFailure(*task.Signature, string) error         { return nil }
func (*groupTestBackend) GetStatus(string) (*task.Status, error)                { return nil, nil }
func (b *groupTestBackend) ResetTask(ids ...string) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.resetTaskCalls = append(b.resetTaskCalls, append([]string(nil), ids...))
	return b.resetTaskErr
}
func (b *groupTestBackend) ResetGroup(ids ...string) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.resetGroupCalls = append(b.resetGroupCalls, append([]string(nil), ids...))
	return b.resetGroupErr
}
func (*groupTestBackend) SetResultExpire(int64) {}

type groupTestController struct {
	publishFn     func(context.Context, *task.Signature) error
	publishCalled chan struct{}
	publishCount  atomic.Int64
	active        atomic.Int64
	maxActive     atomic.Int64
}

func (*groupTestController) RegisterTask(...string)                           {}
func (*groupTestController) IsRegisterTask(string) bool                       { return true }
func (*groupTestController) StartConsuming(int, task.Processor) (bool, error) { return false, nil }
func (*groupTestController) StopConsuming()                                   {}
func (c *groupTestController) Publish(ctx context.Context, signature *task.Signature) error {
	c.publishCount.Add(1)
	if c.publishCalled != nil {
		select {
		case c.publishCalled <- struct{}{}:
		default:
		}
	}
	active := c.active.Add(1)
	defer c.active.Add(-1)
	for {
		current := c.maxActive.Load()
		if active <= current || c.maxActive.CompareAndSwap(current, active) {
			break
		}
	}
	if c.publishFn != nil {
		return c.publishFn(ctx, signature)
	}
	return nil
}
func (*groupTestController) GetPendingTasks(string) ([]*task.Signature, error) { return nil, nil }
func (*groupTestController) GetDelayedTasks() ([]*task.Signature, error)       { return nil, nil }
func (*groupTestController) SetConsumingQueue(string)                          {}
func (*groupTestController) SetDelayedQueue(string)                            {}

type groupSendOutcome struct {
	results []*resultpkg.AsyncResult
	err     error
}

func newIssue79Group(ids ...string) *task.Group {
	signatures := make([]*task.Signature, 0, len(ids))
	for _, id := range ids {
		signatures = append(signatures, &task.Signature{ID: id, Name: "task"})
	}
	group, _ := task.NewGroup("group-id", "group", signatures...)
	return group
}

func sendGroupAsync(server *Server, ctx context.Context, group *task.Group, concurrency int) <-chan groupSendOutcome {
	done := make(chan groupSendOutcome, 1)
	go func() {
		results, err := server.SendGroupWithContext(ctx, group, concurrency)
		done <- groupSendOutcome{results: results, err: err}
	}()
	return done
}

func TestSendGroupZeroConcurrencyUsesOnePublisher(t *testing.T) {
	backend := &groupTestBackend{}
	controller := &groupTestController{publishFn: func(context.Context, *task.Signature) error {
		time.Sleep(time.Millisecond)
		return nil
	}}
	server := &Server{backend: backend, controller: controller}
	done := sendGroupAsync(server, context.Background(), newIssue79Group("t0", "t1", "t2"), 0)

	select {
	case outcome := <-done:
		if outcome.err != nil {
			t.Fatalf("SendGroupWithContext returned error: %v", outcome.err)
		}
		if len(outcome.results) != 3 {
			t.Fatalf("result count = %d, want 3", len(outcome.results))
		}
		for i, result := range outcome.results {
			if result == nil {
				t.Fatalf("result[%d] is nil", i)
			}
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("SendGroupWithContext blocked with concurrency 0")
	}
	if got := controller.maxActive.Load(); got != 1 {
		t.Fatalf("max active publishers = %d, want 1", got)
	}
}

func TestSendGroupPendingFailureDoesNotPublishAndRollsBack(t *testing.T) {
	pendingErr := errors.New("pending failed")
	resetTaskErr := errors.New("reset task failed")
	resetGroupErr := errors.New("reset group failed")
	backend := &groupTestBackend{
		pendingErrByID: map[string]error{"t1": pendingErr},
		resetTaskErr:   resetTaskErr,
		resetGroupErr:  resetGroupErr,
	}
	publishCalled := make(chan struct{}, 1)
	controller := &groupTestController{publishCalled: publishCalled}
	server := &Server{backend: backend, controller: controller}

	_, err := server.SendGroupWithContext(context.Background(), newIssue79Group("t0", "t1", "t2"), 2)
	if !errors.Is(err, pendingErr) {
		t.Fatalf("error = %v, want pending failure as cause", err)
	}
	if !errors.Is(err, resetTaskErr) {
		t.Fatalf("error = %v, want reset task failure as cause", err)
	}
	if !errors.Is(err, resetGroupErr) {
		t.Fatalf("error = %v, want reset group failure as cause", err)
	}
	if !strings.Contains(err.Error(), "set state pending task t1") ||
		!strings.Contains(err.Error(), "reset task failed") ||
		!strings.Contains(err.Error(), "reset group failed") {
		t.Fatalf("error lacks operation or cleanup context: %v", err)
	}
	if got := controller.publishCount.Load(); got != 0 {
		t.Fatalf("publish count = %d, want 0", got)
	}
	select {
	case <-publishCalled:
		t.Fatal("Publish was called after pending initialization failed")
	default:
	}
	backend.mu.Lock()
	defer backend.mu.Unlock()
	if len(backend.resetTaskCalls) != 1 || len(backend.resetTaskCalls[0]) != 1 || backend.resetTaskCalls[0][0] != "t0" {
		t.Fatalf("ResetTask calls = %#v, want [[t0]]", backend.resetTaskCalls)
	}
	if len(backend.resetGroupCalls) != 1 || len(backend.resetGroupCalls[0]) != 1 || backend.resetGroupCalls[0][0] != "group-id" {
		t.Fatalf("ResetGroup calls = %#v, want [[group-id]]", backend.resetGroupCalls)
	}
}

func TestSendGroupPublishErrorUsesTaskOrderNotCompletionOrder(t *testing.T) {
	err0 := errors.New("publish t0 failed")
	err1 := errors.New("publish t1 failed")
	release0 := make(chan struct{})
	t1Finished := make(chan struct{})
	controller := &groupTestController{publishFn: func(_ context.Context, signature *task.Signature) error {
		switch signature.ID {
		case "t0":
			<-release0
			time.Sleep(10 * time.Millisecond)
			return err0
		case "t1":
			close(t1Finished)
			return err1
		default:
			return nil
		}
	}}
	server := &Server{backend: &groupTestBackend{}, controller: controller}
	done := sendGroupAsync(server, context.Background(), newIssue79Group("t0", "t1"), 2)

	select {
	case <-t1Finished:
	case <-time.After(time.Second):
		t.Fatal("t1 publisher did not finish")
	}
	close(release0)
	outcome := <-done
	if !errors.Is(outcome.err, err0) {
		t.Fatalf("error = %v, want task-index-first t0 error", outcome.err)
	}
	if !strings.Contains(outcome.err.Error(), "publish task t0") {
		t.Fatalf("error = %v, want publish task t0 context", outcome.err)
	}
}

func TestSendGroupCancellationStopsAdmission(t *testing.T) {
	started0 := make(chan struct{})
	release0 := make(chan struct{})
	controller := &groupTestController{publishFn: func(_ context.Context, signature *task.Signature) error {
		if signature.ID == "t0" {
			close(started0)
			<-release0
		}
		return nil
	}}
	server := &Server{backend: &groupTestBackend{}, controller: controller}
	ctx, cancel := context.WithCancel(context.Background())
	done := sendGroupAsync(server, ctx, newIssue79Group("t0", "t1", "t2"), 1)
	<-started0
	cancel()
	if got := controller.publishCount.Load(); got != 1 {
		t.Fatalf("publish count before release = %d, want 1", got)
	}
	close(release0)
	outcome := <-done
	if !errors.Is(outcome.err, context.Canceled) {
		t.Fatalf("error = %v, want context.Canceled", outcome.err)
	}
	if got := controller.publishCount.Load(); got != 1 {
		t.Fatalf("publish count after cancellation = %d, want 1", got)
	}
	if outcome.results[0] == nil || outcome.results[1] != nil || outcome.results[2] != nil {
		t.Fatalf("results after cancellation = %#v, want only index 0 populated", outcome.results)
	}
}

func TestSendGroupCancellationJoinsStartedPublishers(t *testing.T) {
	started := make(chan struct{}, 2)
	release := make(chan struct{})
	controller := &groupTestController{publishFn: func(context.Context, *task.Signature) error {
		started <- struct{}{}
		<-release
		return nil
	}}
	server := &Server{backend: &groupTestBackend{}, controller: controller}
	ctx, cancel := context.WithCancel(context.Background())
	done := sendGroupAsync(server, ctx, newIssue79Group("t0", "t1"), 2)
	<-started
	<-started
	cancel()

	var outcome groupSendOutcome
	returnedEarly := false
	select {
	case outcome = <-done:
		returnedEarly = true
	case <-time.After(30 * time.Millisecond):
	}
	close(release)
	if !returnedEarly {
		outcome = <-done
	}
	deadline := time.Now().Add(time.Second)
	for controller.active.Load() != 0 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if returnedEarly {
		t.Fatal("SendGroupWithContext returned before its started publishers completed")
	}
	if !errors.Is(outcome.err, context.Canceled) {
		t.Fatalf("error = %v, want context.Canceled", outcome.err)
	}
	for i, result := range outcome.results {
		if result == nil {
			t.Fatalf("result[%d] is nil after joined successful publisher", i)
		}
	}
}
