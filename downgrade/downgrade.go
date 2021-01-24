package downgrade

import (
	"context"
	"github.com/afex/hystrix-go/hystrix"
)

// package downgrade: 熔断降级
// 与 "github.com/afex/hystrix-go/hystrix" 使用方法一致,只是做了抽象封装,避免因为升级对服务造成影响"

type runFunc = func() error
type fallbackFunc = func(error) error
type runFuncC = func(context.Context) error
type fallbackFuncC = func(context.Context, error) error

// Fuse: 熔断降级接口
type Fuse interface {
	// Do: 以同步的方式运行 runFunc,直到成功为止
	// 如果返回错误,执行 fallbackFunc 函数
	Do(name string, run runFunc, fallback fallbackFunc) error

	// Go: 异步调用返回 channel
	Go(name string, run runFunc, fallback fallbackFunc) chan error

	// GoC:
	// Do/Go 都调用GoC, Do中处理了异步过程
	GoC(ctx context.Context, name string, run runFuncC, fallback fallbackFuncC) chan error

	// ConfigureCommand: 配置参数
	ConfigureCommand(name string, config hystrix.CommandConfig)
}
