package bbr

import "testing"

// TestNewLimiter_RecoversFromBadConfig covers I-n: previously
// SetWinBucket(0) or a window/winBucket combination yielding zero
// bucketDuration crashed newLimiter with integer divide-by-zero. Now we
// fall back to defaults.
func TestNewLimiter_RecoversFromBadConfig(t *testing.T) {
	cases := []struct {
		name string
		opts []func()
	}{
		{"winBucket=0", []func(){}},
	}
	_ = cases
	// We can't easily craft options in this scope; instead, exercise the
	// internal newLimiter directly via SetWinBucket/SetWindow.
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("newLimiter panicked on bad config: %v", r)
		}
	}()
	_ = newLimiter(SetWinBucket(0))
	_ = newLimiter(SetWindow(0))
}
