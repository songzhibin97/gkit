package pool

import (
	"context"
	"errors"
	"os"
	"sync/atomic"
	"testing"
)

type shutdownErrorFile struct {
	file  *os.File
	err   error
	calls int
}

func (r *shutdownErrorFile) Shutdown() error {
	r.calls++
	return errors.Join(r.file.Close(), r.err)
}

// Regression for #165, item 02-04: close all idle resources and preserve each
// close error, while retaining the pool's closed-state and cleanup guarantees.
func TestListShutdownCollectsAllResourceErrors(t *testing.T) {
	firstErr := errors.New("synthetic first resource close failure")
	secondErr := errors.New("synthetic second resource close failure")
	for _, test := range []struct {
		name string
		errs []error
	}{
		{name: "empty"},
		{name: "success", errs: []error{nil, nil}},
		{name: "single_error", errs: []error{firstErr}},
		{name: "mixed_errors", errs: []error{firstErr, nil, secondErr}},
	} {
		t.Run(test.name, func(t *testing.T) {
			list := NewList(SetActive(uint64(len(test.errs))), SetIdle(uint64(len(test.errs))), SetIdleTimeout(0)).(*List)
			t.Cleanup(func() {
				if err := list.Shutdown(); err != nil && err != ErrPoolClosed {
					t.Errorf("cleanup Shutdown: %v", err)
				}
			})
			var resources []*shutdownErrorFile
			dir := t.TempDir()
			list.New(func(context.Context) (IShutdown, error) {
				file, err := os.CreateTemp(dir, "resource-")
				if err != nil {
					return nil, err
				}
				t.Cleanup(func() {
					if err := file.Close(); err != nil && !errors.Is(err, os.ErrClosed) {
						t.Error(err)
					}
				})
				resource := &shutdownErrorFile{file: file, err: test.errs[len(resources)]}
				resources = append(resources, resource)
				return resource, nil
			})
			for range test.errs {
				if _, err := list.Get(context.Background()); err != nil {
					t.Fatalf("Get: %v", err)
				}
			}
			if len(resources) != len(test.errs) {
				t.Fatalf("created resources = %d, want %d", len(resources), len(test.errs))
			}
			for _, resource := range resources {
				if _, err := resource.file.WriteString("synthetic content"); err != nil {
					t.Fatalf("resource was not initially open: %v", err)
				}
				if err := list.Put(context.Background(), resource, false); err != nil {
					t.Fatalf("Put: %v", err)
				}
			}
			gotErr := list.Shutdown()
			wantError := false
			for _, wantErr := range test.errs {
				if wantErr != nil {
					wantError = true
					if !errors.Is(gotErr, wantErr) {
						t.Errorf("Shutdown error %v does not contain %v", gotErr, wantErr)
					}
				}
			}
			if !wantError && gotErr != nil {
				t.Errorf("successful Shutdown returned %v", gotErr)
			}
			for i, resource := range resources {
				if resource.calls != 1 {
					t.Errorf("resource %d close count = %d, want 1", i, resource.calls)
				}
				if _, err := resource.file.WriteString("after shutdown"); !errors.Is(err, os.ErrClosed) {
					t.Errorf("resource %d remains writable after Shutdown: %v", i, err)
				}
			}
			if resource, err := list.Get(context.Background()); resource != nil || err != ErrPoolClosed {
				t.Errorf("Get after Shutdown = (%v, %v), want (nil, ErrPoolClosed)", resource, err)
			}
			if got := atomic.LoadUint64(&list.active); got != 0 {
				t.Errorf("active after Shutdown = %d, want 0", got)
			}
			if list.idles.Len() != 0 {
				t.Errorf("Shutdown left %d idle resources", list.idles.Len())
			}
			select {
			case <-list.cleanerDone:
			default:
				t.Error("Shutdown returned before cleaner completed")
			}
			if err := list.Shutdown(); err != ErrPoolClosed {
				t.Errorf("second Shutdown = %v, want ErrPoolClosed", err)
			}
			for i, resource := range resources {
				if resource.calls != 1 {
					t.Errorf("resource %d closed again on repeated Shutdown: %d", i, resource.calls)
				}
			}
		})
	}
}
