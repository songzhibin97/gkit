package bbr

import (
	"Songzhibin/GKit/internal/container/group"
	"Songzhibin/GKit/internal/window"
	"sync/atomic"
	"time"
)

// package bbr: bbr 限流

var (
	cpu         int64
	decay       = 0.95
	initTime    = time.Now()
	defaultConf = &Config{
		Window:       time.Second * 10,
		WinBucket:    100,
		CPUThreshold: 800,
	}
)

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

// Stats: bbr 指标信息
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
	passStat        window.Window
	rtStat          window.Window
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
	group *group.Group
}
