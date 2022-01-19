package codel

import (
	"context"
	"math"
	"sync"
	"sync/atomic"
	"time"

	"github.com/songzhibin97/gkit/options"
	"github.com/songzhibin97/gkit/overload/bbr"
)

// package queue: 对列实现可控制延时算法
// CoDel 可控制延时算法

// config CoDel config
type config struct {
	// target: 对列延时(默认是20ms)
	target int64

	// internal: 滑动最小时间窗口宽度(默认是500ms)
	internal int64
}

// Stat CoDel 状态信息
type Stat struct {
	Dropping bool
	Packets  int64
	FaTime   int64
	DropNext int64
}

// packet:
type packet struct {
	ch chan bool
	ts int64
}

// Queue CoDel buffer 缓冲队列
type Queue struct {
	// dropping: 是否处于降级状态
	dropping bool

	pool    sync.Pool
	packets chan packet

	mux      sync.RWMutex
	conf     *config
	count    int64 // 计数请求数量
	faTime   int64
	dropNext int64 // 丢弃请求的数量
}

// Reload 重新加载配置
func (q *Queue) Reload(c *config) {
	if c == nil || c.internal <= 0 || c.target <= 0 {
		return
	}
	q.mux.Lock()
	defer q.mux.Unlock()
	q.conf = c
}

// Stat 返回CoDel状态信息
func (q *Queue) Stat() Stat {
	q.mux.Lock()
	defer q.mux.Unlock()
	return Stat{
		Dropping: q.dropping,
		FaTime:   q.faTime,
		DropNext: q.dropNext,
		Packets:  int64(len(q.packets)),
	}
}

// Push 请求进入CoDel Queue
// 如果返回错误为nil，则在完成请求处理后，调用方必须调用q.Done()
func (q *Queue) Push(ctx context.Context) (err error) {
	r := packet{
		ch: q.pool.Get().(chan bool),
		ts: time.Now().UnixNano() / int64(time.Millisecond),
	}
	select {
	case q.packets <- r:
	default:
		// 如果缓冲区阻塞,直接将 err 赋值,并且将资源放回pool中
		err = bbr.LimitExceed
		q.pool.Put(r.ch)
	}
	// 判断是否发送到缓冲区
	if err == nil {
		select {
		case drop := <-r.ch:
			// r.ch = true
			if drop {
				err = bbr.LimitExceed
			}
			q.pool.Put(r.ch)
		case <-ctx.Done():
			err = ctx.Err()
		}
	}
	return
}

// Pop 弹出 CoDel Queue 的请求
func (q *Queue) Pop() {
	for {
		select {
		case p := <-q.packets:
			drop := q.judge(p)
			select {
			case p.ch <- drop:
				if !drop {
					return
				}
			default:
				q.pool.Put(p.ch)
			}
		default:
			return
		}
	}
}

// controlLaw CoDel 控制率
func (q *Queue) controlLaw(now int64) int64 {
	atomic.StoreInt64(&q.dropNext, now+int64(float64(q.conf.internal)/math.Sqrt(float64(q.count))))
	return atomic.LoadInt64(&q.dropNext)
}

// judge 决定数据包是否丢弃
// Core: CoDel
func (q *Queue) judge(p packet) (drop bool) {
	now := time.Now().UnixNano() / int64(time.Millisecond)
	sojurn := now - p.ts
	q.mux.Lock()
	defer q.mux.Unlock()
	if sojurn < q.conf.target {
		atomic.StoreInt64(&q.faTime, 0)
	} else if atomic.LoadInt64(&q.faTime) == 0 {
		atomic.StoreInt64(&q.faTime, now+q.conf.internal)
	} else if now >= atomic.LoadInt64(&q.faTime) {
		drop = true
	}
	if q.dropping {
		if !drop {
			// sojourn time below target - leave dropping state
			q.dropping = false
		} else if now > atomic.LoadInt64(&q.dropNext) {
			atomic.AddInt64(&q.count, 1)
			q.controlLaw(atomic.LoadInt64(&q.dropNext))
			drop = true
			return
		}
	} else if drop && (now-atomic.LoadInt64(&q.dropNext) < q.conf.internal || now-atomic.LoadInt64(&q.faTime) >= q.conf.internal) {
		q.dropping = true
		// If we're in a drop cycle, the drop rate that controlled the queue
		// on the last cycle is a good starting point to control it now.
		if now-atomic.LoadInt64(&q.dropNext) < q.conf.internal {
			if atomic.LoadInt64(&q.count) > 2 {
				atomic.AddInt64(&q.count, -2)
			} else {
				atomic.StoreInt64(&q.count, 1)
			}
		} else {
			atomic.StoreInt64(&q.count, 1)
		}
		q.controlLaw(now)
		drop = true
		return
	}
	return
}

// Default 默认配置CoDel Queue
func Default() *Queue {
	return NewQueue()
}

// defaultConfig 默认配置
func defaultConfig() *config {
	return &config{
		target:   20,
		internal: 500,
	}
}

// Option

// SetTarget 设置对列延时
func SetTarget(target int64) options.Option {
	return func(c interface{}) {
		c.(*config).target = target
	}
}

// SetInternal 设置滑动窗口最小时间宽度
func SetInternal(internal int64) options.Option {
	return func(c interface{}) {
		c.(*config).internal = internal
	}
}

// NewQueue 实例化 CoDel Queue
func NewQueue(options ...options.Option) *Queue {
	// new pool
	q := &Queue{
		packets: make(chan packet, 2048),
		conf:    defaultConfig(),
	}
	for _, option := range options {
		option(q.conf)
	}
	q.pool.New = func() interface{} {
		return make(chan bool)
	}
	return q
}
