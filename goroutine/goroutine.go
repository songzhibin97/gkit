package goroutine

// package goroutine: 管理goroutine并发量托管任务以及兜底

import (
	"context"
	"errors"
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"github.com/songzhibin97/gkit/cache/buffer"

	"github.com/songzhibin97/gkit/log"
	"github.com/songzhibin97/gkit/options"
	"github.com/songzhibin97/gkit/timeout"
)

var ErrRepeatClose = errors.New("goroutine/goroutine :重复关闭")

type Goroutine struct {
	close int32

	// n: 当前goroutine的数量
	n int64
	// 参数选项
	config
	// wait
	wait sync.WaitGroup
	// ctx context
	ctx context.Context
	// cancel
	cancel context.CancelFunc
	// task
	task chan func()
	// growMu serialises pool growth (_go's wg.Add) against Shutdown's
	// wg.Wait — without it, wg.Add can race with wg.Wait and trip
	// `sync: WaitGroup misuse`. It also guards the close flag flip so that
	// no new worker can be spawned after Shutdown has begun waiting.
	growMu sync.Mutex
}

// _go must be called with g.growMu held; it bumps the WaitGroup and spawns a
// worker. Workers exit on ctx cancellation; we deliberately do NOT close
// g.task because Shutdown could otherwise race with AddTask's send and
// panic.
func (g *Goroutine) _go() {
	atomic.AddInt64(&g.n, 1)
	g.wait.Add(1)
	go func() {
		// recover 避免野生goroutine panic后主程退出
		defer func() {
			if err := recover(); err != nil {
				buf := buffer.GetBytes(64 << 10)
				n := runtime.Stack(*buf, false)
				defer buffer.PutBytes(buf)
				if g.logger == nil {
					fmt.Println("\nrecover go func,", "panic:", err, "\n\npanic stack:\n", string((*buf)[:n]))
					return
				}
				g.logger.Log(log.LevelError, "panic err:", err, "panic stack:", string((*buf)[:n]))
				return
			}
		}()
		defer atomic.AddInt64(&g.n, -1)
		defer g.wait.Done()
		t := time.NewTicker(g.checkTime)
		defer t.Stop()
		for {
			select {
			case <-t.C:
				// 当前的g的个数大于设置的闲置值,则退出
				if atomic.LoadInt64(&g.n) > atomic.LoadInt64(&g.idle) {
					return
				}
			case f := <-g.task:
				f()
				if atomic.LoadInt64(&g.n) > atomic.LoadInt64(&g.max) {
					return
				}
				t.Reset(g.checkTime)
			case <-g.ctx.Done():
				return
			}
		}
	}()
}

// AddTask 添加任务
func (g *Goroutine) AddTask(f func()) (ok bool) {
	g.growMu.Lock()
	if atomic.LoadInt32(&g.close) != 0 {
		g.growMu.Unlock()
		return false
	}
	// Fast path: a worker is already parked.
	select {
	case g.task <- f:
		g.growMu.Unlock()
		return true
	default:
	}
	// Slow path: grow the pool if we still can, then hand off outside the lock
	// so other AddTask callers don't block on us.
	if atomic.LoadInt64(&g.n) < atomic.LoadInt64(&g.max) {
		g._go()
	}
	g.growMu.Unlock()
	select {
	case g.task <- f:
		return true
	case <-g.ctx.Done():
		return false
	}
}

// AddTaskN 添加任务 有超时时间
func (g *Goroutine) AddTaskN(ctx context.Context, f func()) (ok bool) {
	g.growMu.Lock()
	if atomic.LoadInt32(&g.close) != 0 {
		g.growMu.Unlock()
		return false
	}
	if atomic.LoadInt64(&g.max) > atomic.LoadInt64(&g.n) {
		g._go()
	}
	g.growMu.Unlock()
	select {
	case <-ctx.Done():
		return false
	case <-g.ctx.Done():
		return false
	case g.task <- f:
		return true
	}
}

// ChangeMax 修改pool上限值
func (g *Goroutine) ChangeMax(m int64) {
	if m <= 0 {
		m = 1
	}
	g.growMu.Lock()
	defer g.growMu.Unlock()
	atomic.StoreInt64(&g.max, m)
	if atomic.LoadInt32(&g.close) == 0 && atomic.LoadInt64(&g.n) == 0 {
		g._go()
	}
}

// Shutdown 优雅关闭
// 符合幂等性
func (g *Goroutine) Shutdown() error {
	g.growMu.Lock()
	if !atomic.CompareAndSwapInt32(&g.close, 0, 1) {
		g.growMu.Unlock()
		return ErrRepeatClose
	}
	g.growMu.Unlock()
	g.cancel()
	// Workers exit via ctx.Done(); we never close g.task because AddTask
	// callers may still be racing to send and would otherwise panic.
	err := Delegate(context.TODO(), g.stopTimeout, func(context.Context) error {
		g.wait.Wait()
		return nil
	})
	if g.logger != nil {
		g.logger.Log(log.LevelDebug, err)
	}
	return err
}

// Trick Debug使用
func (g *Goroutine) Trick() string {
	if g.logger != nil {
		g.logger.Log(log.LevelDebug, "max:", atomic.LoadInt64(&g.max), "idle:", atomic.LoadInt64(&g.idle), "now goroutine", atomic.LoadInt64(&g.n), "task len:", len(g.task))
	}
	return fmt.Sprintln("max:", atomic.LoadInt64(&g.max), "idle:", atomic.LoadInt64(&g.idle), "now goroutine:", atomic.LoadInt64(&g.n), "task len:", len(g.task))
}

// Delegate 委托执行 一般用于回收函数超时控制
func Delegate(c context.Context, t time.Duration, f func(ctx context.Context) error) error {
	ch := make(chan error, 1)
	var cancel context.CancelFunc
	if t > 0 {
		_, c, cancel = timeout.Shrink(c, t)
	} else {
		c, cancel = context.WithCancel(c)
	}
	defer cancel()
	go func() {
		defer func() {
			if err := recover(); err != nil {
				switch e := err.(type) {
				case string:
					ch <- errors.New(e)
				case error:
					ch <- e
				default:
					ch <- fmt.Errorf("%+v", err)
				}
				return
			}
		}()
		// Pass the shrunk context to f so f's own ctx-aware code honours the
		// caller-requested timeout. Previously fctx captured the un-shrunk
		// context and the deadline was visible only on Delegate's outer
		// select, letting f outlive its timeout.
		ch <- f(c)
	}()
	select {
	case <-c.Done():
		return c.Err()
	case err := <-ch:
		return err
	}
}

// NewGoroutine 实例化方法
func NewGoroutine(ctx context.Context, opts ...options.Option) GGroup {
	ctx, cancel := context.WithCancel(ctx)
	o := config{
		stopTimeout: 10 * time.Second,
		max:         1000,
		idle:        1000,
		checkTime:   10 * time.Minute,
	}
	for _, opt := range opts {
		opt(&o)
	}
	if o.checkTime <= 0 {
		o.checkTime = 10 * time.Minute
	}
	if o.max <= 0 {
		o.max = 1
	}
	g := &Goroutine{
		ctx:    ctx,
		cancel: cancel,
		// 为什么设置0
		// task buffer 理论上如果比较大,调度可能会延迟
		task:   make(chan func(), 0),
		config: o,
	}
	if o.idle > o.max {
		o.idle = o.max
	}
	// 预加载出idle池,避免阻塞在buffer中
	g.growMu.Lock()
	for i := int64(0); i < o.idle; i++ {
		g._go()
	}
	g.growMu.Unlock()
	return g
}
