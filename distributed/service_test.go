package distributed

import (
	"context"
	"testing"

	"github.com/songzhibin97/gkit/log"

	"github.com/stretchr/testify/assert"

	"github.com/songzhibin97/gkit/distributed/backend/backend_redis"

	"github.com/go-redis/redis/v8"
	"github.com/songzhibin97/gkit/distributed/broker"
	"github.com/songzhibin97/gkit/distributed/controller/controller_redis"
	"github.com/songzhibin97/gkit/distributed/locker/lock_ridis"
)

func initServer() *Server {
	opt := redis.UniversalOptions{
		Addrs: []string{"127.0.0.1:6379"},
	}
	client := redis.NewUniversalClient(&opt)
	if client == nil {
		return nil
	}
	lock := lock_ridis.NewRedisLock(client)
	bk := broker.NewBroker(broker.NewRegisteredTask(), context.Background())
	c := controller_redis.NewControllerRedis(bk, client, "test_task", "delayed")
	backend := backend_redis.NewBackendRedis(client, -1)
	return NewServer(c, backend, lock, log.NewHelper(log.With(log.DefaultLogger)), nil)
}

func TestRegisterTasks(t *testing.T) {
	t.Parallel()
	s := initServer()
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
	s := initServer()
	_, ok := s.GetRegisteredTask("test_task")
	assert.False(t, ok)

	err := s.RegisteredTask("test_task", func() error { return nil })
	assert.NoError(t, err)

	_, ok = s.GetRegisteredTask("test_task")
	assert.True(t, ok)
}
