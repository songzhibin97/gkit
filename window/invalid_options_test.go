package window

import (
	"errors"
	"testing"
	"time"

	"github.com/songzhibin97/gkit/options"
)

func TestNewWindowInvalidOptionsUseDefaults(t *testing.T) {
	tests := []struct {
		name string
		opts []options.Option
	}{
		{name: "zero size", opts: []options.Option{SetSize(0)}},
		{name: "zero interval", opts: []options.Option{SetInterval(0)}},
		{name: "negative interval", opts: []options.Option{SetInterval(-time.Second)}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := NewWindow(tt.opts...).(*Window)
			defer w.Shutdown()

			if w.size != 5 {
				t.Fatalf("size = %d, want default 5", w.size)
			}
			if w.interval != time.Second {
				t.Fatalf("interval = %v, want default %v", w.interval, time.Second)
			}
			if len(w.buffer) != 5 {
				t.Fatalf("buffer length = %d, want 5", len(w.buffer))
			}
			if cap(w.communication) != 5 {
				t.Fatalf("communication capacity = %d, want 5", cap(w.communication))
			}
		})
	}
}

func TestNewWindowValidOptionsUnchanged(t *testing.T) {
	w := NewWindow(SetSize(2), SetInterval(time.Hour)).(*Window)
	defer w.Shutdown()

	if w.size != 2 {
		t.Fatalf("size = %d, want 2", w.size)
	}
	if w.interval != time.Hour {
		t.Fatalf("interval = %v, want %v", w.interval, time.Hour)
	}
	if len(w.buffer) != 2 {
		t.Fatalf("buffer length = %d, want 2", len(w.buffer))
	}
	if cap(w.communication) != 2 {
		t.Fatalf("communication capacity = %d, want 2", cap(w.communication))
	}
}

func TestNewLeapArrayRejectsZeroDimensions(t *testing.T) {
	tests := []struct {
		name         string
		n            uint64
		intervalSize uint64
	}{
		{name: "zero bucket count", n: 0, intervalSize: IntervalSize},
		{name: "zero interval", n: N, intervalSize: 0},
		{name: "both zero", n: 0, intervalSize: 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NewLeapArray(tt.n, tt.intervalSize, &Mock{})
			if got != nil {
				t.Fatalf("NewLeapArray(%d, %d) = %#v, want nil", tt.n, tt.intervalSize, got)
			}
			if !errors.Is(err, ErrWindowNotSegmentation) {
				t.Fatalf("NewLeapArray(%d, %d) error = %v, want ErrWindowNotSegmentation", tt.n, tt.intervalSize, err)
			}
		})
	}
}

func TestNewLeapArrayNilBuilderTakesPrecedence(t *testing.T) {
	got, err := NewLeapArray(0, 0, nil)
	if got != nil {
		t.Fatalf("NewLeapArray(0, 0, nil) = %#v, want nil", got)
	}
	if !errors.Is(err, ErrBucketBuilderIsNil) {
		t.Fatalf("NewLeapArray(0, 0, nil) error = %v, want ErrBucketBuilderIsNil", err)
	}
}

func TestNewLeapArrayValidDimensionsUnchanged(t *testing.T) {
	got, err := NewLeapArray(4, 1000, &Mock{})
	if err != nil {
		t.Fatalf("NewLeapArray returned error: %v", err)
	}
	if got.n != 4 {
		t.Fatalf("n = %d, want 4", got.n)
	}
	if got.intervalSize != 1000 {
		t.Fatalf("intervalSize = %d, want 1000", got.intervalSize)
	}
	if got.bucketSize != 250 {
		t.Fatalf("bucketSize = %d, want 250", got.bucketSize)
	}
	if got.array.length != 4 {
		t.Fatalf("array length = %d, want 4", got.array.length)
	}
}
