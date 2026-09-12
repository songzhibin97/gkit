package distributed

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"

	"github.com/songzhibin97/gkit/log"

	"github.com/stretchr/testify/assert"

	"github.com/songzhibin97/gkit/distributed/backend/backend_redis"

	"github.com/go-redis/redis/v8"
	"github.com/songzhibin97/gkit/distributed/broker"
	"github.com/songzhibin97/gkit/distributed/controller/controller_redis"
	"github.com/songzhibin97/gkit/distributed/locker/lock_redis"
)

func initServer(t *testing.T) *Server {
	t.Helper()
	mr := miniredis.RunT(t)
	opt := redis.UniversalOptions{
		Addrs: []string{mr.Addr()},
	}
	client := redis.NewUniversalClient(&opt)
	t.Cleanup(func() {
		if err := client.Close(); err != nil {
			t.Error(err)
		}
	})
	lock := lock_redis.NewRedisLock(client)
	bk := broker.NewBroker(broker.NewRegisteredTask(), context.Background())
	c := controller_redis.NewControllerRedis(bk, client, "test_task", "delayed")
	backend := backend_redis.NewBackendRedis(client, -1)
	server, err := NewServerE(c, backend, lock, log.NewHelper(log.With(log.DefaultLogger)), nil)
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		if err := server.Shutdown(ctx); err != nil {
			t.Error(err)
		}
	})
	if err != nil {
		t.Fatalf("initialize isolated test server: %v", err)
	}
	return server
}

func TestRegisterTasks(t *testing.T) {
	t.Parallel()
	s := initServer(t)
	_, ok := s.GetRegisteredTask("test_task")
	assert.False(t, ok)
	err := s.RegisteredTasks(map[string]interface{}{
		"test_task": func() error { return nil },
	})
	assert.NoError(t, err)

	_, ok = s.GetRegisteredTask("test_task")
	assert.True(t, ok)
}

func TestRegisterTask(t *testing.T) {
	t.Parallel()
	s := initServer(t)
	_, ok := s.GetRegisteredTask("test_task")
	assert.False(t, ok)

	err := s.RegisteredTask("test_task", func() error { return nil })
	assert.NoError(t, err)

	_, ok = s.GetRegisteredTask("test_task")
	assert.True(t, ok)
}
