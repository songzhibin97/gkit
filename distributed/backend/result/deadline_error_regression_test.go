package result

import (
	"context"
	"errors"
	"github.com/songzhibin97/gkit/distributed/task"
	"net"
	"os"
	"testing"
	"time"
)

// Keep Err/Done pending after Deadline, reproducing the interval where a socket
// deadline has fired but the context's independent timer has not updated Err.
type pendingDeadlineContext struct {
	context.Context
	deadline time.Time
}

func (c pendingDeadlineContext) Deadline() (time.Time, bool) { return c.deadline, true }

type deadlineOrderBackend struct {
	pendingResultBackend
	wait bool
	err  error
}

func (b *deadlineOrderBackend) GetStatusContext(ctx context.Context, id string) (*task.Status, error) {
	if b.wait {
		deadline, _ := ctx.Deadline()
		timer := time.NewTimer(time.Until(deadline))
		defer timer.Stop()
		<-timer.C
	}
	return &task.Status{TaskID: id, Status: task.StateSuccess, Results: task.Results{{Type: "int64", Value: int64(7)}}}, b.err
}

func TestTimeoutNormalizesReachedDeadlineBeforeContextErr(t *testing.T) {
	socketErr := &net.OpError{Op: "read", Net: "pipe", Err: os.ErrDeadlineExceeded}
	for _, tt := range []struct {
		name      string
		wait      bool
		err, want error
	}{{"late-socket-error", true, socketErr, context.DeadlineExceeded}, {"late-success", true, nil, context.DeadlineExceeded}, {"independent-early-timeout", false, socketErr, os.ErrDeadlineExceeded}} {
		t.Run(tt.name, func(t *testing.T) {
			duration := time.Hour
			if tt.wait {
				duration = 10 * time.Millisecond
			}
			ctx := pendingDeadlineContext{Context: context.Background(), deadline: time.Now().Add(duration)}
			b := &deadlineOrderBackend{wait: tt.wait, err: tt.err}
			r := NewAsyncResult(&task.Signature{ID: "deadline-order"}, b)
			cached := r.state
			values, err := r.monitor(ctx)
			if !errors.Is(err, tt.want) || values != nil {
				t.Fatalf("result=%v error=%v want %v", values, err, tt.want)
			}
			if !tt.wait && errors.Is(err, context.DeadlineExceeded) {
				t.Fatal("early backend timeout replaced by context timeout")
			}
			if r.state != cached {
				t.Fatal("expired or failed read replaced cached state")
			}
			if ctx.Err() != nil {
				t.Fatal("test context unexpectedly updated Err")
			}
		})
	}
	ctx, cancel := context.WithCancel(pendingDeadlineContext{Context: context.Background(), deadline: time.Now().Add(-time.Second)})
	cancel()
	if _, err := NewAsyncResult(&task.Signature{ID: "canceled"}, &deadlineOrderBackend{}).monitor(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation priority: %v", err)
	}
}
