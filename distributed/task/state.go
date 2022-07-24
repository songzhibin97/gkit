package task

import (
	"bytes"
	"database/sql/driver"
	"errors"
	"time"

	json "github.com/json-iterator/go"

	"gorm.io/gorm"
)

type State int

const (
	// StatePending 任务初始状态
	StatePending State = iota
	// StateReceived 收到任务
	StateReceived
	// StateStarted 开始执行任务
	StateStarted
	// StateRetry 准备重试
	StateRetry
	// StateSuccess 任务成功
	StateSuccess
	// StateFailure 任务失败
	StateFailure
)

func (s State) String() string {
	switch s {
	case StatePending:
		return "PENDING"
	case StateReceived:
		return "RECEIVED"
	case StateStarted:
		return "STARTED"
	case StateRetry:
		return "RETRY"
	case StateSuccess:
		return "SUCCESS"
	case StateFailure:
		return "FAILURE"
	default:
		return "UNKNOWN"
	}
}

// NewPendingState 创建pending状态
func NewPendingState(task *Signature) *Status {
	return &Status{
		TaskID:   task.ID,
		Name:     task.Name,
		Status:   StatePending,
		GroupID:  task.GroupID,
		CreateAt: time.Now().Local(),
	}
}

// NewReceivedState 创建Received状态
func NewReceivedState(task *Signature) *Status {
	return &Status{
		TaskID:  task.ID,
		Name:    task.Name,
		Status:  StateReceived,
		GroupID: task.GroupID,
	}
}

// NewStartedState 创建Started状态
func NewStartedState(task *Signature) *Status {
	return &Status{
		TaskID:  task.ID,
		Name:    task.Name,
		Status:  StateStarted,
		GroupID: task.GroupID,
	}
}

// NewRetryState 创建Retry状态
func NewRetryState(task *Signature) *Status {
	return &Status{
		TaskID:  task.ID,
		Name:    task.Name,
		Status:  StateRetry,
		GroupID: task.GroupID,
	}
}

// NewSuccessState 创建Success状态
func NewSuccessState(task *Signature, results ...*Result) *Status {
	return &Status{
		TaskID:  task.ID,
		Name:    task.Name,
		Status:  StateSuccess,
		Results: results,
		GroupID: task.GroupID,
	}
}

// NewFailureState 创建Failure状态
func NewFailureState(task *Signature, err string) *Status {
	return &Status{
		TaskID:  task.ID,
		Name:    task.Name,
		Status:  StateFailure,
		Error:   err,
		GroupID: task.GroupID,
	}
}

type Results []*Result

func (s *Results) Scan(src interface{}) error {
	str, ok := src.([]byte)
	if !ok {
		return errors.New("failed to scan Results field - source is not a string")
	}
	decoder := json.NewDecoder(bytes.NewReader(str))
	decoder.UseNumber()
	return decoder.Decode(s)
}

func (s Results) Value() (driver.Value, error) {
	if s == nil || len(s) == 0 {
		return nil, nil
	}
	return json.Marshal(s)
}

// Status 任务状态
type Status struct {
	ID        uint           `json:"-" bson:"-" gorm:"column:_id;primarykey;comment:_id"`
	TaskID    string         `json:"task_id" bson:"_id" gorm:"column:id;index;comment:id"`
	GroupID   string         `json:"group_id" bson:"group_id" gorm:"column:group_id;comment:组唯一标识"`
	Name      string         `json:"name" bson:"name" gorm:"column:name;comment:组名称"`
	Status    State          `json:"status" bson:"status" gorm:"column:status;comment:任务状态"`
	TTL       int64          `json:"ttl" bson:"ttl" gorm:"column:ttl;comment:过期时间"`
	Error     string         `json:"error" bson:"error" gorm:"column:error;comment:错误"`
	Results   Results        `json:"results" bson:"results" gorm:"column:results;comment:结果;type:text"`
	CreateAt  time.Time      `json:"create_at" bson:"create_at" gorm:"column:create_at;comment:创建时间"`
	DeletedAt gorm.DeletedAt `json:"-" bson:"-" gorm:"index"`
}

func (t *Status) IsCompleted() bool {
	return t.IsSuccess() || t.IsFailure()
}

func (t *Status) IsSuccess() bool {
	return t.Status == StateSuccess
}

func (t *Status) IsFailure() bool {
	return t.Status == StateFailure
}
