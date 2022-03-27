package goroutine

import "context"

type GGroup interface {
	// ChangeMax 更改buffer大小
	ChangeMax(m int64)

	// AddTask 添加需要 `go function`
	AddTask(f func()) bool

	// AddTaskN 异步添加任务,有超时机制
	AddTaskN(ctx context.Context, f func()) bool

	// Shutdown 回收资源
	Shutdown() error

	// Trick debug
	Trick() string
}
