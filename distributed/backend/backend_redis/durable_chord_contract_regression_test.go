package backend_redis

import (
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/go-redis/redis/v8"
	"github.com/songzhibin97/gkit/distributed/backend/chordtest"
)

func TestDurableChordContract(t *testing.T) {
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	chordtest.Run(t, NewBackendRedis(client, -1).(*BackendRedis))
}
