package watching

import (
	"os"
	"path"
	"path/filepath"
	"time"

	"github.com/docker/go-units"
	"github.com/songzhibin97/gkit/options"
)

// WithCollectInterval : interval must be valid time duration string,
// eg. "ns", "us" (or "µs"), "ms", "s", "m", "h".
func WithCollectInterval(interval string) options.Option {
	return func(o interface{}) {
		opts := o.(*Watching)
		var err error
		opts.config.CollectInterval, err = time.ParseDuration(interval)
		if err != nil {
			panic(err)
		}
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

// WithDumpPath set the dump path for holmes.
func WithDumpPath(dumpPath string, loginfo ...string) options.Option {
	return func(o interface{}) {
		opts := o.(*Watching)
		var err error
		f := path.Join(dumpPath, defaultLoggerName)
		if len(loginfo) > 0 {
			f = dumpPath + "/" + path.Join(loginfo...)
		}
		opts.config.DumpPath = filepath.Dir(f)
		opts.config.Logger, err = os.OpenFile(f, defaultLoggerFlags, defaultLoggerPerm)
		if err != nil && os.IsNotExist(err) {
			if err = os.MkdirAll(opts.config.DumpPath, 0o755); err != nil {
				return
			}
			opts.config.Logger, err = os.OpenFile(f, defaultLoggerFlags, defaultLoggerPerm)
			if err != nil {
				return
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
		opts.config.GroupConfigs.GoroutineTriggerNumMin = min
		opts.config.GroupConfigs.GoroutineTriggerPercentDiff = diff
		opts.config.GroupConfigs.GoroutineTriggerNumAbs = abs
		opts.config.GroupConfigs.GoroutineTriggerNumMax = max
		return
	}
}

// WithMemDump set the memory dump options.
func WithMemDump(min int, diff int, abs int) options.Option {
	return func(o interface{}) {
		opts := o.(*Watching)
		opts.config.MemConfigs.MemTriggerPercentMin = min
		opts.config.MemConfigs.MemTriggerPercentDiff = diff
		opts.config.MemConfigs.MemTriggerPercentAbs = abs
	}
}

// WithCPUDump set the cpu dump options.
func WithCPUDump(min int, diff int, abs int) options.Option {
	return func(o interface{}) {
		opts := o.(*Watching)
		opts.config.CpuConfigs.CPUTriggerPercentMin = min
		opts.config.CpuConfigs.CPUTriggerPercentDiff = diff
		opts.config.CpuConfigs.CPUTriggerPercentAbs = abs
		return
	}
}

// WithThreadDump set the thread dump options.
func WithThreadDump(min, diff, abs int) options.Option {
	return func(o interface{}) {
		opts := o.(*Watching)
		opts.config.ThreadConfigs.ThreadTriggerPercentMin = min
		opts.config.ThreadConfigs.ThreadTriggerPercentDiff = diff
		opts.config.ThreadConfigs.ThreadTriggerPercentAbs = abs
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

// WithCGroup set holmes use cgroup or not.
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
		opts.config.GCHeapConfigs.GCHeapTriggerPercentMin = min
		opts.config.GCHeapConfigs.GCHeapTriggerPercentDiff = diff
		opts.config.GCHeapConfigs.GCHeapTriggerPercentAbs = abs
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
