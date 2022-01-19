package backend_mongodb

import (
	"context"
	"errors"
	"time"

	"github.com/songzhibin97/gkit/distributed/task"
	"github.com/songzhibin97/gkit/options"
	"go.mongodb.org/mongo-driver/bson"
	moption "go.mongodb.org/mongo-driver/mongo/options"

	"github.com/songzhibin97/gkit/distributed/backend"
	"go.mongodb.org/mongo-driver/mongo"
)

type BackendMongoDB struct {
	// client mongo客户端
	client *mongo.Client
	// resultExpire 数据过期时间
	// -1 代表永不过期
	// 0 会设置默认过期时间
	// 单位为ns
	resultExpire int64
	// taskTable taskTable
	taskTable *mongo.Collection
	// groupTable groupTable
	groupTable *mongo.Collection
}

// SetResultExpire 设置结果超时时间
func (b *BackendMongoDB) SetResultExpire(expire int64) {
	b.resultExpire = expire
}

func (b *BackendMongoDB) GroupTakeOver(groupID string, name string, taskIDs ...string) error {
	group := task.InitGroupMeta(groupID, name, b.resultExpire, taskIDs...)
	_, err := b.groupTable.InsertOne(context.Background(), group)
	return err
}

func (b *BackendMongoDB) GroupCompleted(groupID string) (bool, error) {
	group, err := b.getGroup(groupID)
	if err != nil {
		return false, err
	}
	status, err := b.getTaskStatus(group.TaskIDs)
	if err != nil {
		return false, err
	}
	ln := 0
	for _, t := range status {
		if !t.IsCompleted() {
			return false, nil
		}
		ln++
	}
	return len(group.TaskIDs) == ln, nil
}

func (b *BackendMongoDB) getGroup(groupID string) (*task.GroupMeta, error) {
	group := task.GroupMeta{}
	query := bson.M{
		"_id": groupID,
	}
	err := b.groupTable.FindOne(context.Background(), query).Decode(&group)
	return &group, err
}

func (b *BackendMongoDB) getTaskStatus(taskIDs []string) ([]*task.Status, error) {
	statusList := make([]*task.Status, 0, len(taskIDs))
	taskQuery := bson.M{
		"_id": bson.M{
			"$in": taskIDs,
		},
	}
	result, err := b.taskTable.Find(context.Background(), taskQuery)
	if err != nil {
		return nil, err
	}
	defer result.Close(context.Background())
	for result.Next(context.Background()) {
		var status task.Status
		if err = result.Decode(&status); err != nil {
			return nil, err
		}
		statusList = append(statusList, &status)
	}
	return statusList, nil
}

func (b *BackendMongoDB) GroupTaskStatus(groupID string) ([]*task.Status, error) {
	group, err := b.getGroup(groupID)
	if err != nil {
		return nil, err
	}
	return b.getTaskStatus(group.TaskIDs)
}

func (b *BackendMongoDB) TriggerCompleted(groupID string) (bool, error) {
	query := bson.M{
		"_id":           groupID,
		"trigger_chord": false,
	}
	change := bson.M{
		"$set": bson.M{
			"trigger_chord": true,
		},
	}
	v, err := b.groupTable.UpdateOne(context.Background(), query, change, moption.Update())
	if err != nil && errors.Is(err, mongo.ErrNoDocuments) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return v.ModifiedCount > 0, nil
}

func (b *BackendMongoDB) SetStatePending(signature *task.Signature) error {
	update := bson.M{
		"_id":       signature.ID,
		"status":    task.StatePending,
		"group_id":  signature.GroupID,
		"name":      signature.Name,
		"create_at": time.Now().Local(),
	}
	return b.updateStatus(signature, update)
}

func (b *BackendMongoDB) SetStateReceived(signature *task.Signature) error {
	update := bson.M{
		"status": task.StateReceived,
	}
	return b.updateStatus(signature, update)
}

func (b *BackendMongoDB) SetStateStarted(signature *task.Signature) error {
	update := bson.M{
		"status": task.StateStarted,
	}
	return b.updateStatus(signature, update)
}

func (b *BackendMongoDB) SetStateRetry(signature *task.Signature) error {
	update := bson.M{
		"status": task.StateRetry,
	}
	return b.updateStatus(signature, update)
}

func (b *BackendMongoDB) SetStateSuccess(signature *task.Signature, results []*task.Result) error {
	update := bson.M{
		"status":  task.StateSuccess,
		"results": results,
	}
	return b.updateStatus(signature, update)
}

func (b *BackendMongoDB) SetStateFailure(signature *task.Signature, err string) error {
	update := bson.M{
		"status": task.StateFailure,
		"error":  err,
	}
	return b.updateStatus(signature, update)
}

func (b *BackendMongoDB) GetStatus(taskID string) (*task.Status, error) {
	var status task.Status
	query := bson.M{
		"_id": taskID,
	}
	err := b.taskTable.FindOne(context.Background(), query).Decode(&status)
	if err != nil {
		return nil, err
	}
	return &status, err
}

func (b *BackendMongoDB) ResetTask(taskIDs ...string) error {
	query := bson.M{
		"_id": bson.M{
			"$in": taskIDs,
		},
	}
	_, err := b.taskTable.DeleteMany(context.Background(), query)
	return err
}

func (b *BackendMongoDB) ResetGroup(groupIDs ...string) error {
	query := bson.M{
		"_id": bson.M{
			"$in": groupIDs,
		},
	}
	_, err := b.groupTable.DeleteMany(context.Background(), query)
	return err
}

// updateStatus 更新状态
func (b *BackendMongoDB) updateStatus(signature *task.Signature, update bson.M) error {
	update = bson.M{"$set": update}
	query := bson.M{"_id": signature.ID}
	_, err := b.taskTable.UpdateOne(context.Background(), query, update, moption.Update().SetUpsert(true))
	return err
}

func (b *BackendMongoDB) createIndex() error {
	_, err := b.taskTable.Indexes().CreateMany(context.Background(), []mongo.IndexModel{
		{
			Keys:    bson.M{"status": 1},
			Options: moption.Index().SetBackground(true).SetExpireAfterSeconds(int32(b.resultExpire)),
		},
		{
			Keys:    bson.M{"lock": 1},
			Options: moption.Index().SetBackground(true).SetExpireAfterSeconds(int32(b.resultExpire)),
		},
	})
	return err
}

func NewBackendMongoDB(client *mongo.Client, resultExpire int64, options ...options.Option) backend.Backend {
	c := &config{
		databaseName:   "gkit",
		tableTaskName:  "tasks",
		tableGroupName: "groups",
	}
	for _, option := range options {
		option(c)
	}
	b := BackendMongoDB{
		client:       client,
		resultExpire: resultExpire,
		taskTable:    client.Database(c.databaseName).Collection(c.tableTaskName),
		groupTable:   client.Database(c.databaseName).Collection(c.tableGroupName),
	}
	_ = b.createIndex()
	return &b
}
