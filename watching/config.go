package watching

import (
	"os"
	"time"
)

type configs struct {
	// whether use the cgroup to calc memory or not
	UseCGroup bool

	// overwrite the system level memory limitation when > 0.
	memoryLimit uint64

	// full path to put the profile files, default /tmp
	DumpPath string
	// default dump to binary profile, set to true if you want a text profile
	DumpProfileType dumpProfileType
	// only dump top 10 if set to false, otherwise dump all, only effective when in_text = true
	DumpFullStack bool

	LogLevel int
	Logger   *os.File

	// interval for dump loop, default 5s
	CollectInterval time.Duration

	// the cooldown time after every type of dump
	// interval for cooldown，default 1m
	// the cpu/mem/goroutine have different cooldowns of their own
	CoolDown time.Duration

	// if current cpu usage percent is greater than CPUMaxPercent,
	// holmes would not dump all types profile, cuz this
	// move may result of the system crash.
	CPUMaxPercent int

	logConfigs    *logConfigs
	GroupConfigs  *groupConfigs
	MemConfigs    *memConfigs
	GCHeapConfigs *gcHeapConfigs
	CpuConfigs    *cpuConfigs
	ThreadConfigs *threadConfigs
}

type logConfigs struct {
	RotateEnable    bool
	SplitLoggerSize int64 // SplitLoggerSize The size of the log split
	Changelog       int32
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
	Enable                      bool
	GoroutineTriggerNumMin      int // goroutine trigger min in number
	GoroutineTriggerPercentDiff int // goroutine trigger diff in percent
	GoroutineTriggerNumAbs      int // goroutine trigger abs in number
	GoroutineTriggerNumMax      int // goroutine trigger max in number
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

func defaultLogConfigs() *logConfigs {
	return &logConfigs{
		RotateEnable:    true,
		SplitLoggerSize: defaultShardLoggerSize,
	}
}

func defaultGroupConfigs() *groupConfigs {
	return &groupConfigs{
		Enable:                      false,
		GoroutineTriggerNumAbs:      defaultGoroutineTriggerAbs,
		GoroutineTriggerPercentDiff: defaultGoroutineTriggerDiff,
		GoroutineTriggerNumMin:      defaultGoroutineTriggerMin,
	}
}

func defaultGCHeapOptions() *gcHeapConfigs {
	return &gcHeapConfigs{
		Enable:                   false,
		GCHeapTriggerPercentAbs:  defaultGCHeapTriggerAbs,
		GCHeapTriggerPercentDiff: defaultGCHeapTriggerDiff,
		GCHeapTriggerPercentMin:  defaultGCHeapTriggerMin,
	}
}

func defaultMemConfigs() *memConfigs {
	return &memConfigs{
		Enable:                false,
		MemTriggerPercentAbs:  defaultMemTriggerAbs,
		MemTriggerPercentDiff: defaultMemTriggerDiff,
		MemTriggerPercentMin:  defaultMemTriggerMin,
	}
}

func defaultCPUConfigs() *cpuConfigs {
	return &cpuConfigs{
		Enable:                false,
		CPUTriggerPercentAbs:  defaultCPUTriggerAbs,
		CPUTriggerPercentDiff: defaultCPUTriggerDiff,
		CPUTriggerPercentMin:  defaultCPUTriggerMin,
	}
}

func defaultThreadConfig() *threadConfigs {
	return &threadConfigs{
		Enable:                   false,
		ThreadTriggerPercentAbs:  defaultThreadTriggerAbs,
		ThreadTriggerPercentDiff: defaultThreadTriggerDiff,
		ThreadTriggerPercentMin:  defaultThreadTriggerMin,
	}
}

func defaultConfig() *configs {
	return &configs{
		logConfigs:      defaultLogConfigs(),
		GroupConfigs:    defaultGroupConfigs(),
		MemConfigs:      defaultMemConfigs(),
		GCHeapConfigs:   defaultGCHeapOptions(),
		CpuConfigs:      defaultCPUConfigs(),
		ThreadConfigs:   defaultThreadConfig(),
		LogLevel:        LogLevelDebug,
		Logger:          os.Stdout,
		CollectInterval: defaultInterval,
		CoolDown:        defaultCooldown,
		DumpPath:        defaultDumpPath,
		DumpProfileType: defaultDumpProfileType,
		DumpFullStack:   false,
	}
}
