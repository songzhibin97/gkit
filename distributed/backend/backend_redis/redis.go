package backend_redis

import (
	"bytes"
	"context"
	"errors"
	"reflect"
	"time"

	json "github.com/json-iterator/go"

	"github.com/go-redsync/redsync/v4/redis/goredis/v8"
	"github.com/songzhibin97/gkit/distributed/task"

	"github.com/go-redis/redis/v8"
	"github.com/go-redsync/redsync/v4"
	"github.com/songzhibin97/gkit/distributed/backend"
)

var defaultResultExpire int64 = 3600

var ErrType = errors.New("err type")

type BackendRedis struct {
	// client redis客户端
	client redis.UniversalClient
	// lock 分布式锁
	lock *redsync.Redsync
	// resultExpire 数据过期时间
	// -1 代表永不过期
	// 0 会设置默认过期时间
	// 单位为ns
	resultExpire int64
}

func NewBackendRedis(client redis.UniversalClient, resultExpire int64) backend.Backend {
	b := &BackendRedis{
		client:       client,
		lock:         redsync.New(goredis.NewPool(client)),
		resultExpire: resultExpire,
	}
	if b.resultExpire == 0 {
		b.resultExpire = defaultResultExpire
	}
	return b
}

// SetResultExpire 设置结果超时时间
func (b *BackendRedis) SetResultExpire(expire int64) {
	b.resultExpire = expire
}

func (b *BackendRedis) GroupTakeOver(groupID string, name string, taskIDs ...string) error {
	group := task.InitGroupMeta(groupID, name, b.resultExpire, taskIDs...)
	body, err := json.Marshal(group)
	if err != nil {
		return err
	}
	expire := b.resultExpire
	if expire < 0 {
		expire = 0
	}
	// 避免接管任务记录被覆盖
	var ok bool
	for !ok {
		ok, err = b.client.SetNX(context.Background(), groupID, body, time.Duration(expire)).Result()
		if err != nil {
			return err
		}
		if !ok {
			time.Sleep(time.Second)
		}
	}
	return nil
}

func (b *BackendRedis) GroupCompleted(groupID string) (bool, error) {
	list, err := b.GroupTaskStatus(groupID)
	if err != nil {
		return false, err
	}
	for _, status := range list {
		if !status.IsCompleted() {
			return false, nil
		}
	}
	return true, nil
}

func (b *BackendRedis) GroupTaskStatus(groupID string) ([]*task.Status, error) {
	var ret []*task.Status
	// 同一个groupID 可能接管多个任务
	// 拿到所有的key
	var taskIDs []string
	groups, err := b.shouldAndBind(&task.GroupMeta{}, groupID)
	if err != nil {
		return nil, err
	}
	_groups := groups.([]interface{})
	if len(_groups) == 0 {
		return nil, nil
	}

	for _, group := range _groups {
		_group, ok := group.(*task.GroupMeta)
		if !ok {
			return nil, ErrType
		}
		for _, id := range _group.TaskIDs {
			taskIDs = append(taskIDs, id)
		}
	}
	statusList, err := b.shouldAndBind(&task.Status{}, taskIDs...)
	if err != nil {
		return nil, err
	}

	_statusList := statusList.([]interface{})
	for _, status := range _statusList {
		_status, ok := status.(*task.Status)
		if !ok {
			return nil, ErrType
		}
		ret = append(ret, _status)
	}
	return ret, nil
}

func (b *BackendRedis) TriggerCompleted(groupID string) (bool, error) {
	// 分布式锁
	l := b.lock.NewMutex("TriggerCompletedMutex" + groupID)
	if err := l.Lock(); err != nil {
		return false, err
	}
	defer l.Unlock()
	group, err := b.getGroup(groupID)
	if err != nil {
		return false, err
	}
	if group.TriggerCompleted {
		return false, nil
	}
	group.TriggerCompleted = true
	body, _ := json.Marshal(group)
	expire := b.resultExpire
	if expire < 0 {
		expire = 0
	}
	err = b.client.Set(context.Background(), groupID, body, time.Duration(expire)).Err()
	if err != nil {
		return false, err
	}
	return true, nil
}

func (b *BackendRedis) SetStatePending(signature *task.Signature) error {
	return b.updateStatus(task.NewPendingState(signature))
}

func (b *BackendRedis) SetStateReceived(signature *task.Signature) error {
	dst := task.NewReceivedState(signature)
	b.migrate(dst)
	return b.updateStatus(dst)
}

func (b *BackendRedis) SetStateStarted(signature *task.Signature) error {
	dst := task.NewStartedState(signature)
	b.migrate(dst)
	return b.updateStatus(dst)
}

func (b *BackendRedis) SetStateRetry(signature *task.Signature) error {
	dst := task.NewRetryState(signature)
	b.migrate(dst)
	return b.updateStatus(dst)
}

func (b *BackendRedis) SetStateSuccess(signature *task.Signature, results []*task.Result) error {
	dst := task.NewSuccessState(signature, results...)
	b.migrate(dst)
	return b.updateStatus(dst)
}

func (b *BackendRedis) SetStateFailure(signature *task.Signature, err string) error {
	dst := task.NewFailureState(signature, err)
	b.migrate(dst)
	return b.updateStatus(dst)
}

func (b *BackendRedis) GetStatus(taskID string) (*task.Status, error) {
	return b.getStatus(taskID)
}

func (b *BackendRedis) ResetTask(taskIDs ...string) error {
	if len(taskIDs) == 0 {
		return nil
	}
	return b.client.Del(context.Background(), taskIDs...).Err()
}

func (b *BackendRedis) ResetGroup(groupIDs ...string) error {
	if len(groupIDs) == 0 {
		return nil
	}
	return b.client.Del(context.Background(), groupIDs...).Err()
}

// shouldAndBind 批量获取对应key的group信息
// obj interface must ptr
func (b *BackendRedis) shouldAndBind(dst interface{}, keys ...string) (interface{}, error) {
	var src []interface{}
	results, err := b.client.Pipelined(context.Background(), func(pipeline redis.Pipeliner) error {
		for _, key := range keys {
			pipeline.Get(context.Background(), key)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	for _, result := range results {
		stringCmd, ok := result.(*redis.StringCmd)
		if !ok {
			continue
		}
		body, err := stringCmd.Bytes()
		if err != nil {
			return nil, err
		}
		obj := reflect.New(reflect.TypeOf(dst).Elem()).Interface()
		err = json.Unmarshal(body, obj)
		if err != nil {
			return nil, err
		}
		src = append(src, obj)
	}
	return src, nil
}

// getGroup 获取组详情
func (b *BackendRedis) getGroup(groupID string) (*task.GroupMeta, error) {
	body, err := b.client.Get(context.Background(), groupID).Bytes()
	if err != nil {
		return nil, err
	}
	var group task.GroupMeta
	err = json.Unmarshal(body, &group)
	return &group, err
}

// updateStatus 更新状态
func (b *BackendRedis) updateStatus(status *task.Status) error {
	body, err := json.Marshal(status)
	if err != nil {
		return err
	}
	expire := b.resultExpire
	if expire < 0 {
		expire = 0
	}
	_, err = b.client.Set(context.Background(), status.TaskID, body, time.Duration(expire)).Result()
	return err
}

// getStatus 获取任务状态
func (b *BackendRedis) getStatus(taskID string) (*task.Status, error) {
	body, err := b.client.Get(context.Background(), taskID).Bytes()
	if err != nil {
		return nil, err
	}
	var status task.Status
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.UseNumber()
	if err := decoder.Decode(&status); err != nil {
		return nil, err
	}

	return &status, nil
}

func (b *BackendRedis) migrate(dst *task.Status) {
	src, err := b.getStatus(dst.TaskID)
	if err == nil {
		dst.CreateAt = src.CreateAt
		dst.Name = src.Name
	}
}
