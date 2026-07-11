package backend_redis

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"reflect"
	"time"

	json "github.com/json-iterator/go"

	"github.com/go-redsync/redsync/v4/redis/goredis/v8"
	"github.com/songzhibin97/gkit/distributed/task"

	"github.com/go-redis/redis/v8"
	"github.com/go-redsync/redsync/v4"
	"github.com/songzhibin97/gkit/distributed/backend"
	"github.com/songzhibin97/gkit/log"
)

var defaultResultExpire int64 = 3600

var ErrType = errors.New("err type")

// ErrGroupAlreadyExists is kept as an alias for callers that historically
// imported the Redis-specific sentinel. New code can use the shared backend
// contract instead.
var ErrGroupAlreadyExists = backend.ErrGroupAlreadyExists

type persistedTaskStatus struct {
	*task.Status
	PublicationAttemptID string `json:"publication_attempt_id,omitempty"`
}

var failPendingAttemptScript = redis.NewScript(`
local raw = redis.call("GET", KEYS[1])
if not raw then
  return 0
end

local current = cjson.decode(raw)
if tonumber(current.status) ~= tonumber(ARGV[2]) then
  return 0
end
if current.publication_attempt_id ~= ARGV[1] then
  return 0
end

local ttl = redis.call("PTTL", KEYS[1])
if ttl == 0 or ttl == -2 then
  return 0
end

current.status = tonumber(ARGV[3])
current.error = ARGV[4]
local updated = cjson.encode(current)
if ttl > 0 then
  redis.call("SET", KEYS[1], updated, "PX", ttl)
else
  redis.call("SET", KEYS[1], updated)
end
return 1
`)

type BackendRedis struct {
	// client redis客户端
	client redis.UniversalClient
	// lock 分布式锁
	lock *redsync.Redsync
	// helper 结构化日志，TriggerCompleted 等路径会用它记录不可降级的运行时错误
	helper *log.Helper
	// resultExpire 数据过期时间
	// -1 代表永不过期
	// 0 会设置默认过期时间
	// 单位为s
	resultExpire int64
}

// SetHelper installs the structured-log helper. Defaults to log.DefaultLogger
// if not called.
func (b *BackendRedis) SetHelper(h *log.Helper) {
	if h != nil {
		b.helper = h
	}
}

func NewBackendRedis(client redis.UniversalClient, resultExpire int64) backend.Backend {
	b := &BackendRedis{
		client:       client,
		lock:         redsync.New(goredis.NewPool(client)),
		helper:       log.NewHelper(log.DefaultLogger),
		resultExpire: resultExpire,
	}
	if b.resultExpire == 0 {
		b.resultExpire = defaultResultExpire
	}
	return b
}

// SetResultExpire 设置结果超时时间
// expire == 0 时回落到 defaultResultExpire，与 NewBackendRedis 的语义保持一致
func (b *BackendRedis) SetResultExpire(expire int64) {
	if expire == 0 {
		expire = defaultResultExpire
	}
	b.resultExpire = expire
}

func (b *BackendRedis) GroupTakeOver(groupID string, name string, taskIDs ...string) error {
	group := task.InitGroupMeta(groupID, name, b.resultExpire, taskIDs...)
	body, err := json.Marshal(group)
	if err != nil {
		return err
	}
	expire := b.resultExpire
	// resultExpire == -1 表示永不过期；go-redis 收到 0 即不设置 TTL
	if expire < 0 {
		expire = 0
	}
	// Avoid overwriting an existing group record, but do not wait for its TTL.
	// A duplicate identifier is a caller-visible conflict; retrying forever
	// leaks every timed invocation that reuses that identifier.
	ok, err := b.client.SetNX(context.Background(), groupID, body, time.Duration(expire)*time.Second).Result()
	if err != nil {
		return fmt.Errorf("take over group %q: %w", groupID, err)
	}
	if !ok {
		return fmt.Errorf("take over group %q: %w", groupID, ErrGroupAlreadyExists)
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
	// redsync.Mutex.Unlock returns (bool, error); the bare `defer l.Unlock()`
	// silently dropped both. An Unlock failure (clock-skew, owner-mark
	// mismatch) is genuinely operational — surface it through the logger
	// rather than vanishing.
	defer func() {
		if _, err := l.Unlock(); err != nil {
			b.helper.Errorf("redsync unlock TriggerCompleted/%s: %v", groupID, err)
		}
	}()
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
	// resultExpire == -1 表示永不过期；go-redis 收到 0 即不设置 TTL
	if expire < 0 {
		expire = 0
	}
	err = b.client.Set(context.Background(), groupID, body, time.Duration(expire)*time.Second).Err()
	if err != nil {
		return false, err
	}
	return true, nil
}

func (b *BackendRedis) SetStatePending(signature *task.Signature) error {
	return b.updateStatus(task.NewPendingState(signature))
}

func (b *BackendRedis) SetStatePendingAttempt(signature *task.Signature, attemptID string) error {
	if attemptID == "" {
		return errors.New("backend_redis: empty publication attempt ID")
	}
	return b.updateStatusWithAttempt(task.NewPendingState(signature), attemptID)
}

func (b *BackendRedis) FailPendingAttempt(signature *task.Signature, attemptID, reason string) (bool, error) {
	if attemptID == "" {
		return false, nil
	}
	changed, err := failPendingAttemptScript.Run(
		context.Background(),
		b.client,
		[]string{signature.ID},
		attemptID,
		int(task.StatePending),
		int(task.StateFailure),
		reason,
	).Int()
	if err != nil {
		return false, err
	}
	return changed == 1, nil
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
	return b.updateStatusWithAttempt(status, "")
}

func (b *BackendRedis) updateStatusWithAttempt(status *task.Status, attemptID string) error {
	body, err := json.Marshal(&persistedTaskStatus{
		Status:               status,
		PublicationAttemptID: attemptID,
	})
	if err != nil {
		return err
	}
	expire := b.resultExpire
	// resultExpire == -1 表示永不过期；go-redis 收到 0 即不设置 TTL
	if expire < 0 {
		expire = 0
	}
	_, err = b.client.Set(context.Background(), status.TaskID, body, time.Duration(expire)*time.Second).Result()
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
