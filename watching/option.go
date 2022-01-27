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
		opts := o.(*configs)
		var err error
		opts.CollectInterval, err = time.ParseDuration(interval)
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
		opts := o.(*configs)
		var err error
		opts.CoolDown, err = time.ParseDuration(coolDown)
		if err != nil {
			panic(err)
		}
		return
	}
}

// WithDumpPath set the dump path for holmes.
func WithDumpPath(dumpPath string, loginfo ...string) options.Option {
	return func(o interface{}) {
		opts := o.(*configs)
		var err error
		f := path.Join(dumpPath, defaultLoggerName)
		if len(loginfo) > 0 {
			f = dumpPath + "/" + path.Join(loginfo...)
		}
		opts.DumpPath = filepath.Dir(f)
		opts.Logger, err = os.OpenFile(f, defaultLoggerFlags, defaultLoggerPerm)
		if err != nil && os.IsNotExist(err) {
			if err = os.MkdirAll(opts.DumpPath, 0o755); err != nil {
				return
			}
			opts.Logger, err = os.OpenFile(f, defaultLoggerFlags, defaultLoggerPerm)
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
		opts := o.(*configs)
		opts.DumpProfileType = profileType
		return
	}
}

// WithLoggerSplit set the split log options.
// eg. "b/B", "k/K" "kb/Kb" "mb/Mb", "gb/Gb" "tb/Tb" "pb/Pb".
func WithLoggerSplit(enable bool, shardLoggerSize string) options.Option {
	return func(o interface{}) {
		opts := o.(*configs)
		opts.logConfigs.RotateEnable = enable
		if !enable {
			return
		}

		parseShardLoggerSize, err := units.FromHumanSize(shardLoggerSize)
		if err != nil {
			panic(err)
		}
		if parseShardLoggerSize <= 0 {
			opts.logConfigs.SplitLoggerSize = defaultShardLoggerSize
			return
		}
		opts.logConfigs.SplitLoggerSize = parseShardLoggerSize
		return
	}
}

// WithGoroutineDump set the goroutine dump options.
func WithGoroutineDump(min int, diff int, abs int, max int) options.Option {
	return func(o interface{}) {
		opts := o.(*configs)
		opts.GroupConfigs.GoroutineTriggerNumMin = min
		opts.GroupConfigs.GoroutineTriggerPercentDiff = diff
		opts.GroupConfigs.GoroutineTriggerNumAbs = abs
		opts.GroupConfigs.GoroutineTriggerNumMax = max
		return
	}
}

// WithMemDump set the memory dump options.
func WithMemDump(min int, diff int, abs int) options.Option {
	return func(o interface{}) {
		opts := o.(*configs)
		opts.MemConfigs.MemTriggerPercentMin = min
		opts.MemConfigs.MemTriggerPercentDiff = diff
		opts.MemConfigs.MemTriggerPercentAbs = abs
	}
}

// WithCPUDump set the cpu dump options.
func WithCPUDump(min int, diff int, abs int) options.Option {
	return func(o interface{}) {
		opts := o.(*configs)
		opts.CpuConfigs.CPUTriggerPercentMin = min
		opts.CpuConfigs.CPUTriggerPercentDiff = diff
		opts.CpuConfigs.CPUTriggerPercentAbs = abs
		return
	}
}

// WithThreadDump set the thread dump options.
func WithThreadDump(min, diff, abs int) options.Option {
	return func(o interface{}) {
		opts := o.(*configs)
		opts.ThreadConfigs.ThreadTriggerPercentMin = min
		opts.ThreadConfigs.ThreadTriggerPercentDiff = diff
		opts.ThreadConfigs.ThreadTriggerPercentAbs = abs
		return
	}
}

// WithLoggerLevel set logger level
func WithLoggerLevel(level int) options.Option {
	return func(o interface{}) {
		opts := o.(*configs)
		opts.LogLevel = level
		return
	}
}

// WithCGroup set holmes use cgroup or not.
func WithCGroup(useCGroup bool) options.Option {
	return func(o interface{}) {
		opts := o.(*configs)
		opts.UseCGroup = useCGroup
		return
	}
}
