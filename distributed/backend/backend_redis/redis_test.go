package backend_redis

import (
	"strconv"
	"testing"
	"time"

	"github.com/songzhibin97/gkit/generator"

	"github.com/songzhibin97/gkit/distributed/task"
	"github.com/stretchr/testify/assert"

	"github.com/go-redis/redis/v8"
	"github.com/songzhibin97/gkit/distributed/backend"
)

func InitBackend() backend.Backend {
	opt := redis.UniversalOptions{
		Addrs: []string{"127.0.0.1:6379"},
	}
	client := redis.NewUniversalClient(&opt)
	if client == nil {
		return nil
	}
	return NewBackendRedis(client, -1)
}

func TestGroupTakeOver(t *testing.T) {
	_backend := InitBackend()
	if _backend == nil {
		t.Skip()
	}
	g := generator.NewSnowflake(time.Now().Local(), 1)
	ids := make([]string, 0, 3)
	for i := 0; i < 3; i++ {
		id, _ := g.NextID()
		if i == 0 {
			ids = append(ids, "group:"+strconv.FormatUint(id, 10))
		} else {
			ids = append(ids, "task:"+strconv.FormatUint(id, 10))
		}

	}
	var (
		group = task.GroupMeta{
			GroupID: ids[0],
			Name:    "group",
		}
		task1 = task.Signature{
			ID:      ids[1],
			GroupID: group.GroupID,
			Name:    "task1",
		}
		task2 = task.Signature{
			ID:      ids[2],
			GroupID: group.GroupID,
			Name:    "task2",
		}
	)
	_ = _backend.ResetGroup(group.GroupID)
	_ = _backend.ResetTask(task1.ID, task2.ID)
	isCompleted, err := _backend.GroupCompleted(group.GroupID)
	if assert.Error(t, err) {
		assert.False(t, isCompleted)
		assert.Equal(t, err, redis.Nil)
	}

	_ = _backend.GroupTakeOver(group.GroupID, group.Name, task1.ID, task2.ID)
	isCompleted, err = _backend.GroupCompleted(group.GroupID)
	if assert.Error(t, err) {
		assert.False(t, isCompleted)
		assert.Equal(t, err, redis.Nil)
	}

	_ = _backend.SetStatePending(&task1)
	_ = _backend.SetStateStarted(&task2)

	isCompleted, err = _backend.GroupCompleted(group.GroupID)
	if assert.NoError(t, err) {
		assert.False(t, isCompleted)
	}
	result := []*task.Result{{
		Type:  "int",
		Value: 1,
	}}
	_ = _backend.SetStateStarted(&task1)
	_ = _backend.SetStateSuccess(&task2, result)
	isCompleted, err = _backend.GroupCompleted(group.GroupID)
	if assert.NoError(t, err) {
		assert.False(t, isCompleted)
	}
	_ = _backend.SetStateFailure(&task1, "failure")
	isCompleted, err = _backend.GroupCompleted(group.GroupID)
	if assert.NoError(t, err) {
		assert.True(t, isCompleted)
	}
}

func TestGetStatus(t *testing.T) {
	_backend := InitBackend()
	if _backend == nil {
		t.Skip()
	}
	task1 := task.Signature{
		ID:      "task1",
		GroupID: "group",
		Name:    "task1",
	}
	_ = _backend.ResetTask(task1.ID)

	status, err := _backend.GetStatus(task1.ID)
	assert.Equal(t, err, redis.Nil)
	assert.Nil(t, status)

	_ = _backend.SetStatePending(&task1)
	status, err = _backend.GetStatus(task1.ID)
	assert.NoError(t, err)
	assert.Equal(t, status.Status, task.StatePending)

	_ = _backend.SetStateReceived(&task1)
	status, err = _backend.GetStatus(task1.ID)
	assert.NoError(t, err)
	assert.Equal(t, status.Status, task.StateReceived)

	_ = _backend.SetStateStarted(&task1)
	status, err = _backend.GetStatus(task1.ID)
	assert.NoError(t, err)
	assert.Equal(t, status.Status, task.StateStarted)

	result := &task.Result{
		Type:  "int",
		Value: 1,
	}
	_ = _backend.SetStateSuccess(&task1, []*task.Result{result})
	status, err = _backend.GetStatus(task1.ID)
	assert.NoError(t, err)
	assert.Equal(t, status.Status, task.StateSuccess)
}

func TestResult(t *testing.T) {
	_backend := InitBackend()
	if _backend == nil {
		t.Skip()
	}
	task1 := task.Signature{
		ID:      "task1",
		GroupID: "group",
		Name:    "task1",
	}
	_ = _backend.SetStatePending(&task1)
	status, err := _backend.GetStatus(task1.ID)
	assert.NoError(t, err)
	assert.Equal(t, status.Status, task.StatePending)

	_ = _backend.ResetTask(task1.ID)

	status, err = _backend.GetStatus(task1.ID)
	assert.Equal(t, err, redis.Nil)
	assert.Nil(t, status)
}
