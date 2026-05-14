package egroup

import (
	"context"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/songzhibin97/gkit/goroutine"
)

func TestLifeAdmin_Start(t *testing.T) {
	admin := NewLifeAdmin()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	srv := &http.Server{}

	admin.Add(Member{
		Start: func(ctx context.Context) error {
			t.Log("http start")
			return goroutine.Delegate(ctx, -1, func(ctx context.Context) error {
				if err := srv.Serve(ln); err != nil && err != http.ErrServerClosed {
					return err
				}
				return nil
			})
		},
		Shutdown: func(ctx context.Context) error {
			t.Log("http shutdown")
			return srv.Shutdown(context.Background())
		},
	})

	go func() {
		time.Sleep(200 * time.Millisecond)
		admin.Shutdown()
	}()

	if err := admin.Start(); err != nil && err != context.Canceled {
		t.Logf("Start returned: %v", err)
	}
}
