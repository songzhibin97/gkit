package cpu

import (
	"fmt"
	"sync/atomic"
	"time"
)

const (
	interval time.Duration = time.Millisecond * 500
)

var (
	stats CPU
	usage uint64
)

// CPU interface 定义CPU用法
type CPU interface {
	Usage() (u uint64, e error)
	Info() Info
}

func init() {
	var err error
	// 判断操作系统使用的是cGroup,如果不是cGroup退化为Psutil
	stats, err = newCGroupCPU()
	if err != nil {
		stats, err = newPsutilCPU(interval)
		if err != nil {
			panic(fmt.Sprintf("cGroup cpu init failed!err:=%v", err))
		}
	}
	// 开启定时任务
	go func() {
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
