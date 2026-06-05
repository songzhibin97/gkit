package delayed

import (
	"context"
	"errors"
	"github.com/songzhibin97/gkit/goroutine"
	"github.com/songzhibin97/gkit/options"
	"os"
	"os/signal"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
)

var BadDelayed = &badDelayed{}
var ErrorRepeatShutdown = errors.New("重复关闭")

type badDelayed struct{}

func (b badDelayed) Do() {
	return
}

func (b badDelayed) ExecTime() int64 {
	return 0
}

func (b badDelayed) Identify() string {
	return "badDelayed"
}

// DispatchingDelayed 调度延时任务.
// Concurrency safety
type DispatchingDelayed struct {
	sync.RWMutex
	delays         []Delayed
	checkTime      time.Duration                                 // 检查时间
	Worker         int64                                         // 并发数(实际执行任务)
	signal         []os.Signal                                   // 接受注册的信号
	signalCallback func(signal os.Signal, d *DispatchingDelayed) // 接受到信号的回调
	close          chan struct{}                                 // 内部使用(标记是否已经关闭)
	isClose        int32
	pool           goroutine.GGroup // 并发内部使用的pool // min 1
	refresh        chan struct{}    // 强行刷新
	sentinelDone   chan struct{}    // sentinel 退出信号
	closeCtx       context.Context  // 取消后唤醒 sentinel 卡在 AddTaskN 的提交
	closeCancel    context.CancelFunc
	shutdownErr    error // pool.Shutdown 结果；sentinel 写、Close 读（经 sentinelDone 同步）
}

// AddDelayed 添加延时任务
func (d *DispatchingDelayed) AddDelayed(delayed Delayed) {
	if delayed.ExecTime() <= 0 {
		// 无效任务
		return
	}
	if atomic.LoadInt32(&d.isClose) == 1 {
		return
	}

	d.Lock()
	defer d.Unlock()
	// Re-check under the lock: Close() may have set isClose (and the sentinel
	// may have cleared d.delays) between the atomic check above and this lock;
	// appending now would re-populate — leak into — a closed dispatcher.
	if atomic.LoadInt32(&d.isClose) == 1 {
		return
	}

	i := len(d.delays)
	d.delays = append(d.delays, delayed)
	siftupDelayed(d.delays, i)
}

func (d *DispatchingDelayed) delDelayed(i int) Delayed {
	d.Lock()
	defer d.Unlock()

	if i >= len(d.delays) {
		return BadDelayed
	}

	ret := d.delays[i]
	last := len(d.delays) - 1
	if i != last {
		d.delays[i] = d.delays[last]
	}
	d.delays[last] = nil
	d.delays = d.delays[:last]
	if i != last {
		siftupDelayed(d.delays, i)
		siftdownDelayed(d.delays, i)
	}

	return ret
}

// delDelayedTop pop最小时间
func (d *DispatchingDelayed) delDelayedTop() Delayed {
	d.Lock()
	defer d.Unlock()

	if len(d.delays) == 0 {
		return BadDelayed
	}

	ret := d.delays[0]
	last := len(d.delays) - 1
	if last > 0 {
		d.delays[0] = d.delays[last]
	}

	d.delays[last] = nil
	d.delays = d.delays[:last]
	if last > 0 {
		siftdownDelayed(d.delays, 0)
	}
	return ret
}

// getTopDelayed 获取下一个需要执行的任务
func (d *DispatchingDelayed) getTopDelayed() Delayed {
	d.RLock()
	defer d.RUnlock()
	if len(d.delays) == 0 {
		return BadDelayed
	}
	return d.delays[0]
}

// popIfReady 在持锁状态下检查 top 是否已到达执行时间；
// 是则 pop 并返回，否则返回 BadDelayed。
// 把"判断 + pop"合到同一把锁里，避免 sentinel 中无锁读 len(d.delays)
// 以及 getTopDelayed/delDelayedTop 分离造成的 TOCTOU。
func (d *DispatchingDelayed) popIfReady(now int64) Delayed {
	d.Lock()
	defer d.Unlock()
	if len(d.delays) == 0 {
		return BadDelayed
	}
	if d.delays[0].ExecTime() > now {
		return BadDelayed
	}
	ret := d.delays[0]
	last := len(d.delays) - 1
	if last > 0 {
		d.delays[0] = d.delays[last]
	}
	d.delays[last] = nil
	d.delays = d.delays[:last]
	if last > 0 {
		siftdownDelayed(d.delays, 0)
	}
	return ret
}

// IsInvalid 判断任务是否有效
func (d *DispatchingDelayed) IsInvalid(delayed Delayed) bool {
	return delayed == BadDelayed
}

// Close 关闭
func (d *DispatchingDelayed) Close() error {
	if !atomic.CompareAndSwapInt32(&d.isClose, 0, 1) {
		return ErrorRepeatShutdown
	}
	// The sentinel is the SOLE owner of the pool's AddTask/Shutdown, so it must
	// be the one to shut the pool down (a Shutdown here would race the
	// sentinel's in-flight AddTaskN send vs close(g.task)). Cancel closeCtx to
	// unblock any submit stuck on the unbuffered task channel, signal the
	// sentinel to drain, then wait for it and surface its shutdown error.
	d.closeCancel()
	close(d.close)
	<-d.sentinelDone
	return d.shutdownErr
}

// Refresh 刷新
func (d *DispatchingDelayed) Refresh() {
	select {
	case d.refresh <- struct{}{}:
	default:
	}
}

// sentinel 启动
func (d *DispatchingDelayed) sentinel() {
	go func() {
		// Signal Close() that the sentinel has fully drained, shut the pool
		// down, and returned. The sentinel is the only goroutine that calls
		// pool.AddTaskN / pool.Shutdown, so those can never race each other.
		defer close(d.sentinelDone)
		// Stop the ticker on exit; previously it kept firing for the lifetime
		// of the process on every DispatchingDelayed close.
		timer := time.NewTicker(d.checkTime)
		defer timer.Stop()
		for {
			select {
			case <-timer.C:
			case <-d.refresh:
			case <-d.close:
				// 关闭流程：丢弃尚未提交的待执行任务并关闭 pool。
				// On close, drop any pending (not-yet-submitted) tasks and shut
				// the pool down. The sentinel is the sole owner of the pool's
				// AddTaskN/Shutdown, so Shutdown here can't race a submit, and
				// already-submitted tasks finish under the pool's stop timeout.
				// Release the dropped tasks' references so a closed dispatcher
				// doesn't retain every queued closure.
				d.Lock()
				d.delays = nil
				d.Unlock()
				d.shutdownErr = d.pool.Shutdown()
				return
			}
			now := time.Now().Unix()
			for {
				top := d.popIfReady(now)
				if d.IsInvalid(top) {
					break
				}
				// Cancellable submit: Close() cancels closeCtx so a send stuck
				// on the unbuffered task channel (all workers busy) unblocks and
				// the sentinel can always reach the close branch.
				d.pool.AddTaskN(d.closeCtx, top.Do)
			}
		}
	}()
}

// NewDispatchingDelayed 初始化调度实例
func NewDispatchingDelayed(o ...options.Option) *DispatchingDelayed {
	dispatchingDelayed := &DispatchingDelayed{
		checkTime: time.Second,
		Worker:    1,
		signal:    []os.Signal{syscall.SIGINT},
		signalCallback: func(signal os.Signal, d *DispatchingDelayed) {
			_ = d.Close()
		},
		close:        make(chan struct{}, 1),
		refresh:      make(chan struct{}, 1),
		sentinelDone: make(chan struct{}),
	}
	dispatchingDelayed.closeCtx, dispatchingDelayed.closeCancel = context.WithCancel(context.Background())
	for _, option := range o {
		option(dispatchingDelayed)
	}
	if dispatchingDelayed.checkTime <= 0 {
		dispatchingDelayed.checkTime = time.Second
	}
	if dispatchingDelayed.Worker <= 0 {
		dispatchingDelayed.Worker = 1
	}
	// Create the pool exactly once with the resolved Worker count. The
	// previous code allocated a Worker=1 pool unconditionally inside the
	// struct literal, then discarded it (along with its idle goroutines,
	// ticker, and chan) when Worker != 1.
	dispatchingDelayed.pool = goroutine.NewGoroutine(
		context.Background(),
		goroutine.SetMax(dispatchingDelayed.Worker),
		goroutine.SetIdle(dispatchingDelayed.Worker),
	)
	if len(dispatchingDelayed.signal) != 0 && dispatchingDelayed.signalCallback != nil {
		sign := make(chan os.Signal, 1)
		signal.Notify(sign, dispatchingDelayed.signal...)
		go func() {
			// Match Notify with Stop on goroutine exit; the previous code
			// leaked the signal forwarder for every DispatchingDelayed
			// instance, accumulating handlers across the process lifetime.
			defer signal.Stop(sign)
			for {
				select {
				case <-dispatchingDelayed.close:
					return
				case v := <-sign:
					dispatchingDelayed.signalCallback(v, dispatchingDelayed)
				}
			}
		}()
	}
	dispatchingDelayed.sentinel()
	return dispatchingDelayed
}
