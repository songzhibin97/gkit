package backend

import (
	"math"
	"testing"
	"time"
)

func TestChordTerminalRetentionBounds(t *testing.T) {
	now := time.Date(2026, 9, 13, 0, 0, 0, 123, time.UTC)
	maxSeconds := int64(math.MaxInt64 / int64(time.Second))
	for _, seconds := range []int64{-1, 0, 1, maxSeconds, maxSeconds + 1, math.MaxInt64} {
		d := ChordDelivery{Retention: seconds}
		if err := ApplyChordCallbackTerminal(&d, CallbackTerminalSuccess, now); err != nil {
			t.Fatal(err)
		}
		if seconds < 0 {
			if d.TerminalExpireAt != nil {
				t.Fatal("negative retention expires")
			}
			continue
		}
		want := seconds
		if want == 0 {
			want = DefaultChordRetentionSeconds
		}
		if want > maxSeconds {
			want = maxSeconds
		}
		if d.TerminalExpireAt == nil || !d.TerminalExpireAt.Equal(now.Add(time.Duration(want)*time.Second)) {
			t.Fatalf("retention %d: expiry %v", seconds, d.TerminalExpireAt)
		}
	}
}
