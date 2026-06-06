package bbr

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/songzhibin97/gkit/internal/stat"
	"github.com/songzhibin97/gkit/overload"
)

// TestAllow_OnlySuccessFeedsStats guards the Op-gate in the release callback:
// only an overload.Success outcome feeds the RT distribution (rtStat) and the
// pass counter (passStat). A Drop / fail-fast outcome's RT must NOT enter
// rtStat, otherwise that sample biases minRT and therefore maxFlight. inFlight,
// by contrast, must ALWAYS be released regardless of outcome.
//
// Asserting on rtStat/passStat directly (rather than minRT()/maxPASS()) avoids
// the rolling-window's "current bucket" quirk: minRT's reduce loop stops one
// bucket short of the freshly-written sample, so a single new sample never
// shows up through Stat(). Reduce(stat.Sum/Count) reads every bucket.
//
// Reverting `if do.Op == overload.Success` makes the Drop case feed rtStat /
// passStat and trips the first assertions.
func TestAllow_OnlySuccessFeedsStats(t *testing.T) {
	const rt = 20 * time.Millisecond

	// A Drop outcome: inFlight released, but rtStat/passStat untouched.
	dropLimiter := NewGroup().Get("k").(*BBR)
	done, err := dropLimiter.Allow(context.Background())
	if err != nil {
		t.Fatalf("Allow (drop case): %v", err)
	}
	time.Sleep(rt)
	done(overload.DoneInfo{Op: overload.Drop})

	if got := atomic.LoadInt64(&dropLimiter.inFlight); got != 0 {
		t.Fatalf("inFlight = %d after a Drop outcome; want 0 (inFlight must always be released)", got)
	}
	if got := dropLimiter.rtStat.Reduce(stat.Sum); got != 0 {
		t.Fatalf("rtStat sum = %v after a Drop outcome; want 0 (a non-Success outcome must not feed rtStat)", got)
	}
	if got := dropLimiter.passStat.Reduce(stat.Count); got != 0 {
		t.Fatalf("passStat count = %v after a Drop outcome; want 0 (a non-Success outcome must not feed passStat)", got)
	}

	// A Success outcome with the same RT feeds both counters.
	okLimiter := NewGroup().Get("k").(*BBR)
	done, err = okLimiter.Allow(context.Background())
	if err != nil {
		t.Fatalf("Allow (success case): %v", err)
	}
	time.Sleep(rt)
	done(overload.DoneInfo{Op: overload.Success})

	if got := atomic.LoadInt64(&okLimiter.inFlight); got != 0 {
		t.Fatalf("inFlight = %d after a Success outcome; want 0", got)
	}
	if got := okLimiter.rtStat.Reduce(stat.Sum); got <= 0 {
		t.Fatalf("rtStat sum = %v after a Success outcome; want > 0 (Success must feed rtStat)", got)
	}
	if got := okLimiter.passStat.Reduce(stat.Count); got != 1 {
		t.Fatalf("passStat count = %v after a Success outcome; want 1 (Success must feed passStat)", got)
	}
}
