package clock

import (
	"sync"
	"sync/atomic"
	"time"
)

const (
	TimeFormat         = "2006-01-02 15:04:05"
	DateFormat         = "2006-01-02"
	UnixTimeUnitOffset = uint64(time.Millisecond / time.Nanosecond)
)

var (
	_ Clock = &RealClock{}
	_ Clock = &MockClock{}

	_ Ticker = &RealTicker{}
	_ Ticker = &MockTicker{}

	_ TickerCreator = &RealTickerCreator{}
	_ TickerCreator = &MockTickerCreator{}
)

var (
	currentClock         *atomic.Value
	currentTickerCreator *atomic.Value
)

func init() {
	realClock := NewRealClock()
	currentClock = new(atomic.Value)
	SetClock(realClock)

	realTickerCreator := NewRealTickerCreator()
	currentTickerCreator = new(atomic.Value)
	SetTickerCreator(realTickerCreator)
}

// Clock 时钟接口
type Clock interface {
	Now() time.Time
	Sleep(d time.Duration)
	GetTimeMillis() uint64
	GetTimeNano() uint64
}

// clockWrapper is used for atomic operation.
type clockWrapper struct {
	clock Clock
}

// RealClock 真实使用的Clock对象
type RealClock struct{}

func NewRealClock() *RealClock {
	return &RealClock{}
}

func (t *RealClock) Now() time.Time {
	return time.Now()
}

func (t *RealClock) Sleep(d time.Duration) {
	time.Sleep(d)
}

func (t *RealClock) GetTimeMillis() uint64 {
	tickerNow := GetTimestamp()
	if tickerNow > uint64(0) {
		return tickerNow
	}
	return uint64(time.Now().UnixNano()) / UnixTimeUnitOffset
}

func (t *RealClock) GetTimeNano() uint64 {
	return uint64(t.Now().UnixNano())
}

// MockClock 测试使用的Clock对象
type MockClock struct {
	lock sync.RWMutex
	now  time.Time
}

func NewMockClock() *MockClock {
	return &MockClock{
		now: time.Now(),
	}
}

func (t *MockClock) Now() time.Time {
	t.lock.RLock()
	defer t.lock.RUnlock()
	return t.now
}

func (t *MockClock) Sleep(d time.Duration) {
	if d <= 0 {
		return
	}
	t.lock.Lock()
	t.now = t.now.Add(d)
	t.lock.Unlock()
	time.Sleep(time.Millisecond)
}

func (t *MockClock) GetTimeMillis() uint64 {
	return uint64(t.Now().UnixNano()) / UnixTimeUnitOffset
}

func (t *MockClock) GetTimeNano() uint64 {
	return uint64(t.Now().UnixNano())
}

// Ticker time.Ticker 对象封装
type Ticker interface {
	C() <-chan time.Time
	Stop()
}

// RealTicker 真实使用的 Ticker 对象
type RealTicker struct {
	t *time.Ticker
}

func NewRealTicker(d time.Duration) *RealTicker {
	return &RealTicker{
		t: time.NewTicker(d),
	}
}

func (t *RealTicker) C() <-chan time.Time {
	return t.t.C
}

func (t *RealTicker) Stop() {
	t.t.Stop()
}

// MockTicker 测试使用的 Ticker 对象
// MockTicker 和 MockClock 一般搭配使用
type MockTicker struct {
	lock   sync.Mutex
	period time.Duration
	c      chan time.Time
	last   time.Time
	stop   chan struct{}
}

func NewMockTicker(d time.Duration) *MockTicker {
	t := &MockTicker{
		period: d,
		c:      make(chan time.Time, 1),
		last:   Now(),
		stop:   make(chan struct{}),
	}

	go t.checkLoop()

	return t
}

func (t *MockTicker) C() <-chan time.Time {
	return t.c
}

func (t *MockTicker) Stop() {
	close(t.stop)
}

func (t *MockTicker) check() {
	t.lock.Lock()
	defer t.lock.Unlock()

	now := Now()
	for next := t.last.Add(t.period); !next.After(now); next = next.Add(t.period) {
		t.last = next
		select {
		case <-t.stop:
			return
		case t.c <- t.last:
		default:
		}
	}
}

func (t *MockTicker) checkLoop() {
	ticker := time.NewTicker(time.Microsecond)
	for {
		select {
		case <-t.stop:
			return
		case <-ticker.C:
		}
		t.check()
	}
}

// TickerCreator 实例化Ticker.
type TickerCreator interface {
	NewTicker(d time.Duration) Ticker
}

// tickerCreatorWrapper 封装 atomic 操作
type tickerCreatorWrapper struct {
	tickerCreator TickerCreator
}

// RealTickerCreator 创建真实的 RealTicker 和 time.Ticker 对象.
type RealTickerCreator struct{}

func NewRealTickerCreator() *RealTickerCreator {
	return &RealTickerCreator{}
}

func (tc *RealTickerCreator) NewTicker(d time.Duration) Ticker {
	return NewRealTicker(d)
}

// MockTickerCreator 创建 MockTicker 用于测试
// MockTickerCreator 和 MockClock 通常一起使用
type MockTickerCreator struct{}

func NewMockTickerCreator() *MockTickerCreator {
	return &MockTickerCreator{}
}

func (tc *MockTickerCreator) NewTicker(d time.Duration) Ticker {
	return NewMockTicker(d)
}

// SetClock 设置 Clock
func SetClock(c Clock) {
	currentClock.Store(&clockWrapper{c})
}

// CurrentClock 返回 Clock 对象
func CurrentClock() Clock {
	return currentClock.Load().(*clockWrapper).clock
}

// SetTickerCreator 设置 Ticker 对象.
func SetTickerCreator(tc TickerCreator) {
	currentTickerCreator.Store(&tickerCreatorWrapper{tc})
}

// CurrentTickerCreator 获取 Ticker 对象
func CurrentTickerCreator() TickerCreator {
	return currentTickerCreator.Load().(*tickerCreatorWrapper).tickerCreator
}

func NewTicker(d time.Duration) Ticker {
	return CurrentTickerCreator().NewTicker(d)
}

// FormatTimeMillis 将Unix时间戳(ms)格式化为时间字符串
func FormatTimeMillis(tsMillis uint64) string {
	return time.Unix(0, int64(tsMillis*UnixTimeUnitOffset)).Format(TimeFormat)
}

// FormatDate 将Unix时间戳(ms)格式化为日期字符串
func FormatDate(tsMillis uint64) string {
	return time.Unix(0, int64(tsMillis*UnixTimeUnitOffset)).Format(DateFormat)
}

// GetTimeMillis 返回当前的Unix时间戳(ms)
func GetTimeMillis() uint64 {
	return CurrentClock().GetTimeMillis()
}

// GetTimeNano 返回当前的Unix时间戳(ns)
func GetTimeNano() uint64 {
	return CurrentClock().GetTimeNano()
}

// Now 返回当前本地时间。
func Now() time.Time {
	return CurrentClock().Now()
}

func Sleep(d time.Duration) {
	CurrentClock().Sleep(d)
}
