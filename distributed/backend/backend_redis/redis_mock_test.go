package backend_redis

import (
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/go-redis/redis/v8"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/songzhibin97/gkit/distributed/backend"
	"github.com/songzhibin97/gkit/distributed/task"
)

func newMockBackend(t *testing.T, expire int64) (backend.Backend, *miniredis.Miniredis) {
	t.Helper()
	mr, err := miniredis.Run()
	require.NoError(t, err)
	t.Cleanup(mr.Close)
	client := redis.NewUniversalClient(&redis.UniversalOptions{Addrs: []string{mr.Addr()}})
	t.Cleanup(func() { _ = client.Close() })
	return NewBackendRedis(client, expire), mr
}

// TestUpdateStatusTTLUnit 回归测试：updateStatus 设置的 TTL 单位为秒，不是纳秒。
// 历史 bug：time.Duration(3600) 被解释为 3600ns (~3.6µs)，导致 key 几乎瞬间过期。
func TestUpdateStatusTTLUnit(t *testing.T) {
	b, mr := newMockBackend(t, 3600)

	sig := &task.Signature{ID: "task1", GroupID: "group1", Name: "task1"}
	require.NoError(t, b.SetStatePending(sig))

	ttl := mr.TTL(sig.ID)
	assert.InDelta(t, time.Hour.Seconds(), ttl.Seconds(), 5,
		"expected TTL ~= 1h, got %s — confirms unit is seconds, not nanoseconds", ttl)
}

// TestGroupTakeOverTTLUnit 回归测试：GroupTakeOver 设置的 TTL 单位为秒。
func TestGroupTakeOverTTLUnit(t *testing.T) {
	b, mr := newMockBackend(t, 60)

	require.NoError(t, b.GroupTakeOver("group1", "g", "task1", "task2"))

	ttl := mr.TTL("group1")
	assert.InDelta(t, (60 * time.Second).Seconds(), ttl.Seconds(), 2,
		"expected TTL ~= 60s, got %s", ttl)
}

// TestNeverExpire 验证 resultExpire == -1 时 key 不会过期。
func TestNeverExpire(t *testing.T) {
	b, mr := newMockBackend(t, -1)

	sig := &task.Signature{ID: "task1", GroupID: "group1", Name: "task1"}
	require.NoError(t, b.SetStatePending(sig))

	// miniredis: TTL 为 0 表示该 key 没有设置过期时间。
	assert.Equal(t, time.Duration(0), mr.TTL(sig.ID))
}
