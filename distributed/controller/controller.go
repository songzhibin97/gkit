package controller

import (
	"context"

	"github.com/songzhibin97/gkit/distributed/task"
)

type Controller interface {
	// RegisterTask 注册任务
	RegisterTask(name ...string)

	// IsRegisterTask 判断任务是否注册
	IsRegisterTask(name string) bool

	// StartConsuming 开始消费
	StartConsuming(concurrency int, handler task.Processor) (bool, error)

	// StopConsuming 停止消费
	StopConsuming()

	// Publish 任务发布
	Publish(ctx context.Context, t *task.Signature) error

	// GetPendingTasks 获取等待任务
	GetPendingTasks(queue string) ([]*task.Signature, error)

	// GetDelayedTasks 获取延时任务
	GetDelayedTasks() ([]*task.Signature, error)

	// SetConsumingQueue 设置消费队列名称
	SetConsumingQueue(consumingQueue string)

	// SetDelayedQueue 设置延迟队列名称
	SetDelayedQueue(delayedQueue string)
}
