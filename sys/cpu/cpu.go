package cpu

import (
	"fmt"
	"runtime"
	"sync/atomic"
	"time"
)

const (
	interval time.Duration = time.Millisecond * 500
)

var (
	stats   CPU
	usage   uint64
	initErr error
)

// CPU interface 定义CPU用法
type CPU interface {
	Usage() (u uint64, e error)
	Info() Info
}

// noopCPU is used when both cgroup and psutil probes fail. It lets the
// process keep running in restricted environments (sandboxes, distroless,
// containers with no /proc) where the previous init() panic would abort
// every importing binary at startup.
type noopCPU struct{}

func (noopCPU) Usage() (uint64, error) { return 0, nil }
func (noopCPU) Info() Info             { return Info{} }

// InitErr returns nil on success or the underlying error that caused CPU
// stats to fall back to a no-op source. Callers that need to fail loudly on
// missing CPU stats can check this at startup.
func InitErr() error { return initErr }

func init() {
	// 判断操作系统使用的是cGroup,如果不是cGroup退化为Psutil
	var err error
	stats, err = newCGroupCPU()
	if err != nil {
		var perr error
		stats, perr = newPsutilCPU(interval)
		if perr != nil {
			initErr = fmt.Errorf("cgroup init failed (%v) and psutil init failed (%v)", err, perr)
			stats = noopCPU{}
			return
		}
	}
	// 开启定时任务
	go func() {
		defer func() {
			if r := recover(); r != nil {
				// A panic in the sampling goroutine (e.g. an empty slice
				// indexed by an underlying probe) would otherwise kill the
				// whole process. Print a stack and stop sampling instead.
				buf := make([]byte, 64<<10)
				n := runtime.Stack(buf, false)
				fmt.Printf("sys/cpu: sampling goroutine panicked: %v\n%s\n", r, buf[:n])
			}
		}()
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			<-ticker.C
			u, err := stats.Usage()
			if err == nil && u != 0 {
				atomic.StoreUint64(&usage, u)
			}
		}
	}()
}

// Stat 状态信息
type Stat struct {
	// Usage: CPU使用率
	Usage uint64
}

// Info 详细信息
type Info struct {
	// Frequency: 频率
	Frequency uint64
	// Quota: 磁盘配额
	Quota float64
}

// ReadStat 读取状态
func ReadStat(stat *Stat) {
	stat.Usage = atomic.LoadUint64(&usage)
}

// GetInfo 获取信息
func GetInfo() Info {
	return stats.Info()
}
