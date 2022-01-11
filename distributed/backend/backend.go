package backend

import "github.com/songzhibin97/gkit/distributed/task"

type Backend interface {
	// GroupTakeOver 组接管任务详情
	GroupTakeOver(groupID string, name string, taskIDs ...string) error

	// GroupCompleted 组任务是否完成
	GroupCompleted(groupID string) (bool, error)

	// GroupTaskStatus 组任务状态
	GroupTaskStatus(groupID string) ([]*task.Status, error)

	// TriggerCompleted 任务全部完成后更改标记位
	// TriggerCompleted 是并发安全的,保证只能成功更改一次
	TriggerCompleted(groupID string) (bool, error)

	// 设置任务状态

	// SetStatePending 设置任务状态为等待
	SetStatePending(task *task.Task) error

	// SetStateReceived 设置任务状态为接受
	SetStateReceived(task *task.Task) error

	// SetStateStarted 设置任务状态为开始
	SetStateStarted(task *task.Task) error

	// SetStateRetry 设置任务状态为重试
	SetStateRetry(task *task.Task) error

	// SetStateSuccess 设置任务状态为成功
	SetStateSuccess(task *task.Task, results []*task.Result) error

	// SetStateFailure 设置任务状态为失败
	SetStateFailure(task *task.Task, err string) error

	// GetStatus 获取任务状态
	GetStatus(taskID string) (*task.Status, error)

	// ResetTask 重置任务状态
	ResetTask(taskIDs ...string) error

	// ResetGroup 重置组信息
	ResetGroup(groupIDs ...string) error
}
