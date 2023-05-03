package pool

import (
	"container/list"
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/songzhibin97/gkit/options"
	"github.com/songzhibin97/gkit/timeout"
)

var _ Pool = &List{}

// List 双向链表
type List struct {
	// f: item
	f func(ctx context.Context) (IShutdown, error)

	// mu: 互斥锁, 保护以下字段
	mu sync.Mutex

	// cond: 发送信号,通知有回收动作,在等待的可以再次尝试获取资源
	cond chan struct{}

	// cleanerCh: 清空 ch
	cleanerCh chan struct{}

	// active: 最大连接数
	active uint64

	// conf: 配置信息
	conf *config

	// closed:
	closed uint32

	// idles: 链表
	idles list.List
}

// Reload 重新设置配置文件
func (l *List) Reload(options ...options.Option) {
	l.mu.Lock()
	defer l.mu.Unlock()
	for _, option := range options {
		option(l.conf)
	}
}

// Init 初始化
func (l *List) Init(d time.Duration) {
	// 如果 <= 0 放弃设置
	if d <= 0 {
		return
	}
	// 如果时间间隔d小于等待超时,并且 cleanerCh 不为nil 监听信号
	if d < l.conf.idleTimeout && l.cleanerCh != nil {
		select {
		// 发送立即清除旧配置的信号,如果阻塞说明在时间周期内进行清洁,跳过
		case l.cleanerCh <- struct{}{}:
		default:
		}
	}
	// 懒加载
	if l.cleanerCh == nil {
		l.cleanerCh = make(chan struct{}, 1)
		// 开启定时任务
		go l.Timer(l.conf.idleTimeout)
	}
}

// Timer 定时任务
func (l *List) Timer(d time.Duration) {
	if d < minDuration {
		d = minDuration
	}
	// ticker: 定时任务
	ticker := time.NewTicker(d)
	for {
		select {
		// 触发条件:
		// 1. 定时周期
		// 2. l.cleanerCh 接收到信号
		case <-ticker.C:
		case <-l.cleanerCh:
		}
		l.mu.Lock()
		// 是否关闭 或者 没有设置超时时间
		if atomic.LoadUint32(&l.closed) == 1 || l.conf.idleTimeout <= 0 {
			l.mu.Unlock()
			return
		}
		// 循环链表
		for i, n := 0, l.idles.Len(); i < n; i++ {
			// idles.Back() 返回链表中最后一个元素, 如果当前链表已经是空了 则返回nil
			e := l.idles.Back()
			if e == nil {
				break
			}
			// 断言为 item
			ic := e.Value.(item)
			// 判断时间是否超时
			if !ic.expire(l.conf.idleTimeout) {
				break
			}
			// 如果已经超时,则删除此元素
			l.idles.Remove(e)
			// release 计数
			l.release()
			l.mu.Unlock()
			_ = ic.s.Shutdown()
			l.mu.Lock()
		}
		l.mu.Unlock()
	}
}

// release 当前活跃线程数-1 并发送信号通知
// hold p.mu during the call.
func (l *List) release() {
	// l.active -= 1
	atomic.AddUint64(&l.active, ^uint64(0))
	l.signal()
}

// signal 发送信号通知
func (l *List) signal() {
	select {
	case l.cond <- struct{}{}:
	default:
	}
}

// Get 获取
func (l *List) Get(ctx context.Context) (IShutdown, error) {
	l.mu.Lock()
	// 判断是否关闭
	if atomic.LoadUint32(&l.closed) == 1 {
		l.mu.Unlock()
		return nil, ErrPoolClosed
	}
	for {
		for i, n := 0, l.idles.Len(); i < n; i++ {
			e := l.idles.Front()
			if e == nil {
				break
			}
			ic := e.Value.(item)
			l.idles.Remove(e)
			l.mu.Unlock()
			// 没有过期的可以直接返回了
			if !ic.expire(l.conf.idleTimeout) {
				return ic.s, nil
			}
			// 清理 重新获取锁
			_ = ic.s.Shutdown()
			l.mu.Lock()
			l.release()
		}

		// 检查是否关闭
		if atomic.LoadUint32(&l.closed) == 1 {
			l.mu.Unlock()
			return nil, ErrPoolClosed
		}
		// 判断是否需要新增
		if l.conf.active == 0 || l.active < l.conf.active {
			if l.f == nil {
				return nil, ErrPoolNewFuncIsNull
			}
			newItem := l.f
			l.mu.Unlock()
			atomic.AddUint64(&l.active, 1)
			// 新增:
			c, err := newItem(ctx)
			if err != nil {
				l.release()
				c = nil
			}
			return c, err
		}
		// 如果满了判断是否需要等待
		if l.conf.waitTimeout == 0 && !l.conf.wait {
			l.mu.Unlock()
			return nil, ErrPoolExhausted
		}
		// 获取超时时间,解锁进入等待状态
		wt := l.conf.waitTimeout
		l.mu.Unlock()

		// 控制链路超时时间
		_, nCtx, cancel := timeout.Shrink(ctx, wt)

		// 超时/收到了某应用回收的信号
		select {
		case <-nCtx.Done():
			cancel()
			return nil, nCtx.Err()
		case <-l.cond:
		}
		// 自旋,再次尝试获得句柄
		cancel()
		l.mu.Lock()
	}
}

// Put 回收
func (l *List) Put(ctx context.Context, s IShutdown, forceClose bool) error {
	l.mu.Lock()
	if atomic.LoadUint32(&l.closed) == 0 && !forceClose {
		// 插入到链表头
		l.idles.PushFront(item{createdAt: nowFunc(), s: s})
		// 判断闲置数量是否达到阈值
		if uint64(l.idles.Len()) > l.conf.idle {
			// 拿到尾部淘汰的 shutdown
			s = l.idles.Remove(l.idles.Back()).(item).s
		} else {
			s = nil
		}
	}
	// 如果 s == nil 进入回收
	if s == nil {
		l.signal()
		l.mu.Unlock()
		return nil
	}
	l.mu.Unlock()
	l.release()
	return s.Shutdown()
}

// Shutdown 关闭
func (l *List) Shutdown() error {
	l.mu.Lock()
	if atomic.SwapUint32(&l.closed, 1) == 1 {
		return ErrPoolClosed
	}
	idles := l.idles
	// .Init 重新初始化链表 快速清空
	l.idles.Init()
	if idles.Len() > 0 {
		atomic.AddUint64(&l.active, ^uint64(idles.Len()-1))
	}
	l.mu.Unlock()
	// 在循环旧链表进行资源回收
	for e := idles.Front(); e != nil; e = e.Next() {
		_ = e.Value.(item).s.Shutdown()
	}
	return nil
}

// New 设置创建资源函数
func (l *List) New(f func(ctx context.Context) (IShutdown, error)) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.f = f
}

// NewList 实例化
func NewList(options ...options.Option) Pool {
	l := &List{conf: defaultConfig()}
	l.cond = make(chan struct{})
	for _, option := range options {
		option(l.conf)
	}
	l.Init(l.conf.idleTimeout)
	return l
}
