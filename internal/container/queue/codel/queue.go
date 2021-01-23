package codel

import (
	"Songzhibin/GKit/overload/bbr"
	"context"
	"math"
	"sync"
	"time"
)

// package queue: 对列实现可控制延时算法
// CoDel 可控制延时算法

// Config: CoDel config
type Config struct {
	// Delay: 对列延时(默认是20ms)
	Delay int64

	// Width: 滑动最小时间窗口宽度(默认是500ms)
	Width int64
}

// Stat: CoDel 状态信息
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

// Queue: CoDel buffer 缓冲队列
type Queue struct {
	// dropping: 是否处于降级状态
	dropping bool

	pool    sync.Pool
	packets chan packet

	mux      sync.RWMutex
	conf     *Config
	count    int64 // 计数请求数量
	faTime   int64
	dropNext int64 // 丢弃请求的数量
}

var defaultConf = &Config{
	Delay: 20,
	Width: 500,
}

// Default: 默认配置CoDel Queue
func Default() *Queue {
	return New(defaultConf)
}

// New: 实例化 CoDel Queue
func New(conf *Config) *Queue {
	if conf == nil {
		conf = defaultConf
	}
	// new pool
	q := &Queue{
		packets: make(chan packet, 2048),
		conf:    conf,
	}
	q.pool.New = func() interface{} {
		return make(chan bool)
	}
	return q
}

// Reload: 重新加载配置
func (q *Queue) Reload(c *Config) {
	if c == nil || c.Width == 0 || c.Delay == 0 {
		return
	}
	q.mux.Lock()
	q.conf = c
	q.mux.Unlock()
}

// Stat: 返回CoDel状态信息
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

// Push: 请求进入CoDel Queue
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

// Pop: 弹出 CoDel Queue 的请求
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

// controlLaw: CoDel 控制率
func (q *Queue) controlLaw(now int64) int64 {
	q.dropNext = now + int64(float64(q.conf.Width)/math.Sqrt(float64(q.count)))
	return q.dropNext
}

// judge: 决定数据包是否丢弃
// Core: CoDel
func (q *Queue) judge(p packet) (drop bool) {
	now := time.Now().UnixNano() / int64(time.Millisecond)
	sojurn := now - p.ts
	q.mux.Lock()
	defer q.mux.Unlock()
	if sojurn < q.conf.Delay {
		q.faTime = 0
	} else if q.faTime == 0 {
		q.faTime = now + q.conf.Width
	} else if now >= q.faTime {
		drop = true
	}
	if q.dropping {
		if !drop {
			// sojourn time below target - leave dropping state
			q.dropping = false
		} else if now > q.dropNext {
			q.count++
			q.dropNext = q.controlLaw(q.dropNext)
			drop = true
			return
		}
	} else if drop && (now-q.dropNext < q.conf.Width || now-q.faTime >= q.conf.Width) {
		q.dropping = true
		// If we're in a drop cycle, the drop rate that controlled the queue
		// on the last cycle is a good starting point to control it now.
		if now-q.dropNext < q.conf.Width {
			if q.count > 2 {
				q.count = q.count - 2
			} else {
				q.count = 1
			}
		} else {
			q.count = 1
		}
		q.dropNext = q.controlLaw(now)
		drop = true
		return
	}
	return
}
