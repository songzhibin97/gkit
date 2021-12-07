package bbr

import (
	"context"

	"github.com/songzhibin97/gkit/overload"
)

func ExampleNewGroup() {
	group := NewGroup()
	// 如果没有就会创建
	limiter := group.Get("key")
	f, err := limiter.Allow(context.TODO())
	if err != nil {
		// 代表已经过载了,服务不允许接入
		return
	}
	// Op:流量实际的操作类型回写记录指标
	f(overload.DoneInfo{Op: overload.Success})
}

func ExampleNewLimiter() {
	// 建立Group 中间件
	middle := NewLimiter()

	// 在middleware中
	// ctx中携带这两个可配置的有效数据
	// 可以通过 ctx.Set

	// 配置获取限制器类型,可以根据不同api获取不同的限制器
	ctx := context.WithValue(context.TODO(), LimitKey, "key")

	// 可配置成功是否上报
	// 必须是 overload.Op 类型
	ctx = context.WithValue(ctx, LimitOp, overload.Success)

	_ = middle
}
