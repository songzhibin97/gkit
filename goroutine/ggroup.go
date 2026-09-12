package goroutine

import "context"

type GGroup interface {
	// ChangeMax 修改pool内goroutine数量上限,m<=0 时修正为1
	// 若此时pool未关闭且无存活goroutine,会顺带拉起一个
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
