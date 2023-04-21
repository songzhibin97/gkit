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
}

// _go 封装goroutine 使其安全执行
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
				// recover panic
				if g.logger == nil {
					fmt.Println("recover go func,", "panic err:", err, "panic stack:", string((*buf)[:n]))
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
					// 闲置数超过预期
					return
				}
			case f, ok := <-g.task:
				// channel已经被关闭
				if !ok {
					return
				}
				// 执行函数
				f()
				if atomic.LoadInt64(&g.n) > atomic.LoadInt64(&g.max) {
					// 如果已经超出预定值,则该goroutine退出
					return
				}
				t.Reset(g.checkTime)
			case <-g.ctx.Done():
				// 触发ctx退出
				return
			}
		}
	}()
}

// AddTask 添加任务
// 直到添加成功为止
func (g *Goroutine) AddTask(f func()) bool {
	// 判断channel是否关闭
	if atomic.LoadInt32(&g.close) != 0 {
		return false
	}
	// 尝试直接塞入
	// 如果阻塞尝试进行扩容
	select {
	case g.task <- f:
	default:
		if atomic.LoadInt64(&g.n) < atomic.LoadInt64(&g.max) {
			g._go()
		}
		g.task <- f
	}
	return true
}

// AddTaskN 添加任务 有超时时间
func (g *Goroutine) AddTaskN(ctx context.Context, f func()) bool {
	// 判断channel是否关闭
	if atomic.LoadInt32(&g.close) != 0 {
		return false
	}
	if atomic.LoadInt64(&g.max) > atomic.LoadInt64(&g.n) {
		g._go()
	}
	select {
	case <-ctx.Done():
		return false
	case g.task <- f:
		return true
	}
}

// ChangeMax 修改pool上限值
func (g *Goroutine) ChangeMax(m int64) {
	atomic.StoreInt64(&g.max, m)
}

// Shutdown 优雅关闭
// 符合幂等性
func (g *Goroutine) Shutdown() error {
	if atomic.SwapInt32(&g.close, 1) == 1 {
		return ErrRepeatClose
	}
	g.cancel()
	close(g.task)
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
	go func() {
		defer func() {
			if err := recover(); err != nil {
				// panic兜底
				switch e := err.(type) {
				case string:
					ch <- errors.New(e)
				case error:
					ch <- e
				default:
					ch <- errors.New(fmt.Sprintf("%+v\n", err))
				}
				return
			}
		}()
		ch <- f(c)
	}()
	// 增加优雅退出超时控制
	var (
		cancel context.CancelFunc
	)
	if t > 0 {
		_, c, cancel = timeout.Shrink(c, t)
	} else {
		c, cancel = context.WithCancel(c)
	}
	defer cancel()
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
	for i := int64(0); i < o.idle; i++ {
		g._go()
	}
	return g
}
