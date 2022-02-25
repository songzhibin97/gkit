package downgrade

import (
	"context"

	"github.com/afex/hystrix-go/hystrix"
)

// package downgrade: 熔断降级
// 与 "github.com/afex/hystrix-go/hystrix" 使用方法一致,只是做了抽象封装,避免因为升级对服务造成影响"

type (
	RunFunc       = func() error
	FallbackFunc  = func(error) error
	RunFuncC      = func(context.Context) error
	FallbackFuncC = func(context.Context, error) error
)

// Fuse 熔断降级接口
type Fuse interface {
	// Do 以同步的方式运行 RunFunc,直到成功为止
	// 如果返回错误,执行 FallbackFunc 函数
	Do(name string, run RunFunc, fallback FallbackFunc) error

	// DoC 同步方式处理
	DoC(ctx context.Context, name string, run RunFuncC, fallback FallbackFuncC) error

	// Go 异步调用返回 channel
	Go(name string, run RunFunc, fallback FallbackFunc) chan error

	// GoC
	// Do/Go 都调用GoC, Do中处理了异步过程
	GoC(ctx context.Context, name string, run RunFuncC, fallback FallbackFuncC) chan error

	// ConfigureCommand 配置参数
	ConfigureCommand(name string, config hystrix.CommandConfig)
}
