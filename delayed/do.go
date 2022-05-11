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

	last := len(d.delays) - 1
	if i != last {
		d.delays[i] = d.delays[last]
	}
	d.delays[last] = nil
	d.delays = d.delays[:last]
	smallestChanged := i
	if i != last {
		// Moving to i may have moved the last timer to a new parent,
		// so sift up to preserve the heap guarantee.
		smallestChanged = siftupDelayed(d.delays, i)
	}

	return d.delays[smallestChanged]
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
	d.RUnlock()
	if len(d.delays) == 0 {
		return BadDelayed
	}
	return d.delays[0]
}

// IsInvalid 判断任务是否有效
func (d *DispatchingDelayed) IsInvalid(delayed Delayed) bool {
	return delayed == badDelayed{}
}

// Close 关闭
func (d *DispatchingDelayed) Close() error {
	if !atomic.CompareAndSwapInt32(&d.isClose, 0, 1) {
		return ErrorRepeatShutdown
	}
	close(d.close)
	return d.pool.Shutdown()
}

// Refresh 刷新
func (d *DispatchingDelayed) Refresh() {
	select {
	case d.refresh <- struct{}{}:
	}
}

// sentinel 启动
func (d *DispatchingDelayed) sentinel() {
	go func() {
		timer := time.NewTicker(d.checkTime)
		for {
			select {
			case <-timer.C:
			case <-d.refresh:
			case <-d.close:
				// 关闭流程
				ln := len(d.delays)
				for i := 0; i < ln; i++ {
					pop := d.delDelayedTop()
					if d.IsInvalid(pop) {
						continue
					}
					d.pool.AddTask(pop.Do)
				}
				return
			}
			now := time.Now().Unix()
			for i := 0; i < len(d.delays); i++ {
				top := d.getTopDelayed()

				// 还没到达执行时间
				if top.ExecTime() > now {
					break
				}

				d.delDelayedTop()
				d.pool.AddTask(top.Do)
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
		close:   make(chan struct{}, 1),
		pool:    goroutine.NewGoroutine(context.Background(), goroutine.SetMax(1), goroutine.SetIdle(1)),
		refresh: make(chan struct{}, 1),
	}
	for _, option := range o {
		option(dispatchingDelayed)
	}
	if dispatchingDelayed.checkTime <= 0 {
		dispatchingDelayed.checkTime = time.Second
	}
	if dispatchingDelayed.Worker <= 0 {
		dispatchingDelayed.Worker = 1
	}
	if dispatchingDelayed.Worker != 1 {
		dispatchingDelayed.pool = goroutine.NewGoroutine(context.Background(), goroutine.SetMax(dispatchingDelayed.Worker), goroutine.SetIdle(dispatchingDelayed.Worker))
	}
	if len(dispatchingDelayed.signal) != 0 && dispatchingDelayed.signalCallback != nil {
		sign := make(chan os.Signal, 1)
		signal.Notify(sign, dispatchingDelayed.signal...)
		go func() {
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
