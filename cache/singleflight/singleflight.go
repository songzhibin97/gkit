package singleflight

import "golang.org/x/sync/singleflight"

// SingleFlight Merge back to source
type SingleFlight interface {
	// Do 同步调用单飞
	Do(key string, fn func() (interface{}, error)) (v interface{}, err error, shared bool)

	// DoChan 异步调用单飞
	DoChan(key string, fn func() (interface{}, error)) <-chan singleflight.Result

	// Forget 让 singleflight 忘记 key，使未来的同 key 调用不再等待当前调用。
	// 当前调用的 fn 不会被取消，仍会继续执行并返回给它原有的调用者。
	Forget(key string)
}

type Group struct {
	// import "golang.org/x/sync/singleflight"
	singleflight.Group
}

// NewSingleFlight 实例化
func NewSingleFlight() SingleFlight {
	return &Group{}
}
