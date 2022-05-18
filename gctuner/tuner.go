// Package gctuner implements https://github.com/bytedance/gopkg
package gctuner

import (
	"fmt"
	"github.com/docker/go-units"
	mem_util "github.com/shirou/gopsutil/mem"
	"io/ioutil"
	"math"
	"os"
	"runtime/debug"
	"strconv"
	"strings"
	"sync/atomic"
)

var (
	maxGCPercent uint32 = 500
	minGCPercent uint32 = 50
)

var defaultGCPercent uint32 = 100

func init() {
	gogcEnv := os.Getenv("GOGC")
	gogc, err := strconv.ParseInt(gogcEnv, 10, 32)
	if err != nil {
		return
	}
	defaultGCPercent = uint32(gogc)
}

// Tuning sets the threshold of heap which will be respect by gc tuner.
// When Tuning, the env GOGC will not be take effect.
// threshold: disable tuning if threshold == 0
func Tuning(threshold uint64) {
	// disable gc tuner if percent is zero
	if threshold <= 0 && globalTuner != nil {
		globalTuner.stop()
		globalTuner = nil
		return
	}

	if globalTuner == nil {
		globalTuner = newTuner(threshold)
		return
	}
	globalTuner.setThreshold(threshold)
}

// GetGCPercent returns the current GCPercent.
func GetGCPercent() uint32 {
	if globalTuner == nil {
		return defaultGCPercent
	}
	return globalTuner.getGCPercent()
}

// GetMaxGCPercent returns the max gc percent value.
func GetMaxGCPercent() uint32 {
	return atomic.LoadUint32(&maxGCPercent)
}

// SetMaxGCPercent sets the new max gc percent value.
func SetMaxGCPercent(n uint32) uint32 {
	return atomic.SwapUint32(&maxGCPercent, n)
}

// GetMinGCPercent returns the min gc percent value.
func GetMinGCPercent() uint32 {
	return atomic.LoadUint32(&minGCPercent)
}

// SetMinGCPercent sets the new min gc percent value.
func SetMinGCPercent(n uint32) uint32 {
	return atomic.SwapUint32(&minGCPercent, n)
}

// only allow one gc tuner in one process
var globalTuner *tuner = nil

/* Heap
 _______________  => limit: host/cgroup memory hard limit
|               |
|---------------| => threshold: increase GCPercent when gc_trigger < threshold
|               |
|---------------| => gc_trigger: heap_live + heap_live * GCPercent / 100
|               |
|---------------|
|   heap_live   |
|_______________|

Go runtime only trigger GC when hit gc_trigger which affected by GCPercent and heap_live.
So we can change GCPercent dynamically to tuning GC performance.
*/
type tuner struct {
	finalizer *finalizer
	gcPercent uint32
	threshold uint64 // high water level, in bytes
}

// tuning check the memory inuse and tune GC percent dynamically.
// Go runtime ensure that it will be called serially.
func (t *tuner) tuning() {
	inuse := readMemoryInuse()
	threshold := t.getThreshold()
	// stop gc tuning
	if threshold <= 0 {
		return
	}
	t.setGCPercent(calcGCPercent(inuse, threshold))
	return
}

// threshold = inuse + inuse * (gcPercent / 100)
// => gcPercent = (threshold - inuse) / inuse * 100
// if threshold < inuse*2, so gcPercent < 100, and GC positively to avoid OOM
// if threshold > inuse*2, so gcPercent > 100, and GC negatively to reduce GC times
func calcGCPercent(inuse, threshold uint64) uint32 {
	// invalid params
	if inuse == 0 || threshold == 0 {
		return defaultGCPercent
	}
	// inuse heap larger than threshold, use min percent
	if threshold <= inuse {
		return minGCPercent
	}
	gcPercent := uint32(math.Floor(float64(threshold-inuse) / float64(inuse) * 100))
	if gcPercent < minGCPercent {
		return minGCPercent
	} else if gcPercent > maxGCPercent {
		return maxGCPercent
	}
	return gcPercent
}

func newTuner(threshold uint64) *tuner {
	t := &tuner{
		gcPercent: defaultGCPercent,
		threshold: threshold,
	}
	t.finalizer = newFinalizer(t.tuning) // start tuning
	return t
}

func (t *tuner) stop() {
	t.finalizer.stop()
}

func (t *tuner) setThreshold(threshold uint64) {
	atomic.StoreUint64(&t.threshold, threshold)
}

func (t *tuner) getThreshold() uint64 {
	return atomic.LoadUint64(&t.threshold)
}

func (t *tuner) setGCPercent(percent uint32) uint32 {
	atomic.StoreUint32(&t.gcPercent, percent)
	return uint32(debug.SetGCPercent(int(percent)))
}

func (t *tuner) getGCPercent() uint32 {
	return atomic.LoadUint32(&t.gcPercent)
}

// TuningWithFromHuman
// eg. "b/B", "k/K" "kb/Kb" "mb/Mb", "gb/Gb" "tb/Tb" "pb/Pb".
func TuningWithFromHuman(threshold string) {
	parseThreshold, err := units.FromHumanSize(threshold)
	if err != nil {
		fmt.Println("format err:", err)
		return
	}
	Tuning(uint64(parseThreshold))
}

// TuningWithAuto By automatic calculation of the total amount
func TuningWithAuto(isContainer bool) {
	var (
		threshold uint64
		err       error
	)
	if isContainer {
		threshold, err = getCGroupMemoryLimit()
	} else {
		threshold, err = getNormalMemoryLimit()
	}
	if err != nil {
		fmt.Println("get memery err:", err)
		return
	}
	Tuning(uint64(float64(threshold) * 0.7))
}

const cgroupMemLimitPath = "/sys/fs/cgroup/memory/memory.limit_in_bytes"

func getCGroupMemoryLimit() (uint64, error) {
	usage, err := readUint(cgroupMemLimitPath)
	if err != nil {
		return 0, err
	}
	machineMemory, err := mem_util.VirtualMemory()
	if err != nil {
		return 0, err
	}
	limit := uint64(math.Min(float64(usage), float64(machineMemory.Total)))
	return limit, nil
}

func getNormalMemoryLimit() (uint64, error) {
	machineMemory, err := mem_util.VirtualMemory()
	if err != nil {
		return 0, err
	}
	return machineMemory.Total, nil
}

// copied from https://github.com/containerd/cgroups/blob/318312a373405e5e91134d8063d04d59768a1bff/utils.go#L251
func parseUint(s string, base, bitSize int) (uint64, error) {
	v, err := strconv.ParseUint(s, base, bitSize)
	if err != nil {
		intValue, intErr := strconv.ParseInt(s, base, bitSize)
		// 1. Handle negative values greater than MinInt64 (and)
		// 2. Handle negative values lesser than MinInt64
		if intErr == nil && intValue < 0 {
			return 0, nil
		} else if intErr != nil &&
			intErr.(*strconv.NumError).Err == strconv.ErrRange &&
			intValue < 0 {
			return 0, nil
		}
		return 0, err
	}
	return v, nil
}

func readUint(path string) (uint64, error) {
	v, err := ioutil.ReadFile(path)
	if err != nil {
		return 0, err
	}
	return parseUint(strings.TrimSpace(string(v)), 10, 64)
}
