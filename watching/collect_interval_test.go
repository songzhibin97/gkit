package watching

import (
	"testing"
	"time"
)

func TestWithCollectIntervalRejectsInvalidDurations(t *testing.T) {
	tests := []struct {
		name     string
		interval string
	}{
		{name: "parse error", interval: "invalid"},
		{name: "zero", interval: "0s"},
		{name: "negative", interval: "-1s"},
	}

	for _, tt := range tests {
		t.Run(tt.name+" keeps default", func(t *testing.T) {
			w := NewWatching(WithCollectInterval(tt.interval))
			if got := w.config.CollectInterval; got != defaultInterval {
				t.Fatalf("CollectInterval = %v, want default %v", got, defaultInterval)
			}
			assertNoIntervalReset(t, w)
		})

		t.Run(tt.name+" keeps current", func(t *testing.T) {
			const current = 2 * time.Second
			w := NewWatching(WithCollectInterval(current.String()))
			requireIntervalReset(t, w)

			WithCollectInterval(tt.interval)(w)
			if got := w.config.CollectInterval; got != current {
				t.Fatalf("CollectInterval = %v, want current %v", got, current)
			}
			assertNoIntervalReset(t, w)
		})
	}
}

func TestWithCollectIntervalAcceptsPositiveDuration(t *testing.T) {
	const interval = 250 * time.Millisecond
	w := NewWatching(WithCollectInterval(interval.String()))
	if got := w.config.CollectInterval; got != interval {
		t.Fatalf("CollectInterval = %v, want %v", got, interval)
	}
	requireIntervalReset(t, w)
}

func requireIntervalReset(t *testing.T, w *Watching) {
	t.Helper()
	select {
	case <-w.config.intervalResetting:
	default:
		t.Fatal("positive interval did not request a ticker reset")
	}
}

func assertNoIntervalReset(t *testing.T, w *Watching) {
	t.Helper()
	select {
	case <-w.config.intervalResetting:
		t.Fatal("invalid interval requested a ticker reset")
	default:
	}
}
