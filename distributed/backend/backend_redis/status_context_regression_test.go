package backend_redis

import (
	"context"
	"errors"
	"github.com/go-redis/redis/v8"
	"github.com/songzhibin97/gkit/distributed/backend/result"
	"github.com/songzhibin97/gkit/distributed/task"
	"net"
	"testing"
	"time"
)

func TestIssue167RedisReadDeadline(t *testing.T) {
	// The owned pipe accepts GET but never replies; the real driver's socket
	// deadline must interrupt the read without a goroutine per result request.
	clientSide, serverSide := net.Pipe()
	release := make(chan struct{})
	serverDone := make(chan struct{})
	go func() {
		defer close(serverDone)
		var request [1024]byte
		_, err := serverSide.Read(request[:])
		if err == nil {
			<-release
		}
	}()
	client := redis.NewClient(&redis.Options{Dialer: func(context.Context, string, string) (net.Conn, error) { return clientSide, nil }, MaxRetries: -1, ReadTimeout: 5 * time.Second})
	defer func() {
		close(release)
		if err := serverSide.Close(); err != nil {
			t.Error(err)
		}
		if err := client.Close(); err != nil {
			t.Error(err)
		}
		<-serverDone
	}()
	b := NewBackendRedis(client, -1)
	done := make(chan error, 1)
	go func() {
		_, err := result.NewAsyncResult(&task.Signature{ID: "socket-deadline"}, b).GetWithTimeout(20*time.Millisecond, time.Hour)
		done <- err
	}()
	select {
	case err := <-done:
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("socket read timeout: %v", err)
		}
	case <-time.After(time.Second):
		if err := serverSide.Close(); err != nil {
			t.Error(err)
		}
		<-done
		t.Fatal("socket read exceeded timeout")
	}
}
