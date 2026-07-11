package codel

import (
	"context"
	"sync/atomic"
	"testing"
	"time"
)

const decisionMargin = int64(1_000_000)

func TestQueueJudgeTargetAndIntervalThresholds(t *testing.T) {
	t.Run("below target clears first-above time", func(t *testing.T) {
		q := NewQueue(SetTarget(decisionMargin), SetInternal(decisionMargin))
		atomic.StoreInt64(&q.faTime, nowMillis()+decisionMargin)

		if drop := q.judge(packet{ts: nowMillis()}); drop {
			t.Fatal("judge() dropped a packet below target")
		}
		if got := atomic.LoadInt64(&q.faTime); got != 0 {
			t.Fatalf("faTime after packet below target = %d, want 0", got)
		}
	})

	t.Run("first packet above target starts interval", func(t *testing.T) {
		q := NewQueue(SetTarget(decisionMargin), SetInternal(decisionMargin))
		before := nowMillis()
		if drop := q.judge(packet{ts: 0}); drop {
			t.Fatal("judge() dropped the first packet above target")
		}
		after := nowMillis()
		faTime := atomic.LoadInt64(&q.faTime)
		if faTime < before+decisionMargin || faTime > after+decisionMargin {
			t.Fatalf("faTime = %d, want within [%d, %d]", faTime, before+decisionMargin, after+decisionMargin)
		}
		if q.dropping {
			t.Fatal("queue entered dropping before the interval elapsed")
		}
	})

	t.Run("packet before interval deadline is allowed", func(t *testing.T) {
		q := NewQueue(SetTarget(decisionMargin), SetInternal(decisionMargin))
		atomic.StoreInt64(&q.faTime, nowMillis()+decisionMargin)

		if drop := q.judge(packet{ts: 0}); drop {
			t.Fatal("judge() dropped a packet before faTime")
		}
		if q.dropping {
			t.Fatal("queue entered dropping before faTime")
		}
	})
}

func TestQueueJudgeDroppingLifecycleAndStat(t *testing.T) {
	q := NewQueue(SetTarget(decisionMargin), SetInternal(decisionMargin))
	atomic.StoreInt64(&q.faTime, nowMillis()-decisionMargin-1_000)

	before := nowMillis()
	if drop := q.judge(packet{ts: 0}); !drop {
		t.Fatal("judge() allowed a packet after the interval elapsed")
	}
	after := nowMillis()
	if !q.dropping {
		t.Fatal("queue did not enter dropping after the interval elapsed")
	}
	if got := atomic.LoadInt64(&q.count); got != 1 {
		t.Fatalf("drop count on entry = %d, want 1", got)
	}
	dropNext := atomic.LoadInt64(&q.dropNext)
	if dropNext < before+decisionMargin || dropNext > after+decisionMargin {
		t.Fatalf("dropNext on entry = %d, want within [%d, %d]", dropNext, before+decisionMargin, after+decisionMargin)
	}

	q.packets <- packet{ch: make(chan bool, 1)}
	stat := q.Stat()
	if !stat.Dropping || stat.Packets != 1 || stat.FaTime != atomic.LoadInt64(&q.faTime) || stat.DropNext != dropNext {
		t.Fatalf("Stat() = %+v, want dropping state, one packet, faTime %d, dropNext %d", stat, q.faTime, dropNext)
	}
	<-q.packets

	if drop := q.judge(packet{ts: nowMillis()}); drop {
		t.Fatal("judge() dropped a packet below target while leaving dropping")
	}
	if q.dropping {
		t.Fatal("queue did not leave dropping after a packet below target")
	}
	if got := atomic.LoadInt64(&q.faTime); got != 0 {
		t.Fatalf("faTime after leaving dropping = %d, want 0", got)
	}
	if got := atomic.LoadInt64(&q.count); got != 1 {
		t.Fatalf("drop count after leaving dropping = %d, want 1", got)
	}
}

func TestQueueJudgeDropScheduleAdvancesCount(t *testing.T) {
	t.Run("not due", func(t *testing.T) {
		q := queueInDroppingState(7, nowMillis()+decisionMargin)
		dropNext := atomic.LoadInt64(&q.dropNext)

		if drop := q.judge(packet{ts: 0}); !drop {
			t.Fatal("judge() allowed an above-target packet while dropping")
		}
		if got := atomic.LoadInt64(&q.count); got != 7 {
			t.Fatalf("drop count before dropNext = %d, want 7", got)
		}
		if got := atomic.LoadInt64(&q.dropNext); got != dropNext {
			t.Fatalf("dropNext changed before it was due: got %d, want %d", got, dropNext)
		}
	})

	t.Run("due", func(t *testing.T) {
		oldDropNext := nowMillis() - 1
		q := queueInDroppingState(1, oldDropNext)

		if drop := q.judge(packet{ts: 0}); !drop {
			t.Fatal("judge() allowed a due above-target packet while dropping")
		}
		if got := atomic.LoadInt64(&q.count); got != 2 {
			t.Fatalf("drop count after due drop = %d, want 2", got)
		}
		if got := atomic.LoadInt64(&q.dropNext); got <= oldDropNext {
			t.Fatalf("dropNext after due drop = %d, want greater than %d", got, oldDropNext)
		}
	})
}

func queueInDroppingState(count, dropNext int64) *Queue {
	q := NewQueue(SetTarget(decisionMargin), SetInternal(decisionMargin))
	q.dropping = true
	atomic.StoreInt64(&q.count, count)
	atomic.StoreInt64(&q.faTime, nowMillis()-decisionMargin-1_000)
	atomic.StoreInt64(&q.dropNext, dropNext)
	return q
}

func nowMillis() int64 {
	return time.Now().UnixNano() / int64(time.Millisecond)
}

func BenchmarkAQM(b *testing.B) {
	q := Default()
	b.RunParallel(func(p *testing.PB) {
		for p.Next() {
			ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(time.Millisecond*5))
			err := q.Push(ctx)
			if err == nil {
				q.Pop()
			}
			cancel()
		}
	})
}
