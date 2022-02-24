package downgrade

import (
	"context"

	"github.com/afex/hystrix-go/hystrix"
)

var fuse Fuse

// type RunFunc = func() error
// type FallbackFunc = func(error) error
// type RunFuncC = func(context.Context) error
// type FallbackFuncC = func(context.Context, error) error

func mockRunFunc() RunFunc {
	return func() error {
		return nil
	}
}

func mockFallbackFunc() FallbackFunc {
	return func(err error) error {
		return nil
	}
}

func mockRunFuncC() RunFuncC {
	return func(ctx context.Context) error {
		return nil
	}
}

func mockFallbackFuncC() FallbackFuncC {
	return func(ctx context.Context, err error) error {
		return nil
	}
}

func ExampleNewFuse() {
	// 拿到一个熔断器
	fuse = NewFuse()
}

func ExampleHystrix_ConfigureCommand() {
	// 不设置 ConfigureCommand 走默认配置
	// hystrix.CommandConfig{} 设置参数
	fuse.ConfigureCommand("test", hystrix.CommandConfig{})
}

func ExampleHystrix_Do() {
	// Do: 同步执行 func() error, 没有超时控制 直到等到返回,
	// 如果返回 error != nil 则触发 FallbackFunc 进行降级
	err := fuse.Do("do", mockRunFunc(), mockFallbackFunc())
	if err != nil {
		// 处理 error
	}
}

func ExampleHystrix_Go() {
	// Go: 异步执行 返回 channel
	ch := fuse.Go("go", mockRunFunc(), mockFallbackFunc())
	if err := <-ch; err != nil {
		// 处理 error
	}
}

func ExampleHystrix_GoC() {
	// GoC: Do/Go 实际上最终调用的就是GoC, Do主处理了异步过程
	// GoC可以传入 context 保证链路超时控制
	fuse.GoC(context.TODO(), "goc", mockRunFuncC(), mockFallbackFuncC())
}
