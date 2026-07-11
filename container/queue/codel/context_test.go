package codel

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestQueuePushPreCanceledContextDoesNotEnqueue(t *testing.T) {
	q := NewQueue(SetTarget(1<<60), SetInternal(1<<60))
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if err := q.Push(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("Push error = %v, want context.Canceled", err)
	}
	if got := q.Stat().Packets; got != 0 {
		t.Fatalf("queued packets after pre-canceled Push = %d, want 0", got)
	}

	result := make(chan error, 1)
	go func() { result <- q.Push(context.Background()) }()
	waitForPackets(t, q, 1)
	q.Pop()
	if err := awaitPushResult(t, result); err != nil {
		t.Fatalf("live Push after pre-canceled Push returned %v", err)
	}
	if got := q.Stat().Packets; got != 0 {
		t.Fatalf("queued packets after live Push/Pop = %d, want 0", got)
	}
}

func TestQueuePushCanceledAfterEnqueueKeepsDecisionIsolated(t *testing.T) {
	q := NewQueue(SetTarget(1<<60), SetInternal(1<<60))
	ctx, cancel := context.WithCancel(context.Background())
	result := make(chan error, 1)
	go func() { result <- q.Push(ctx) }()

	waitForPackets(t, q, 1)
	cancel()
	if err := awaitPushResult(t, result); !errors.Is(err, context.Canceled) {
		t.Fatalf("Push error = %v, want context.Canceled", err)
	}
	if got := q.Stat().Packets; got != 1 {
		t.Fatalf("queued packets after post-enqueue cancellation = %d, want 1", got)
	}

	// The canceled caller has left, but its buffered one-shot decision must let
	// Pop drain the request without blocking.
	popWithWatchdog(t, q)
	if got := q.Stat().Packets; got != 0 {
		t.Fatalf("queued packets after draining canceled request = %d, want 0", got)
	}
}

func waitForPackets(t *testing.T, q *Queue, want int64) {
	t.Helper()
	deadline := time.Now().Add(250 * time.Millisecond)
	for time.Now().Before(deadline) {
		if q.Stat().Packets == want {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("queued packets = %d, want %d", q.Stat().Packets, want)
}
