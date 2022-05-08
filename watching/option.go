package watching

import (
	"os"
	"path"
	"path/filepath"
	"sync/atomic"
	"time"
	"unsafe"

	"github.com/docker/go-units"
	"github.com/songzhibin97/gkit/options"
)

// WithCollectInterval : interval must be valid time duration string,
// eg. "ns", "us" (or "µs"), "ms", "s", "m", "h".
func WithCollectInterval(interval string) options.Option {
	return func(o interface{}) {
		opts := o.(*Watching)
		var err error
		// CollectInterval wouldn't be zero value, because it
		// will be initialized as defaultInterval at newOptions()
		newInterval, err := time.ParseDuration(interval)
		if err != nil || opts.config.CollectInterval.Seconds() == newInterval.Seconds() {
			return
		}
		opts.config.CollectInterval = newInterval
		opts.config.intervalResetting <- struct{}{}
		return
	}
}

// WithCoolDown : coolDown must be valid time duration string,
// eg. "ns", "us" (or "µs"), "ms", "s", "m", "h".
func WithCoolDown(coolDown string) options.Option {
	return func(o interface{}) {
		opts := o.(*Watching)
		var err error
		opts.config.CoolDown, err = time.ParseDuration(coolDown)
		if err != nil {
			panic(err)
		}
		return
	}
}

// WithCPUMax : set the CPUMaxPercent parameter as max
func WithCPUMax(max int) options.Option {
	return func(o interface{}) {
		opts := o.(*Watching)
		opts.config.CPUMaxPercent = max
	}
}

// WithDumpPath set the dump path for Watching.
func WithDumpPath(dumpPath string, loginfo ...string) options.Option {
	return func(o interface{}) {
		opts := o.(*Watching)
		var err error
		var logger *os.File
		f := path.Join(dumpPath, defaultLoggerName)
		if len(loginfo) > 0 {
			f = dumpPath + "/" + path.Join(loginfo...)
		}
		opts.config.DumpPath = filepath.Dir(f)
		logger, err = os.OpenFile(f, defaultLoggerFlags, defaultLoggerPerm)
		if err != nil && os.IsNotExist(err) {
			if err = os.MkdirAll(opts.config.DumpPath, 0o755); err != nil {
				return
			}
			logger, err = os.OpenFile(f, defaultLoggerFlags, defaultLoggerPerm)
			if err != nil {
				return
			}
		}
		old := opts.config.Logger
		if atomic.CompareAndSwapPointer((*unsafe.Pointer)(unsafe.Pointer(&opts.config.Logger)), unsafe.Pointer(opts.config.Logger), unsafe.Pointer(logger)) {
			if old != os.Stdout {
				_ = old.Close()
			}
		}
		return
	}
}

// WithBinaryDump set dump mode to binary.
func WithBinaryDump() options.Option {
	return withDumpProfileType(binaryDump)
}

// WithTextDump set dump mode to text.
func WithTextDump() options.Option {
	return withDumpProfileType(textDump)
}

// WithFullStack set to dump full stack or top 10 stack, when dump in text mode.
func WithFullStack(isFull bool) options.Option {
	return func(o interface{}) {
		opts := o.(*Watching)
		opts.config.DumpFullStack = isFull
		return
	}
}

func withDumpProfileType(profileType dumpProfileType) options.Option {
	return func(o interface{}) {
		opts := o.(*Watching)
		opts.config.DumpProfileType = profileType
		return
	}
}

// WithLoggerSplit set the split log options.
// eg. "b/B", "k/K" "kb/Kb" "mb/Mb", "gb/Gb" "tb/Tb" "pb/Pb".
func WithLoggerSplit(enable bool, shardLoggerSize string) options.Option {
	return func(o interface{}) {
		opts := o.(*Watching)
		opts.config.logConfigs.RotateEnable = enable
		if !enable {
			return
		}

		parseShardLoggerSize, err := units.FromHumanSize(shardLoggerSize)
		if err != nil {
			panic(err)
		}
		if parseShardLoggerSize <= 0 {
			opts.config.logConfigs.SplitLoggerSize = defaultShardLoggerSize
			return
		}
		opts.config.logConfigs.SplitLoggerSize = parseShardLoggerSize
		return
	}
}

// WithGoroutineDump set the goroutine dump options.
func WithGoroutineDump(min int, diff int, abs int, max int) options.Option {
	return func(o interface{}) {
		opts := o.(*Watching)
		opts.config.GroupConfigs.TriggerMin = min
		opts.config.GroupConfigs.TriggerDiff = diff
		opts.config.GroupConfigs.TriggerAbs = abs
		opts.config.GroupConfigs.GoroutineTriggerNumMax = max
		return
	}
}

// WithMemDump set the memory dump options.
func WithMemDump(min int, diff int, abs int) options.Option {
	return func(o interface{}) {
		opts := o.(*Watching)
		opts.config.MemConfigs.TriggerMin = min
		opts.config.MemConfigs.TriggerDiff = diff
		opts.config.MemConfigs.TriggerAbs = abs
	}
}

// WithCPUDump set the cpu dump options.
func WithCPUDump(min int, diff int, abs int) options.Option {
	return func(o interface{}) {
		opts := o.(*Watching)
		opts.config.CpuConfigs.TriggerMin = min
		opts.config.CpuConfigs.TriggerDiff = diff
		opts.config.CpuConfigs.TriggerAbs = abs
		return
	}
}

// WithGoProcAsCPUCore set Watching use cgroup or not.
func WithGoProcAsCPUCore(enabled bool) options.Option {
	return func(o interface{}) {
		opts := o.(*Watching)
		opts.config.UseGoProcAsCPUCore = enabled
		return
	}
}

// WithThreadDump set the thread dump options.
func WithThreadDump(min, diff, abs int) options.Option {
	return func(o interface{}) {
		opts := o.(*Watching)
		opts.config.ThreadConfigs.TriggerMin = min
		opts.config.ThreadConfigs.TriggerDiff = diff
		opts.config.ThreadConfigs.TriggerAbs = abs
		return
	}
}

// WithLoggerLevel set logger level
func WithLoggerLevel(level int) options.Option {
	return func(o interface{}) {
		opts := o.(*Watching)
		opts.config.LogLevel = level
		return
	}
}

// WithCGroup set Watching use cgroup or not.
func WithCGroup(useCGroup bool) options.Option {
	return func(o interface{}) {
		opts := o.(*Watching)
		opts.config.UseCGroup = useCGroup
		return
	}
}

// WithGCHeapDump set the GC heap dump options.
func WithGCHeapDump(min int, diff int, abs int) options.Option {
	return func(o interface{}) {
		opts := o.(*Watching)
		opts.config.GCHeapConfigs.TriggerMin = min
		opts.config.GCHeapConfigs.TriggerDiff = diff
		opts.config.GCHeapConfigs.TriggerAbs = abs
		return
	}
}

// WithCPUCore overwrite the system level CPU core number when it > 0.
// it's not a good idea to modify it on fly since it affects the CPU percent caculation.
func WithCPUCore(cpuCore float64) options.Option {
	return func(o interface{}) {
		opts := o.(*Watching)
		opts.config.cpuCore = cpuCore
		return
	}
}

// WithMemoryLimit overwrite the system level memory limit when it > 0.
func WithMemoryLimit(limit uint64) options.Option {
	return func(o interface{}) {
		opts := o.(*Watching)
		opts.config.memoryLimit = limit
		return
	}
}

// WithShrinkThread enable/disable shrink thread when the thread number exceed the max threshold.
func WithShrinkThread(enable bool, threshold int, delay time.Duration) options.Option {
	return func(o interface{}) {
		opts := o.(*Watching)
		opts.config.ShrinkThrConfigs.Enable = enable
		if threshold > 0 {
			opts.config.ShrinkThrConfigs.Threshold = threshold
		}
		opts.config.ShrinkThrConfigs.Delay = delay
		return
	}
}

// WithProfileReporter will enable reporter
// reopens profile reporter through WithProfileReporter(h.opts.rptOpts.reporter)
func WithProfileReporter(r ProfileReporter) options.Option {
	return func(o interface{}) {
		opts := o.(*Watching)
		if r == nil {
			return
		}

		opts.config.rptConfigs.reporter = r
		atomic.StoreInt32(&opts.config.rptConfigs.active, 1)
		return
	}
}
