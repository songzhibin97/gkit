package cpu

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/pkg/errors"
	c "github.com/shirou/gopsutil/cpu"
)

type cGroupCPU struct {
	frequency uint64
	quota     float64
	cores     uint64

	preSystem uint64
	preTotal  uint64
	usage     uint64
}

func newCGroupCPU() (cpu *cGroupCPU, err error) {
	var cores int
	cores, err = c.Counts(true)
	if err != nil || cores == 0 {
		var cpus []uint64
		cpus, err = perCPUUsage()
		if err != nil {
			err = errors.Errorf("perCPUUsage() failed!err:=%v", err)
			return
		}
		cores = len(cpus)
	}

	sets, err := cpuSets()
	if err != nil {
		err = errors.Errorf("cpuSets() failed!err:=%v", err)
		return
	}
	quota := float64(len(sets))
	cq, err := cpuQuota()
	if err == nil && cq != -1 {
		var period uint64
		if period, err = cpuPeriod(); err != nil {
			err = errors.Errorf("cpuPeriod() failed!err:=%v", err)
			return
		}
		limit := float64(cq) / float64(period)
		if limit < quota {
			quota = limit
		}
	}
	maxFreq := cpuMaxFreq()

	preSystem, err := systemCPUUsage()
	if err != nil {
		err = errors.Errorf("systemCPUUsage() failed!err:=%v", err)
		return
	}
	preTotal, err := totalCPUUsage()
	if err != nil {
		err = errors.Errorf("totalCPUUsage() failed!err:=%v", err)
		return
	}
	cpu = &cGroupCPU{
		frequency: maxFreq,
		quota:     quota,
		cores:     uint64(cores),
		preSystem: preSystem,
		preTotal:  preTotal,
	}
	return
}

func (cpu *cGroupCPU) Usage() (u uint64, err error) {
	var (
		total  uint64
		system uint64
	)
	total, err = totalCPUUsage()
	if err != nil {
		return
	}
	system, err = systemCPUUsage()
	if err != nil {
		return
	}
	if system != cpu.preSystem {
		u = uint64(float64((total-cpu.preTotal)*cpu.cores*1e3) / (float64(system-cpu.preSystem) * cpu.quota))
	}
	cpu.preSystem = system
	cpu.preTotal = total
	return
}

func (cpu *cGroupCPU) Info() Info {
	return Info{
		Frequency: cpu.frequency,
		Quota:     cpu.quota,
	}
}

const nanoSecondsPerSecond = 1e9

// ErrNoCFSLimit: 没有配额限制
// var ErrNoCFSLimit = errors.Errorf("no quota limit")

var clockTicksPerSecond = uint64(getClockTicks())

// systemCPUUsage
// 以纳秒为单位返回主机系统的cpu使用情况。
// 如果基础文件的格式不匹配，则返回错误。
// 用POSIX定义的 /proc/stat 查找cpu 统计信息行，然后汇总提供的前七个字段
// 有关特定字段的详细信息，请参见 man 5 proc.
func systemCPUUsage() (usage uint64, err error) {
	var (
		line string
		f    *os.File
	)
	if f, err = os.Open("/proc/stat"); err != nil {
		return
	}
	bufReader := bufio.NewReaderSize(nil, 128)
	defer func() {
		bufReader.Reset(nil)
		_ = f.Close()
	}()
	bufReader.Reset(f)
	for err == nil {
		if line, err = bufReader.ReadString('\n'); err != nil {
			err = errors.WithStack(err)
			return
		}
		parts := strings.Fields(line)
		switch parts[0] {
		case "cpu":
			if len(parts) < 8 {
				err = errors.WithStack(fmt.Errorf("bad format of cpu stats"))
				return
			}
			var totalClockTicks uint64
			for _, i := range parts[1:8] {
				var v uint64
				if v, err = strconv.ParseUint(i, 10, 64); err != nil {
					err = errors.WithStack(fmt.Errorf("error parsing cpu stats"))
					return
				}
				totalClockTicks += v
			}
			usage = (totalClockTicks * nanoSecondsPerSecond) / clockTicksPerSecond
			return
		}
	}
	err = errors.Errorf("bad stats format")
	return
}

func totalCPUUsage() (usage uint64, err error) {
	var cg *cGroup
	if cg, err = currentcGroup(); err != nil {
		return
	}
	return cg.CPUAcctUsage()
}

func perCPUUsage() (usage []uint64, err error) {
	var cg *cGroup
	if cg, err = currentcGroup(); err != nil {
		return
	}
	return cg.CPUAcctUsagePerCPU()
}

func cpuSets() (sets []uint64, err error) {
	var cg *cGroup
	if cg, err = currentcGroup(); err != nil {
		return
	}
	return cg.CPUSetCPUs()
}

func cpuQuota() (quota int64, err error) {
	var cg *cGroup
	if cg, err = currentcGroup(); err != nil {
		return
	}
	return cg.CPUCFSQuotaUs()
}

func cpuPeriod() (peroid uint64, err error) {
	var cg *cGroup
	if cg, err = currentcGroup(); err != nil {
		return
	}
	return cg.CPUCFSPeriodUs()
}

func cpuFreq() uint64 {
	lines, err := readLines("/proc/cpuinfo")
	if err != nil {
		return 0
	}
	for _, line := range lines {
		fields := strings.Split(line, ":")
		if len(fields) < 2 {
			continue
		}
		key := strings.TrimSpace(fields[0])
		value := strings.TrimSpace(fields[1])
		if key == "cpu MHz" || key == "clock" {
			if t, err := strconv.ParseFloat(strings.Replace(value, "MHz", "", 1), 64); err == nil {
				return uint64(t * 1000.0 * 1000.0)
			}
		}
	}
	return 0
}

func cpuMaxFreq() uint64 {
	feq := cpuFreq()
	data, err := readFile("/sys/devices/system/cpu/cpu0/cpufreq/cpuinfo_max_freq")
	if err != nil {
		return feq
	}
	cfeq, err := parseUint(data)
	if err == nil {
		feq = cfeq
	}
	return feq
}

// getClockTicks 获取ticks 周期数
func getClockTicks() int {
	return 100
}
