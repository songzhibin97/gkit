package codel

import (
	"context"
	"testing"

	"github.com/songzhibin97/gkit/options"
)

func TestNewQueueInvalidConfigFallsBackToDefaults(t *testing.T) {
	tests := []struct {
		name    string
		options []options.Option
	}{
		{name: "zero target only", options: []options.Option{SetTarget(0), SetInternal(777)}},
		{name: "negative target only", options: []options.Option{SetTarget(-1), SetInternal(777)}},
		{name: "zero internal only", options: []options.Option{SetTarget(33), SetInternal(0)}},
		{name: "negative internal only", options: []options.Option{SetTarget(33), SetInternal(-1)}},
		{name: "both zero", options: []options.Option{SetTarget(0), SetInternal(0)}},
		{name: "zero target negative internal", options: []options.Option{SetTarget(0), SetInternal(-1)}},
		{name: "negative target zero internal", options: []options.Option{SetTarget(-1), SetInternal(0)}},
		{name: "both negative", options: []options.Option{SetTarget(-1), SetInternal(-1)}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			q := NewQueue(tt.options...)
			assertQueueConfig(t, q, defaultConfig())

			for i := 0; i < 2; i++ {
				if err := pushAndPopImmediately(t, q); err != nil {
					t.Fatalf("immediate Push/Pop %d returned %v, want nil", i+1, err)
				}
			}
		})
	}
}

func TestNewQueuePreservesDefaultAndValidConfig(t *testing.T) {
	tests := []struct {
		name    string
		options []options.Option
		want    *config
	}{
		{name: "default", want: defaultConfig()},
		{
			name:    "valid",
			options: []options.Option{SetTarget(33), SetInternal(777)},
			want:    &config{target: 33, internal: 777},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			q := NewQueue(tt.options...)
			assertQueueConfig(t, q, tt.want)
			for i := 0; i < 2; i++ {
				if err := pushAndPopImmediately(t, q); err != nil {
					t.Fatalf("immediate Push/Pop %d returned %v, want nil", i+1, err)
				}
			}
		})
	}
}

func assertQueueConfig(t *testing.T, q *Queue, want *config) {
	t.Helper()
	if q.conf.target != want.target || q.conf.internal != want.internal {
		t.Fatalf(
			"queue config = {target:%d internal:%d}, want {target:%d internal:%d}",
			q.conf.target,
			q.conf.internal,
			want.target,
			want.internal,
		)
	}
}

func pushAndPopImmediately(t *testing.T, q *Queue) error {
	t.Helper()
	result := make(chan error, 1)
	go func() { result <- q.Push(context.Background()) }()
	p := takePacket(t, q)
	q.packets <- p
	popWithWatchdog(t, q)
	return awaitPushResult(t, result)
}
