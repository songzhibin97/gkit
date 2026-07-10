package cpu

import (
	"time"

	c "github.com/shirou/gopsutil/cpu"
)

type psutilCPU struct {
	interval time.Duration
}

func newPsutilCPU(interval time.Duration) (cpu *psutilCPU, err error) {
	cpu = &psutilCPU{interval: interval}
	_, err = cpu.Usage()
	if err != nil {
		return
	}
	return
}

func (ps *psutilCPU) Usage() (u uint64, err error) {
	var percents []float64
	percents, err = c.Percent(ps.interval, false)
	if err != nil {
		return 0, err
	}
	u = percentToUsage(percents)
	return
}

func percentToUsage(percents []float64) uint64 {
	if len(percents) == 0 {
		return 0
	}
	return uint64(percents[0] * 10)
}

func (ps *psutilCPU) Info() (info Info) {
	stats, err := c.Info()
	if err != nil || len(stats) == 0 {
		return
	}
	cores, err := c.Counts(true)
	if err != nil {
		return
	}
	info = buildCPUInfo(stats, cores)
	return
}

func buildCPUInfo(stats []c.InfoStat, cores int) Info {
	if len(stats) == 0 {
		return Info{}
	}
	return Info{Frequency: uint64(stats[0].Mhz), Quota: float64(cores)}
}
