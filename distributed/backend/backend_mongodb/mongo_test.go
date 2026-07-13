package backend_mongodb

import (
	"context"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/songzhibin97/gkit/generator"

	"github.com/stretchr/testify/assert"

	"github.com/songzhibin97/gkit/distributed/backend"
	"github.com/songzhibin97/gkit/distributed/task"
	"go.mongodb.org/mongo-driver/mongo"
	moption "go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readpref"
)

// initLiveBackend returns a MongoDB-backed backend for live tests.
// It skips the calling test unless GKIT_MONGODB_URI points to a reachable server.
func initLiveBackend(t *testing.T) backend.Backend {
	t.Helper()
	uri := os.Getenv("GKIT_MONGODB_URI")
	if uri == "" {
		t.Skip("GKIT_MONGODB_URI is not set")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	client, err := mongo.Connect(ctx, moption.Client().ApplyURI(uri).SetServerSelectionTimeout(5*time.Second))
	if err != nil {
		t.Skipf("connect MongoDB %q: %v", uri, err)
	}
	if err := client.Ping(ctx, readpref.Primary()); err != nil {
		disconnectCtx, disconnectCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer disconnectCancel()
		_ = client.Disconnect(disconnectCtx)
		t.Skipf("ping MongoDB %q: %v", uri, err)
	}
	t.Cleanup(func() {
		disconnectCtx, disconnectCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer disconnectCancel()
		if err := client.Disconnect(disconnectCtx); err != nil {
			t.Errorf("disconnect MongoDB client: %v", err)
		}
	})
	return NewBackendMongoDB(client, -1)
}

func TestGroupTaskOver(t *testing.T) {
	_backend := initLiveBackend(t)
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
		assert.Error(t, err, mongo.ErrNoDocuments)
	}
	_ = _backend.GroupTakeOver(group.GroupID, group.Name, task1.ID, task2.ID)
	isCompleted, err = _backend.GroupCompleted(group.GroupID)
	if assert.NoError(t, err) {
		assert.False(t, isCompleted)
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
	_backend := initLiveBackend(t)
	task1 := task.Signature{
		ID:      "task1",
		GroupID: "group",
		Name:    "task1",
	}
	_ = _backend.ResetTask(task1.ID)

	status, err := _backend.GetStatus(task1.ID)
	assert.Equal(t, err, mongo.ErrNoDocuments)
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
	_backend := initLiveBackend(t)
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
	assert.Equal(t, err, mongo.ErrNoDocuments)
	assert.Nil(t, status)
}
