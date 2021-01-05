package goroutine

type Goer interface {
	// ChangeMax: 更改buffer大小
	ChangeMax(m int64)

	// AddTask: 添加需要 `go function`
	AddTask(f func()) bool

	// Shutdown: 回收资源
	Shutdown() error

	// trick: debug
	trick()
}
