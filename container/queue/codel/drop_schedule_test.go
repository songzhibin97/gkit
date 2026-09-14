package codel

import (
	"context"
	"math"
	"sync/atomic"
	"testing"

	"github.com/songzhibin97/gkit/overload/bbr"
)

func TestQueueJudgeExactDropDeadline(t *testing.T) {
	const deadline = int64(2_000_000)
	for _, test := range []struct {
		name     string
		now      int64
		wantDrop bool
	}{
		{name: "before", now: deadline - 1},
		{name: "equal", now: deadline, wantDrop: true},
		{name: "after", now: deadline + 1, wantDrop: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			q := queueInDroppingState(1, deadline)
			atomic.StoreInt64(&q.faTime, 1)
			if drop := q.judgeAt(packet{ts: 0}, test.now); drop != test.wantDrop {
				t.Errorf("judgeAt(deadline%+d) = %t, want %t", test.now-deadline, drop, test.wantDrop)
			}
			wantCount, wantNext := int64(1), deadline
			if test.wantDrop {
				wantCount = 2
				wantNext += controlLawOffset(q.conf.internal, wantCount)
			}
			if got := atomic.LoadInt64(&q.count); got != wantCount {
				t.Errorf("count = %d, want %d", got, wantCount)
			}
			if got := q.Stat(); !got.Dropping || got.DropNext != wantNext || got.FaTime != 1 {
				t.Errorf("Stat = %+v, want Dropping=true, DropNext=%d, FaTime=1", got, wantNext)
			}
		})
	}
}

// Regression for #165, item 02-01: being eligible to drop is not a drop
// decision before dropNext. Verify the decision reaches the original Push.
func TestQueuePushPopRespectsDropSchedule(t *testing.T) {
	for _, test := range []struct {
		name         string
		dropNext     int64
		belowTarget  bool
		wantErr      error
		wantCount    int64
		wantDropping bool
	}{
		{name: "before_drop_next", dropNext: math.MaxInt64, wantCount: 1, wantDropping: true},
		{name: "due_drop", dropNext: 1, wantErr: bbr.LimitExceed, wantCount: 2, wantDropping: true},
		{name: "below_target_exits_dropping", dropNext: math.MaxInt64, belowTarget: true, wantCount: 1},
	} {
		t.Run(test.name, func(t *testing.T) {
			q := queueInDroppingState(1, test.dropNext)
			firstAbove := atomic.LoadInt64(&q.faTime)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			result := make(chan error, 1)
			go func() { result <- q.Push(ctx) }()
			p := takePacket(t, q)
			// Retain Push's decision channel. Set only the sojourn timestamp
			// while exclusively holding this dequeued packet, without waiting.
			p.ts = 0
			if test.belowTarget {
				p.ts = nowMillis()
			}
			q.packets <- p
			popWithWatchdog(t, q)
			if err := awaitPushResult(t, result); err != test.wantErr {
				t.Errorf("Push decision = %v, want %v", err, test.wantErr)
			}
			if got := atomic.LoadInt64(&q.count); got != test.wantCount {
				t.Errorf("drop count = %d, want %d", got, test.wantCount)
			}
			wantDropNext := test.dropNext
			if test.wantErr == bbr.LimitExceed {
				wantDropNext += controlLawOffset(q.conf.internal, test.wantCount)
			}
			stat := q.Stat()
			if stat.DropNext != wantDropNext || stat.Dropping != test.wantDropping || stat.Packets != 0 {
				t.Errorf("Stat = %+v, want DropNext=%d, Dropping=%t, Packets=0", stat, wantDropNext, test.wantDropping)
			}
			if test.belowTarget {
				firstAbove = 0
			}
			if stat.FaTime != firstAbove {
				t.Errorf("FaTime = %d, want %d", stat.FaTime, firstAbove)
			}
		})
	}
}
