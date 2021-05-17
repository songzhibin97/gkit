package singleflight

import "golang.org/x/sync/singleflight"

// SingleFlight Merge back to source
type SingleFlight interface {
	// Do 同步调用单飞
	Do(key string, fn func() (interface{}, error)) (v interface{}, err error, shared bool)

	// DoChan 异步调用单飞
	DoChan(key string, fn func() (interface{}, error)) <-chan singleflight.Result

	// Forget 可以取消已经下发未执行的任务
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
