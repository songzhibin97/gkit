package watching

import (
	"os"
	"sync"
	"time"
)

type configs struct {
	UseGoProcAsCPUCore bool // use the go max procs number as the CPU core number when it's true
	UseCGroup          bool // use the CGroup to calc cpu/memory when it's true

	// overwrite the system level memory limitation when > 0.
	memoryLimit uint64
	cpuCore     float64

	*ShrinkThrConfigs

	*DumpConfigs

	LogLevel int
	Logger   *os.File

	// interval for dump loop, default 5s
	CollectInterval   time.Duration
	intervalResetting chan struct{}

	// the cooldown time after every type of dump
	// interval for cooldown，default 1m
	// the cpu/mem/goroutine have different cooldowns of their own
	CoolDown time.Duration

	// if current cpu usage percent is greater than CPUMaxPercent,
	// Watching would not dump all types profile, cuz this
	// move may result of the system crash.
	CPUMaxPercent int

	// if write lock is held mean Watching's
	// configuration is being modified.
	L *sync.RWMutex

	logConfigs   *logConfigs
	GroupConfigs *groupConfigs

	MemConfigs    *typeConfig
	GCHeapConfigs *typeConfig
	CpuConfigs    *typeConfig
	ThreadConfigs *typeConfig

	// profile reporter
	rptConfigs *ReporterConfigs
}

// DumpConfigs contains configuration about dump file.
type DumpConfigs struct {
	// full path to put the profile files, default /tmp
	DumpPath string
	// default dump to binary profile, set to true if you want a text profile
	DumpProfileType dumpProfileType
	// only dump top 10 if set to false, otherwise dump all, only effective when in_text = true
	DumpFullStack bool
}

// ShrinkThrConfigs contains the configuration about shrink thread
type ShrinkThrConfigs struct {
	// shrink the thread number when it exceeds the max threshold that specified in Threshold
	Enable    bool
	Threshold int
	Delay     time.Duration // start to shrink thread after the delay time.
}

type logConfigs struct {
	RotateEnable    bool
	SplitLoggerSize int64 // SplitLoggerSize The size of the log split
}

type typeConfig struct {
	Enable bool
	// mem/cpu/gcheap trigger minimum in percent, goroutine/thread trigger minimum in number
	TriggerMin int

	// mem/cpu/gcheap trigger abs in percent, goroutine/thread trigger abs in number
	TriggerAbs int

	// mem/cpu/gcheap/goroutine/thread trigger diff in percent
	TriggerDiff int
}

type gcHeapConfigs struct {
	// enable the heap dumper, should dump if one of the following requirements is matched
	//   1. GC heap usage > GCHeapTriggerPercentMin && GC heap usage diff > GCHeapTriggerPercentDiff
	//   2. GC heap usage > GCHeapTriggerPercentAbs
	Enable                   bool
	GCHeapTriggerPercentMin  int // GC heap trigger minimum in percent
	GCHeapTriggerPercentDiff int // GC heap trigger diff in percent
	GCHeapTriggerPercentAbs  int // GC heap trigger absolute in percent
}

type groupConfigs struct {
	// enable the goroutine dumper, should dump if one of the following requirements is matched
	//   1. goroutine_num > GoroutineTriggerNumMin && goroutine_num < GoroutineTriggerNumMax && goroutine diff percent > GoroutineTriggerPercentDiff
	//   2. goroutine_num > GoroutineTriggerNumAbsNum && goroutine_num < GoroutineTriggerNumMax
	*typeConfig
	GoroutineTriggerNumMax int // goroutine trigger max in number
}

type memConfigs struct {
	// enable the heap dumper, should dump if one of the following requirements is matched
	//   1. memory usage > MemTriggerPercentMin && memory usage diff > MemTriggerPercentDiff
	//   2. memory usage > MemTriggerPercentAbs
	Enable                bool
	MemTriggerPercentMin  int // mem trigger minimum in percent
	MemTriggerPercentDiff int // mem trigger diff in percent
	MemTriggerPercentAbs  int // mem trigger absolute in percent
}

type cpuConfigs struct {
	// enable the cpu dumper, should dump if one of the following requirements is matched
	//   1. cpu usage > CPUTriggerMin && cpu usage diff > CPUTriggerDiff
	//   2. cpu usage > CPUTriggerAbs
	Enable                bool
	CPUTriggerPercentMin  int // cpu trigger min in percent
	CPUTriggerPercentDiff int // cpu trigger diff in percent
	CPUTriggerPercentAbs  int // cpu trigger abs in percent
}

type threadConfigs struct {
	Enable                   bool
	ThreadTriggerPercentMin  int // thread trigger min in number
	ThreadTriggerPercentDiff int // thread trigger diff in percent
	ThreadTriggerPercentAbs  int // thread trigger abs in number
}

type ReporterConfigs struct {
	reporter ProfileReporter
	active   int32 // switch
}

// defaultReporterConfigs returns  ReporterConfigs。
func defaultReporterConfigs() *ReporterConfigs {
	opts := &ReporterConfigs{}

	return opts
}

func defaultLogConfigs() *logConfigs {
	return &logConfigs{
		RotateEnable:    true,
		SplitLoggerSize: defaultShardLoggerSize,
	}
}

func defaultGroupConfigs() *groupConfigs {
	return &groupConfigs{
		typeConfig: &typeConfig{
			Enable:      false,
			TriggerMin:  defaultGoroutineTriggerAbs,
			TriggerAbs:  defaultGoroutineTriggerDiff,
			TriggerDiff: defaultGoroutineTriggerMin,
		},
		GoroutineTriggerNumMax: 0,
	}
}

func defaultGCHeapOptions() *typeConfig {
	return &typeConfig{
		Enable:      false,
		TriggerMin:  defaultGCHeapTriggerAbs,
		TriggerAbs:  defaultGCHeapTriggerDiff,
		TriggerDiff: defaultGCHeapTriggerMin,
	}
}

func defaultMemConfigs() *typeConfig {
	return &typeConfig{
		Enable:      false,
		TriggerMin:  defaultMemTriggerAbs,
		TriggerAbs:  defaultMemTriggerDiff,
		TriggerDiff: defaultMemTriggerMin,
	}
}

func defaultCPUConfigs() *typeConfig {
	return &typeConfig{
		Enable:      false,
		TriggerMin:  defaultCPUTriggerAbs,
		TriggerAbs:  defaultCPUTriggerDiff,
		TriggerDiff: defaultCPUTriggerMin,
	}
}

func defaultThreadConfig() *typeConfig {
	return &typeConfig{
		Enable:      false,
		TriggerMin:  defaultThreadTriggerAbs,
		TriggerAbs:  defaultThreadTriggerDiff,
		TriggerDiff: defaultThreadTriggerMin,
	}
}

func defaultConfig() *configs {
	return &configs{
		logConfigs:        defaultLogConfigs(),
		GroupConfigs:      defaultGroupConfigs(),
		MemConfigs:        defaultMemConfigs(),
		GCHeapConfigs:     defaultGCHeapOptions(),
		CpuConfigs:        defaultCPUConfigs(),
		ThreadConfigs:     defaultThreadConfig(),
		LogLevel:          LogLevelDebug,
		Logger:            os.Stdout,
		CollectInterval:   defaultInterval,
		intervalResetting: make(chan struct{}, 1),
		CoolDown:          defaultCooldown,
		DumpConfigs: &DumpConfigs{
			DumpPath:        defaultDumpPath,
			DumpProfileType: defaultDumpProfileType,
			DumpFullStack:   false,
		},
		ShrinkThrConfigs: &ShrinkThrConfigs{
			Enable: false,
		},
		L:          &sync.RWMutex{},
		rptConfigs: defaultReporterConfigs(),
	}
}

// GetShrinkThreadConfigs return a copy of ShrinkThrConfigs.
func (c *configs) GetShrinkThreadConfigs() ShrinkThrConfigs {
	c.L.RLock()
	defer c.L.RUnlock()
	return *c.ShrinkThrConfigs
}

// GetMemConfigs return a copy of memConfigs.
func (c *configs) GetMemConfigs() typeConfig {
	c.L.RLock()
	defer c.L.RUnlock()
	return *c.MemConfigs
}

// GetCPUConfigs return a copy of cpuConfigs
func (c *configs) GetCPUConfigs() typeConfig {
	c.L.RLock()
	defer c.L.RUnlock()
	return *c.CpuConfigs
}

// GetGroupConfigs return a copy of grOptions
func (c *configs) GetGroupConfigs() groupConfigs {
	c.L.RLock()
	defer c.L.RUnlock()
	return *c.GroupConfigs
}

// GetThreadConfigs return a copy of threadConfigs
func (c *configs) GetThreadConfigs() typeConfig {
	c.L.RLock()
	defer c.L.RUnlock()
	return *c.ThreadConfigs
}

// GetGcHeapConfigs return a copy of gcHeapConfigs
func (c *configs) GetGcHeapConfigs() typeConfig {
	c.L.RLock()
	defer c.L.RUnlock()
	return *c.GCHeapConfigs
}

// GetReporterConfigs returns a copy of ReporterConfigs.
func (c *configs) GetReporterConfigs() ReporterConfigs {
	c.L.RLock()
	defer c.L.RUnlock()
	return *c.rptConfigs
}
