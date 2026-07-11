package backend_mongodb

import (
	"context"
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/songzhibin97/gkit/distributed/task"
	"github.com/songzhibin97/gkit/options"
	"go.mongodb.org/mongo-driver/bson"
	moption "go.mongodb.org/mongo-driver/mongo/options"

	"github.com/songzhibin97/gkit/distributed/backend"
	"go.mongodb.org/mongo-driver/mongo"
)

const (
	defaultMongoResultExpireSeconds int64 = 3600
	maxMongoTTLSeconds              int64 = math.MaxInt32
	mongoIndexSetupTimeout                = 5 * time.Second
	taskTTLIndexName                      = "gkit_tasks_create_at_ttl"
	groupTTLIndexName                     = "gkit_groups_create_at_ttl"
)

type BackendMongoDB struct {
	// client mongo客户端
	client *mongo.Client
	// resultExpire 数据过期时间
	// -1 代表永不过期
	// 0 会设置默认过期时间
	// 单位为s
	resultExpire int64
	// taskTable taskTable
	taskTable *mongo.Collection
	// groupTable groupTable
	groupTable *mongo.Collection
}

// SetResultExpire normalizes and stores the retention value for compatibility.
// It does not rebuild TTL indexes online because this method cannot report
// index-creation errors; configure retention through the constructor instead.
func (b *BackendMongoDB) SetResultExpire(expire int64) {
	b.resultExpire = normalizeResultExpire(expire)
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
	ctx := context.Background()
	taskQuery := bson.M{
		"_id": bson.M{
			"$in": taskIDs,
		},
	}
	result, err := b.taskTable.Find(ctx, taskQuery)
	if err != nil {
		return nil, err
	}
	return collectTaskStatuses(ctx, result, len(taskIDs))
}

type taskStatusCursor interface {
	Next(context.Context) bool
	Decode(interface{}) error
	Err() error
	Close(context.Context) error
}

func collectTaskStatuses(ctx context.Context, cursor taskStatusCursor, capacity int) (statuses []*task.Status, retErr error) {
	defer func() {
		if err := cursor.Close(ctx); err != nil {
			statuses = nil
			retErr = errors.Join(retErr, fmt.Errorf("backend_mongodb: close task status cursor: %w", err))
		}
	}()
	statusList := make([]*task.Status, 0, capacity)
	for cursor.Next(ctx) {
		var status task.Status
		if err := cursor.Decode(&status); err != nil {
			return nil, fmt.Errorf("backend_mongodb: decode task status: %w", err)
		}
		statusList = append(statusList, &status)
	}
	if err := cursor.Err(); err != nil {
		return nil, fmt.Errorf("backend_mongodb: iterate task statuses: %w", err)
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

func normalizeResultExpire(expire int64) int64 {
	if expire == 0 {
		return defaultMongoResultExpireSeconds
	}
	return expire
}

func buildTTLIndexModels(resultExpire int64) (int64, []mongo.IndexModel, []mongo.IndexModel, error) {
	resultExpire = normalizeResultExpire(resultExpire)
	if resultExpire < 0 {
		return resultExpire, nil, nil, nil
	}
	if resultExpire > maxMongoTTLSeconds {
		return 0, nil, nil, fmt.Errorf(
			"backend_mongodb: result expiration %d seconds exceeds MongoDB TTL maximum %d",
			resultExpire,
			maxMongoTTLSeconds,
		)
	}

	expireAfterSeconds := int32(resultExpire)
	taskModels := []mongo.IndexModel{{
		Keys: bson.D{{Key: "create_at", Value: 1}},
		Options: moption.Index().
			SetName(taskTTLIndexName).
			SetExpireAfterSeconds(expireAfterSeconds),
	}}
	groupModels := []mongo.IndexModel{{
		Keys: bson.D{{Key: "create_at", Value: 1}},
		Options: moption.Index().
			SetName(groupTTLIndexName).
			SetExpireAfterSeconds(expireAfterSeconds),
	}}
	return resultExpire, taskModels, groupModels, nil
}

func (b *BackendMongoDB) createIndexes(ctx context.Context, taskModels, groupModels []mongo.IndexModel) error {
	if len(taskModels) > 0 {
		if _, err := b.taskTable.Indexes().CreateMany(ctx, taskModels); err != nil {
			return fmt.Errorf("backend_mongodb: create task TTL index: %w", err)
		}
	}
	if len(groupModels) > 0 {
		if _, err := b.groupTable.Indexes().CreateMany(ctx, groupModels); err != nil {
			return fmt.Errorf("backend_mongodb: create group TTL index: %w", err)
		}
	}
	return nil
}

// NewBackendMongoDB constructs a MongoDB-backed Backend. It returns nil if
// retention validation or TTL index initialization fails.
//
// Deprecated: Use NewBackendMongoDBE to receive the initialization error.
func NewBackendMongoDB(client *mongo.Client, resultExpire int64, options ...options.Option) backend.Backend {
	b, err := NewBackendMongoDBE(client, resultExpire, options...)
	if err != nil {
		return nil
	}
	return b
}

// NewBackendMongoDBE constructs a MongoDB-backed Backend and returns retention
// validation or TTL index initialization errors to the caller.
func NewBackendMongoDBE(client *mongo.Client, resultExpire int64, options ...options.Option) (backend.Backend, error) {
	if client == nil {
		return nil, errors.New("backend_mongodb: nil client")
	}
	normalizedExpire, taskModels, groupModels, err := buildTTLIndexModels(resultExpire)
	if err != nil {
		return nil, err
	}

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
		resultExpire: normalizedExpire,
		taskTable:    client.Database(c.databaseName).Collection(c.tableTaskName),
		groupTable:   client.Database(c.databaseName).Collection(c.tableGroupName),
	}
	ctx, cancel := context.WithTimeout(context.Background(), mongoIndexSetupTimeout)
	defer cancel()
	if err := b.createIndexes(ctx, taskModels, groupModels); err != nil {
		return nil, err
	}
	return &b, nil
}
