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
	// gopsutil contracts to return one element when percpu=false, but exotic
	// environments (containerized arm boards, virtualized hosts without
	// /proc/stat) can hand back an empty slice with no error. Indexing [0]
	// there would panic the background sampling goroutine in cpu.go.
	if len(percents) == 0 {
		return 0, nil
	}
	u = uint64(percents[0] * 10)
	return
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
	info = Info{
		Frequency: uint64(stats[0].Mhz),
		Quota:     float64(cores),
	}
	return
}
