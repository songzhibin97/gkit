package bbr

import (
	"context"
	"math"
	"sync/atomic"
	"time"

	"github.com/songzhibin97/gkit/container/group"
	"github.com/songzhibin97/gkit/internal/stat"
	cupstat "github.com/songzhibin97/gkit/internal/sys/cpu"
	"github.com/songzhibin97/gkit/log"
	"github.com/songzhibin97/gkit/options"
	"github.com/songzhibin97/gkit/overload"
)

// package bbr: bbr 限流

var (
	cpu int64

	// decay: 衰变周期
	decay = 0.95

	// initTime: 起始时间
	initTime = time.Now()
)

// cpuGetter:
type cpuGetter func() int64

// config: bbr 配置
type config struct {
	debug        bool
	enabled      bool
	winBucket    int
	cPUThreshold int64
	window       time.Duration
	rule         string
}

// Stat bbr 指标信息
type Stat struct {
	Cpu         int64
	InFlight    int64
	MaxInFlight int64
	MinRt       int64
	MaxPass     int64
}

// BBR 实现类似bbr的限制器.
type BBR struct {
	cpu             cpuGetter
	passStat        stat.RollingCounter
	rtStat          stat.RollingCounter
	inFlight        int64
	winBucketPerSec int64
	conf            *config
	prevDrop        atomic.Value
	prevDropHit     int32
	rawMaxPASS      int64
	rawMinRt        int64
}

// Group 表示BBRLimiter的类，并形成其中的命名空间
type Group struct {
	group group.LazyLoadGroup
}

// init 启动后台收集cpu信息
func init() {
	go cpuProc()
}

// cpuProc  定时任务,收集当前服务器CPU信息
// cpu = cpuᵗ⁻¹ * decay + cpuᵗ * (1 - decay)
func cpuProc() {
	ticker := time.NewTicker(time.Millisecond * 250)
	defer func() {
		ticker.Stop()
		if err := recover(); err != nil {
			log.NewHelper(log.DefaultLogger).Errorf("rate.limit.cpuproc() err(%+v)", err)
			go cpuProc()
		}
	}()

	for range ticker.C {
		stats := &cupstat.Stat{}
		cupstat.ReadStat(stats)
		stats.Usage = min(stats.Usage, 1000)
		prevCpu := atomic.LoadInt64(&cpu)
		curCpu := int64(float64(prevCpu)*decay + float64(stats.Usage)*(1.0-decay))
		atomic.StoreInt64(&cpu, curCpu)
	}
}

func min(l, r uint64) uint64 {
	if l < r {
		return l
	}
	return r
}

// maxPASS 最大通过值
func (l *BBR) maxPASS() int64 {
	rawMaxPass := atomic.LoadInt64(&l.rawMaxPASS)
	if rawMaxPass > 0 && l.passStat.Timespan() < 1 {
		return rawMaxPass
	}
	rawMaxPass = int64(l.passStat.Reduce(func(iterator stat.Iterator) float64 {
		result := 1.0
		for i := 1; iterator.Next() && i < l.conf.winBucket; i++ {
			bucket := iterator.Bucket()
			count := 0.0
			for _, p := range bucket.Points {
				count += p
			}
			result = math.Max(result, count)
		}
		return result
	}))
	if rawMaxPass == 0 {
		rawMaxPass = 1
	}
	atomic.StoreInt64(&l.rawMaxPASS, rawMaxPass)
	return rawMaxPass
}

// minRT 最小RT
func (l *BBR) minRT() int64 {
	rawMinRT := atomic.LoadInt64(&l.rawMinRt)
	if rawMinRT > 0 && l.rtStat.Timespan() < 1 {
		return rawMinRT
	}
	rawMinRT = int64(math.Ceil(l.rtStat.Reduce(func(iterator stat.Iterator) float64 {
		result := math.MaxFloat64
		for i := 1; iterator.Next() && i < l.conf.winBucket; i++ {
			bucket := iterator.Bucket()
			if len(bucket.Points) == 0 {
				continue
			}
			total := 0.0
			for _, p := range bucket.Points {
				total += p
			}
			avg := total / float64(bucket.Count)
			result = math.Min(result, avg)
		}
		return result
	})))
	if rawMinRT <= 0 {
		rawMinRT = 1
	}
	atomic.StoreInt64(&l.rawMinRt, rawMinRT)
	return rawMinRT
}

// maxFlight
func (l *BBR) maxFlight() int64 {
	return int64(math.Floor(float64(l.maxPASS()*l.minRT()*l.winBucketPerSec)/1000.0 + 0.5))
}

// shouldDrop 判断是否应该降低
func (l *BBR) shouldDrop() bool {
	if l.cpu() < l.conf.cPUThreshold {
		prevDrop, _ := l.prevDrop.Load().(time.Duration)
		if prevDrop == 0 {
			return false
		}
		if time.Since(initTime)-prevDrop <= time.Second {
			if atomic.LoadInt32(&l.prevDropHit) == 0 {
				atomic.StoreInt32(&l.prevDropHit, 1)
			}
			inFlight := atomic.LoadInt64(&l.inFlight)
			return inFlight > 1 && inFlight > l.maxFlight()
		}
		l.prevDrop.Store(time.Duration(0))
		return false
	}
	inFlight := atomic.LoadInt64(&l.inFlight)
	drop := inFlight > 1 && inFlight > l.maxFlight()
	if drop {
		prevDrop, _ := l.prevDrop.Load().(time.Duration)
		if prevDrop != 0 {
			return drop
		}
		l.prevDrop.Store(time.Since(initTime))
	}
	return drop
}

// Stat 状态信息
func (l *BBR) Stat() Stat {
	return Stat{
		Cpu:         l.cpu(),
		InFlight:    atomic.LoadInt64(&l.inFlight),
		MinRt:       l.minRT(),
		MaxPass:     l.maxPASS(),
		MaxInFlight: l.maxFlight(),
	}
}

// Allow 检查所有入站流量
// 一旦检测到过载，它将引发 LimitExceed 错误。
func (l *BBR) Allow(ctx context.Context, opts ...overload.AllowOption) (func(info overload.DoneInfo), error) {
	allowOpts := overload.DefaultAllowOpts()
	for _, opt := range opts {
		opt.Apply(&allowOpts)
	}
	if l.shouldDrop() {
		return nil, LimitExceed
	}
	atomic.AddInt64(&l.inFlight, 1)
	sTime := time.Since(initTime)
	return func(do overload.DoneInfo) {
		rt := int64((time.Since(initTime) - sTime) / time.Millisecond)
		l.rtStat.Add(rt)
		atomic.AddInt64(&l.inFlight, -1)
		switch do.Op {
		case overload.Success:
			l.passStat.Add(1)
			return
		default:
			return
		}
	}, nil
}

// defaultConf 默认配置
func defaultConf() *config {
	return &config{
		// window: 窗口周期
		window:    time.Second * 10,
		winBucket: 100,
		// cPUThreshold: 阈值
		cPUThreshold: 800,
	}
}

// Option

func SetDebug(debug bool) options.Option {
	return func(c interface{}) {
		c.(*config).debug = debug
	}
}

func SetEnabled(enabled bool) options.Option {
	return func(c interface{}) {
		c.(*config).enabled = enabled
	}
}

func SetWinBucket(winBucket int) options.Option {
	return func(c interface{}) {
		c.(*config).winBucket = winBucket
	}
}

func SetCPUThreshold(cPUThreshold int64) options.Option {
	return func(c interface{}) {
		c.(*config).cPUThreshold = cPUThreshold
	}
}

func SetWindow(window time.Duration) options.Option {
	return func(c interface{}) {
		c.(*config).window = window
	}
}

func SetRule(rule string) options.Option {
	return func(c interface{}) {
		c.(*config).rule = rule
	}
}

// newLimiter 实例化限制器
func newLimiter(options ...options.Option) overload.Limiter {
	// 判断传入配置是否为空,否则使用默认配置
	conf := defaultConf()
	for _, opt := range options {
		opt(conf)
	}

	size := conf.winBucket
	bucketDuration := conf.window / time.Duration(conf.winBucket)
	passStat := stat.NewRollingCounter(size, bucketDuration)
	rtStat := stat.NewRollingCounter(size, bucketDuration)
	cpu := func() int64 {
		return atomic.LoadInt64(&cpu)
	}
	limiter := &BBR{
		cpu:             cpu,
		conf:            conf,
		passStat:        passStat,
		rtStat:          rtStat,
		winBucketPerSec: int64(time.Second) / (int64(conf.window) / int64(conf.winBucket)),
	}
	return limiter
}

// NewGroup 实例化限制器容器
func NewGroup(options ...options.Option) *Group {
	// 判断传入配置是否为空,否则使用默认配置
	_group := group.NewGroup(func() interface{} {
		return newLimiter(options...)
	})
	return &Group{
		group: _group,
	}
}

// Get 通过指定的键获取一个限制器，如果不存在限制器，则重新创建一个限制器。
func (g *Group) Get(key string) overload.Limiter {
	limiter := g.group.Get(key)
	return limiter.(overload.Limiter)
}
