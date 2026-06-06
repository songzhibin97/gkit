package distributed

import (
	"testing"
	"time"
)

// TestTimedTaskLockTTL_FloorAndPassthrough covers I-y: the previous code
// passed `int(time.Until(next).Milliseconds())` directly to PEXPIRE, which
// could be negative (next already past) or zero, both of which Redis
// rejects — causing every subsequent fire to skip.
func TestTimedTaskLockTTL_FloorAndPassthrough(t *testing.T) {
	cases := []struct {
		name string
		d    time.Duration
		want int
	}{
		{"negative", -5 * time.Second, minLockTTLMs},
		{"zero", 0, minLockTTLMs},
		{"sub-floor", 50 * time.Millisecond, minLockTTLMs},
		{"at-floor", 100 * time.Millisecond, 100},
		{"comfortable", 5 * time.Second, 5000},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := timedTaskLockTTL(c.d); got != c.want {
				t.Fatalf("timedTaskLockTTL(%v) = %d, want %d", c.d, got, c.want)
			}
		})
	}
}
