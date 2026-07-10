package codel

import (
	"context"
	"errors"
	"runtime"
	"sync"
	"testing"
	"time"
)

type pushGateContext struct {
	entered chan struct{}
	release chan struct{}
	done    chan struct{}
	once    sync.Once
}

func newPushGateContext() *pushGateContext {
	return &pushGateContext{
		entered: make(chan struct{}),
		release: make(chan struct{}),
		done:    make(chan struct{}),
	}
}

func (*pushGateContext) Deadline() (time.Time, bool) { return time.Time{}, false }

func (c *pushGateContext) Done() <-chan struct{} {
	c.once.Do(func() { close(c.entered) })
	<-c.release
	return c.done
}

func (c *pushGateContext) Err() error {
	select {
	case <-c.done:
		return context.Canceled
	default:
		return nil
	}
}

func (*pushGateContext) Value(interface{}) interface{} { return nil }

func TestQueueDecisionBufferedBeforePushWaits(t *testing.T) {
	q := NewQueue(SetTarget(1<<60), SetInternal(1<<60))
	ctx := newPushGateContext()
	result := make(chan error, 1)
	go func() { result <- q.Push(ctx) }()

	select {
	case <-ctx.entered:
	case <-time.After(250 * time.Millisecond):
		t.Fatal("Push did not enqueue and reach the pre-select gate")
	}

	popWithWatchdog(t, q)
	close(ctx.release)
	select {
	case err := <-result:
		if err != nil {
			t.Fatalf("Push returned %v, want the buffered allow decision", err)
		}
	case <-time.After(250 * time.Millisecond):
		close(ctx.done)
		<-result
		t.Fatal("Pop lost its decision before Push began receiving")
	}
}

func TestQueueCanceledPushDoesNotBlockOrCrossWire(t *testing.T) {
	previousProcs := runtime.GOMAXPROCS(1)
	defer runtime.GOMAXPROCS(previousProcs)

	q := NewQueue(SetTarget(1<<60), SetInternal(1<<60))
	firstCtx, cancelFirst := context.WithCancel(context.Background())
	firstResult := make(chan error, 1)
	go func() { firstResult <- q.Push(firstCtx) }()
	firstPacket := takePacket(t, q)
	q.packets <- firstPacket
	cancelFirst()
	if err := awaitPushResult(t, firstResult); !errors.Is(err, context.Canceled) {
		t.Fatalf("first Push error = %v, want context.Canceled", err)
	}

	// The first Push is gone before Pop decides. Its one-shot buffered channel
	// must absorb the decision without blocking and must never be reused.
	popWithWatchdog(t, q)

	secondCtx, cancelSecond := context.WithCancel(context.Background())
	defer cancelSecond()
	secondResult := make(chan error, 1)
	go func() { secondResult <- q.Push(secondCtx) }()
	secondPacket := takePacket(t, q)
	if secondPacket.ch == firstPacket.ch || cap(secondPacket.ch) != 1 {
		cancelSecond()
		_ = awaitPushResult(t, secondResult)
		t.Fatalf(
			"second Push decision channel = %p (cap=%d), first = %p; want independent buffered channels",
			secondPacket.ch,
			cap(secondPacket.ch),
			firstPacket.ch,
		)
	}

	q.packets <- secondPacket
	popWithWatchdog(t, q)
	if err := awaitPushResult(t, secondResult); err != nil {
		t.Fatalf("second Push received another request's decision: %v", err)
	}
}

func takePacket(t *testing.T, q *Queue) packet {
	t.Helper()
	select {
	case p := <-q.packets:
		return p
	case <-time.After(250 * time.Millisecond):
		t.Fatal("Push did not enqueue a packet")
		return packet{}
	}
}

func popWithWatchdog(t *testing.T, q *Queue) {
	t.Helper()
	done := make(chan struct{})
	go func() {
		q.Pop()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(250 * time.Millisecond):
		t.Fatal("Pop blocked while handing off a decision")
	}
}

func awaitPushResult(t *testing.T, result <-chan error) error {
	t.Helper()
	select {
	case err := <-result:
		return err
	case <-time.After(250 * time.Millisecond):
		t.Fatal("Push did not return")
		return nil
	}
}
