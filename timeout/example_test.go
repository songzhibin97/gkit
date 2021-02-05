package timeout

import (
	"context"
	"time"
)

func ExampleShrink() {
	// timeout.Shrink 方法提供全链路的超时控制
	// 只需要传入一个父节点的ctx 和需要设置的超时时间,他会帮你确认这个ctx是否之前设置过超时时间,
	// 如果设置过超时时间的话会和你当前设置的超时时间进行比较,选择一个最小的进行设置,保证链路超时时间不会被下游影响
	// d: 代表剩余的超时时间
	// nCtx: 新的context对象
	// cancel: 如果是成功真正设置了超时时间会返回一个cancel()方法,未设置成功会返回一个无效的cancel,不过别担心,还是可以正常调用的
	d, nCtx, cancel := Shrink(context.Background(), 5*time.Second)
	// d 根据需要判断
	// 一般判断该服务的下游超时时间,如果d过于小,可以直接放弃
	select {
	case <-nCtx.Done():
		cancel()
	default:
		// ...
	}
	_ = d
}
