package singleflight

type Singler interface {
	// Do: 同步调用单飞
	Do(key string, fn func() (interface{}, error)) (v interface{}, err error, shared bool)

	// DoChan: 异步调用单飞
	DoChan(key string, fn func() (interface{}, error)) <-chan Result

	// Forget: 可以取消已经下发未执行的任务
	Forget(key string)
}
