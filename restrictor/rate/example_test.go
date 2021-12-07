package rate

import (
	"context"
	"time"

	"golang.org/x/time/rate"
)

func ExampleNewRate() {
	// 第一个参数是 r Limit。代表每秒可以向 Token 桶中产生多少 token。Limit 实际上是 float64 的别名
	// 第二个参数是 b int。b 代表 Token 桶的容量大小。
	// limit := Every(100 * time.Millisecond);
	// limiter := rate.NewLimiter(limit, 4)
	// 以上就表示每 100ms 往桶中放一个 Token。本质上也就是一秒钟产生 10 个。

	// rate: golang.org/x/time/rate
	limiter := rate.NewLimiter(2, 4)

	af, wf := NewRate(limiter)

	// af.Allow()bool: 默认取1个token
	// af.Allow() == af.AllowN(time.Now(), 1)
	af.Allow()

	// af.AllowN(ctx,n)bool: 可以取N个token
	af.AllowN(time.Now(), 5)

	// wf.Wait(ctx) err: 等待ctx超时,默认取1个token
	// wf.Wait(ctx) == wf.WaitN(ctx, 1)
	_ = wf.Wait(context.TODO())

	// wf.WaitN(ctx, n) err: 等待ctx超时,可以取N个token
	_ = wf.WaitN(context.TODO(), 5)
}
