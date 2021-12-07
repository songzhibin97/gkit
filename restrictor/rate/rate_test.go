package rate

import (
	"context"
	"testing"
	"time"

	"golang.org/x/time/rate"
)

func TestRate(t *testing.T) {
	// 第一个参数是 r Limit。代表每秒可以向 Token 桶中产生多少 token。Limit 实际上是 float64 的别名
	// 第二个参数是 b int。b 代表 Token 桶的容量大小。
	// limit := Every(100 * time.Millisecond);
	// limiter := rate.NewLimiter(limit, 4)
	// 以上就表示每 100ms 往桶中放一个 Token。本质上也就是一秒钟产生 10 个。
	limiter := rate.NewLimiter(2, 4)
	af, wf := NewRate(limiter)
	// 暂停3秒,等待桶满
	time.Sleep(3 * time.Second)
	for i := 0; i < 10; i++ {
		t.Log("i:", i, af.Allow())
	}

	// 暂停3秒,等待桶满
	time.Sleep(3 * time.Second)
	for i := 0; i < 5; i++ {
		t.Log("i:", i, af.AllowN(time.Now(), 2))
	}

	// 暂停3秒,等待桶满
	time.Sleep(3 * time.Second)
	for i := 0; i < 10; i++ {
		func(i int) {
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			t.Log("i:", i, wf.Wait(ctx))
		}(i)
	}
	// 暂停3秒,等待桶满
	time.Sleep(3 * time.Second)
	for i := 0; i < 5; i++ {
		func(i int) {
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			t.Log("i:", i, wf.WaitN(ctx, 2))
		}(i)
	}
}
