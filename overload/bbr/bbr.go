package bbr

import (
	"Songzhibin/GKit/internal/container/group"
	"Songzhibin/GKit/internal/stat"
	cupstat "Songzhibin/GKit/internal/sys/cpu"
	"Songzhibin/GKit/overload"
	"context"
	"math"
	"sync/atomic"
	"time"
)

// package bbr: bbr 限流

var (
	cpu int64

	// decay: 衰变周期
	decay = 0.95

	// initTime: 起始时间
	initTime = time.Now()

	// defaultConf: 默认配置
	defaultConf = &Config{
		// Window: 窗口周期
		Window:    time.Second * 10,
		WinBucket: 100,
		// CPUThreshold: 阈值
		CPUThreshold: 800,
	}
)

// cpuGetter:
type cpuGetter func() int64

// Config: bbr 配置
type Config struct {
	Enabled      bool
	Window       time.Duration
	WinBucket    int
	Rule         string
	Debug        bool
	CPUThreshold int64
}

// Stat: bbr 指标信息
type Stat struct {
	Cpu         int64
	InFlight    int64
	MaxInFlight int64
	MinRt       int64
	MaxPass     int64
}

// BBR: 实现类似bbr的限制器.
type BBR struct {
	cpu             cpuGetter
	passStat        stat.RollingCounter
	rtStat          stat.RollingCounter
	inFlight        int64
	winBucketPerSec int64
	conf            *Config
	prevDrop        atomic.Value
	prevDropHit     int32
	rawMaxPASS      int64
	rawMinRt        int64
}

// Group: 表示BBRLimiter的类，并形成其中的命名空间
type Group struct {
	group group.LazyLoadGroup
}

// init: 启动后台收集cpu信息
func init() {
	go cpuProc()
}

// cpuProc:  定时任务,收集当前服务器CPU信息
// cpu = cpuᵗ⁻¹ * decay + cpuᵗ * (1 - decay)
func cpuProc() {
	ticker := time.NewTicker(time.Millisecond * 250)
	defer func() {
		ticker.Stop()
		if err := recover(); err != nil {
			go cpuProc()
		}
	}()

	for range ticker.C {
		stats := &cupstat.Stat{}
		cupstat.ReadStat(stats)
		prevCpu := atomic.LoadInt64(&cpu)
		curCpu := int64(float64(prevCpu)*decay + float64(stats.Usage)*(1.0-decay))
		atomic.StoreInt64(&cpu, curCpu)
	}
}

// maxPASS: 最大通过值
func (l *BBR) maxPASS() int64 {
	rawMaxPass := atomic.LoadInt64(&l.rawMaxPASS)
	if rawMaxPass > 0 && l.passStat.Timespan() < 1 {
		return rawMaxPass
	}
	rawMaxPass = int64(l.passStat.Reduce(func(iterator stat.Iterator) float64 {
		var result = 1.0
		for i := 1; iterator.Next() && i < l.conf.WinBucket; i++ {
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

// minRT: 最小RT
func (l *BBR) minRT() int64 {
	rawMinRT := atomic.LoadInt64(&l.rawMinRt)
	if rawMinRT > 0 && l.rtStat.Timespan() < 1 {
		return rawMinRT
	}
	rawMinRT = int64(math.Ceil(l.rtStat.Reduce(func(iterator stat.Iterator) float64 {
		var result = math.MaxFloat64
		for i := 1; iterator.Next() && i < l.conf.WinBucket; i++ {
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

// maxFlight:
func (l *BBR) maxFlight() int64 {
	return int64(math.Floor(float64(l.maxPASS()*l.minRT()*l.winBucketPerSec)/1000.0 + 0.5))
}

// shouldDrop: 判断是否应该降低
func (l *BBR) shouldDrop() bool {
	if l.cpu() < l.conf.CPUThreshold {
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

// Stat: 状态信息
func (l *BBR) Stat() Stat {
	return Stat{
		Cpu:         l.cpu(),
		InFlight:    atomic.LoadInt64(&l.inFlight),
		MinRt:       l.minRT(),
		MaxPass:     l.maxPASS(),
		MaxInFlight: l.maxFlight(),
	}
}

// Allow: 检查所有入站流量
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

// newLimiter: 实例化限制器
func newLimiter(conf *Config) overload.Limiter {
	// 判断传入配置是否为空,否则使用默认配置
	if conf == nil {
		conf = defaultConf
	}
	size := conf.WinBucket
	bucketDuration := conf.Window / time.Duration(conf.WinBucket)
	passStat := stat.NewRollingCounter(stat.RollingCounterOpts{Size: size, BucketDuration: bucketDuration})
	rtStat := stat.NewRollingCounter(stat.RollingCounterOpts{Size: size, BucketDuration: bucketDuration})
	cpu := func() int64 {
		return atomic.LoadInt64(&cpu)
	}
	limiter := &BBR{
		cpu:             cpu,
		conf:            conf,
		passStat:        passStat,
		rtStat:          rtStat,
		winBucketPerSec: int64(time.Second) / (int64(conf.Window) / int64(conf.WinBucket)),
	}
	return limiter
}

// NewGroup: 实例化限制器容器
func NewGroup(conf *Config) *Group {
	// 判断传入配置是否为空,否则使用默认配置
	if conf == nil {
		conf = defaultConf
	}
	_group := group.NewGroup(func() interface{} {
		return newLimiter(conf)
	})
	return &Group{
		group: _group,
	}
}

// Get: 通过指定的键获取一个限制器，如果不存在限制器，则重新创建一个限制器。
func (g *Group) Get(key string) overload.Limiter {
	limiter := g.group.Get(key)
	return limiter.(overload.Limiter)
}
