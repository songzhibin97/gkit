package backend_mongodb

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/songzhibin97/gkit/distributed/task"
)

type fakeTaskStatusCursor struct {
	statuses    []task.Status
	nextIndex   int
	current     int
	decodeErrAt int
	decodeErr   error
	terminalErr error
	closeErr    error
	closeCalls  int
}

func (c *fakeTaskStatusCursor) Next(context.Context) bool {
	if c.nextIndex >= len(c.statuses) {
		return false
	}
	c.current = c.nextIndex
	c.nextIndex++
	return true
}

func (c *fakeTaskStatusCursor) Decode(value interface{}) error {
	if c.current == c.decodeErrAt && c.decodeErr != nil {
		return c.decodeErr
	}
	status := value.(*task.Status)
	*status = c.statuses[c.current]
	return nil
}

func (c *fakeTaskStatusCursor) Err() error {
	return c.terminalErr
}

func (c *fakeTaskStatusCursor) Close(context.Context) error {
	c.closeCalls++
	return c.closeErr
}

func TestCollectTaskStatuses(t *testing.T) {
	t.Run("normal", func(t *testing.T) {
		cursor := &fakeTaskStatusCursor{
			statuses:    []task.Status{{TaskID: "task-1"}, {TaskID: "task-2"}},
			decodeErrAt: -1,
		}
		statuses, err := collectTaskStatuses(context.Background(), cursor, 2)
		if err != nil {
			t.Fatal(err)
		}
		if len(statuses) != 2 || statuses[0].TaskID != "task-1" || statuses[1].TaskID != "task-2" {
			t.Fatalf("statuses = %#v, want both decoded tasks", statuses)
		}
		if cursor.closeCalls != 1 {
			t.Fatalf("Close calls = %d, want 1", cursor.closeCalls)
		}
	})

	t.Run("decode error", func(t *testing.T) {
		decodeErr := errors.New("decode failed")
		cursor := &fakeTaskStatusCursor{
			statuses:    []task.Status{{TaskID: "task-1"}, {TaskID: "task-2"}},
			decodeErrAt: 1,
			decodeErr:   decodeErr,
		}
		statuses, err := collectTaskStatuses(context.Background(), cursor, 2)
		if statuses != nil {
			t.Fatalf("statuses = %#v, want nil on decode error", statuses)
		}
		if !errors.Is(err, decodeErr) {
			t.Fatalf("error = %v, want wrapped decode error", err)
		}
		if !strings.Contains(err.Error(), "decode task status") {
			t.Fatalf("error = %v, want decode operation context", err)
		}
		if cursor.closeCalls != 1 {
			t.Fatalf("Close calls = %d, want 1", cursor.closeCalls)
		}
	})

	t.Run("terminal cursor error", func(t *testing.T) {
		terminalErr := errors.New("getMore failed")
		cursor := &fakeTaskStatusCursor{
			statuses:    []task.Status{{TaskID: "partial"}},
			decodeErrAt: -1,
			terminalErr: terminalErr,
		}
		statuses, err := collectTaskStatuses(context.Background(), cursor, 1)
		if statuses != nil {
			t.Fatalf("statuses = %#v, want nil rather than partial results", statuses)
		}
		if !errors.Is(err, terminalErr) {
			t.Fatalf("error = %v, want wrapped cursor error", err)
		}
		if !strings.Contains(err.Error(), "iterate task statuses") {
			t.Fatalf("error = %v, want iteration operation context", err)
		}
		if cursor.closeCalls != 1 {
			t.Fatalf("Close calls = %d, want 1", cursor.closeCalls)
		}
	})

	t.Run("close error after success", func(t *testing.T) {
		closeErr := errors.New("close failed")
		cursor := &fakeTaskStatusCursor{
			statuses:    []task.Status{{TaskID: "task-1"}},
			decodeErrAt: -1,
			closeErr:    closeErr,
		}
		statuses, err := collectTaskStatuses(context.Background(), cursor, 1)
		if statuses != nil {
			t.Fatalf("statuses = %#v, want nil on close error", statuses)
		}
		if !errors.Is(err, closeErr) {
			t.Fatalf("error = %v, want wrapped close error", err)
		}
		if !strings.Contains(err.Error(), "close task status cursor") {
			t.Fatalf("error = %v, want close operation context", err)
		}
	})

	t.Run("iteration and close errors are both preserved", func(t *testing.T) {
		terminalErr := errors.New("getMore failed")
		closeErr := errors.New("close failed")
		cursor := &fakeTaskStatusCursor{
			decodeErrAt: -1,
			terminalErr: terminalErr,
			closeErr:    closeErr,
		}
		statuses, err := collectTaskStatuses(context.Background(), cursor, 0)
		if statuses != nil {
			t.Fatalf("statuses = %#v, want nil on cursor errors", statuses)
		}
		if !errors.Is(err, terminalErr) || !errors.Is(err, closeErr) {
			t.Fatalf("error = %v, want both iteration and close errors", err)
		}
	})
}
