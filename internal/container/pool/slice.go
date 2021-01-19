package pool

import (
	"Songzhibin/GKit/timeout"
	"context"
	"sync"
	"sync/atomic"
	"time"
)

// Slice:
type Slice struct {
	// f: item
	f func(ctx context.Context) (IShutdown, error)

	// cancel: 关闭链路
	cancel context.CancelFunc

	// mu: 互斥锁, 保护以下字段
	mu sync.Mutex

	// itemRequests:
	itemRequests map[uint64]chan item

	// nextIndex: itemRequests 使用的下一个 key
	nextIndex uint64

	// active: 待处理的任务数
	active uint64

	// openerCh:
	openerCh chan struct{}

	// cleanerCh: 清空 ch
	cleanerCh chan struct{}

	// conf: 配置信息
	conf *Config

	// closed:
	closed uint32

	// freeItems: 空闲的 items 对列
	freeItems []*item
}

// Reload: 重新设置配置文件
func (s *Slice) Reload(c *Config) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.conf = c
	// todo 未做完
}

func (s *Slice) Init(d time.Duration) {
	// 如果 <= 0 放弃设置
	if d <= 0 {
		return
	}
	// 如果时间间隔d小于等待超时,并且 cleanerCh 不为nil 监听信号
	if d < s.conf.IdleTimeout && s.cleanerCh != nil {
		select {
		case s.cleanerCh <- struct{}{}:
		default:
		}
	}
	// 懒加载
	if s.cleanerCh == nil {
		s.cleanerCh = make(chan struct{}, 1)
	}
	// 开启定时任务
	go s.Timer(s.conf.IdleTimeout)
}

// Timer: 定时任务
func (s *Slice) Timer(d time.Duration) {
	if d < minDuration {
		d = minDuration
	}
	// ticker: 定时任务
	ticker := time.NewTicker(d)
	for {
		// 触发条件:
		// 1. 定时周期
		// 2. l.cleanerCh 接收到信号
		select {
		case <-ticker.C:
		case <-s.cleanerCh:
		}
		s.mu.Lock()
		//  是否关闭 或者 没有设置超时时间
		if atomic.LoadUint32(&s.closed) == 1 || s.conf.IdleTimeout <= 0 {
			s.mu.Unlock()
			return
		}
		// recycled: 待回收的item
		var recycled []*item
		for i := 0; i < len(s.freeItems); i++ {
			c := s.freeItems[i]
			// 判断时间是否超时
			if !c.expire(d) {
				recycled = append(recycled, c)
				atomic.AddUint64(&s.active, ^uint64(0))
				// last: 数组尾节点
				last := len(s.freeItems) - 1
				// 置换
				s.freeItems[i] = s.freeItems[last]
				s.freeItems[last] = nil
				s.freeItems = s.freeItems[:last]
				i--
			}
		}
		s.mu.Unlock()
		// 释放
		for _, i := range recycled {
			_ = i.shutdown()
		}
		// todo
	}
}

// release: 当前活跃线程数-1 并发送信号通知
// hold p.mu during the call.
func (s *Slice) release() {
	// l.active -= 1
	atomic.AddUint64(&s.active, ^uint64(0))
	s.signal()
}

// getIndex: 获取index位置 指针指向下一位
func (s *Slice) getIndex() uint64 {
	n := s.nextIndex
	atomic.AddUint64(&s.nextIndex, 1)
	return n
}

// addItem: 添加item到维护对列
func (s *Slice) addItem(i IShutdown) bool {
	// 判断是否关闭
	if atomic.LoadUint32(&s.closed) == 1 {
		return false
	}
	// 判断是否还能够存储
	if s.conf.Active > 0 && s.active > s.conf.Active {
		return false
	}
	ic := item{
		createdAt: nowFunc(),
		s:         i,
	}
	if len(s.itemRequests) > 0 {
		// 随机在map中找到一组request 和 key
		var (
			key uint64
			req chan item
		)
		for key, req = range s.itemRequests {
			break
		}
		if req == nil {
			// 表示没有等待的对列
			s.freeItems = append(s.freeItems, &ic)
			return true
		}
		delete(s.itemRequests, key)
		req <- ic
		return true
	} else if atomic.LoadUint32(&s.closed) == 0 && s.maxIdleItems() > uint64(len(s.freeItems)) {
		s.freeItems = append(s.freeItems, &ic)
		return true
	}
	return true
}

// maxIdleItems: 最大等待数
func (s *Slice) maxIdleItems() uint64 {
	n := s.conf.Idle
	switch {
	case n == 0:
		return defaultIdleItems
	case n < 0:
		return 0
	default:
		return n
	}
}

func (s *Slice) signal() {
	r := (uint64)(len(s.itemRequests))
	if s.conf.Active > 0 {
		numCanOpen := s.conf.Active - s.active
		if r > numCanOpen {
			r = numCanOpen
		}
	}
	if r > 0 {
		atomic.AddUint64(&s.active, 1)
		r--
		if atomic.LoadUint32(&s.closed) == 1 {
			return
		}
		s.openerCh <- struct{}{}
	}
}

// Get: 获取
func (s *Slice) Get(ctx context.Context) (IShutdown, error) {
	s.mu.Lock()
	// 判断是否关闭
	if atomic.LoadUint32(&s.closed) == 1 {
		s.mu.Unlock()
		return nil, ErrPoolClosed
	}
	for {
		itemLen := len(s.freeItems)
		for itemLen > 0 {
			// Front: 首部
			ic := s.freeItems[0]
			copy(s.freeItems, s.freeItems[1:])
			s.freeItems = s.freeItems[:itemLen]
			s.mu.Unlock()
			// 没有过期的可以直接返回了
			if !ic.expire(s.conf.IdleTimeout) {
				return ic.s, nil
			}
			// 清理 重新获取锁
			_ = ic.s.Shutdown()
			s.mu.Lock()
			s.release()
			itemLen = len(s.freeItems)
		}
		// 再次检查是否关闭
		if atomic.LoadUint32(&s.closed) == 1 {
			s.mu.Unlock()
			return nil, ErrPoolClosed
		}
		// 判断是否还可以申请,否则等待
		if s.conf.Active == 0 || s.active < s.conf.Active {
			newItem := s.f
			s.mu.Unlock()
			atomic.AddUint64(&s.active, 1)
			c, err := newItem(ctx)
			if err != nil {
				s.release()
				c = nil
			}
			return c, err
		}
		// 如果满了判断是否需要等待
		if s.conf.WaitTimeout == 0 && !s.conf.Wait {
			s.mu.Unlock()
			return nil, ErrPoolExhausted
		}
		// 初始化channel
		req := make(chan item, 1)
		key := s.nextIndex
		s.itemRequests[key] = req
		// 获取超时时间,解锁进入等待状态
		wt := s.conf.WaitTimeout
		s.mu.Unlock()
		// 控制链路超时时间
		_, nCtx, cancel := timeout.Shrink(ctx, wt)
		select {
		case <-ctx.Done():
			cancel()
			return nil, nCtx.Err()
		case v, ok := <-req:
			if !ok {
				return nil, ErrPoolClosed
			}
			if v.expire(s.conf.IdleTimeout) {
				_ = v.shutdown()
				break
			}
			return v.s, nil
		}
		// 自旋,再次尝试获得句柄
		cancel()
		s.mu.Lock()
	}
}

// Put: 回收
func (s *Slice) Put(ctx context.Context, i IShutdown, forceClose bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if forceClose {
		s.release()
		return i.Shutdown()
	}
	if !s.addItem(i) {
		atomic.AddUint64(&s.active, ^uint64(0))
		return i.Shutdown()
	}
	return nil
}

// Shutdown: 回收资源
func (s *Slice) Shutdown() error {
	s.mu.Lock()
	// 关闭
	if atomic.SwapUint32(&s.closed, 1) == 1 {
		s.mu.Unlock()
		return ErrPoolClosed
	}
	if s.cleanerCh != nil {
		close(s.cleanerCh)
	}
	recycled := s.freeItems
	s.freeItems = nil
	for _, req := range s.itemRequests {
		close(req)
	}
	s.mu.Unlock()
	s.cancel()
	// 释放所有节点
	for _, v := range recycled {
		_ = v.shutdown()
	}
	return nil
}
