package watching

import (
	"bytes"
	"fmt"
	"os"
	"runtime"
	"runtime/pprof"
	"sync/atomic"
	"time"

	"github.com/songzhibin97/gkit/options"
)

type Watching struct {
	config *configs

	// stats
	collectCount       int
	gcCycleCount       int
	threadTriggerCount int
	cpuTriggerCount    int
	memTriggerCount    int
	grTriggerCount     int
	gcHeapTriggerCount int
	// channel for GC sweep finalizer event
	finCh chan time.Time

	// cooldown
	threadCoolDownTime time.Time
	cpuCoolDownTime    time.Time
	memCoolDownTime    time.Time
	gcHeapCoolDownTime time.Time
	grCoolDownTime     time.Time

	// GC heap triggered, need to dump next time.
	gcHeapTriggered bool

	// stats ring
	memStats    ring
	cpuStats    ring
	grNumStats  ring
	threadStats ring
	gcHeapStats ring

	// switch
	stopped int64
}

// EnableThreadDump enables the goroutine dump.
func (w *Watching) EnableThreadDump() *Watching {
	w.config.ThreadConfigs.Enable = true
	return w
}

// DisableThreadDump disables the goroutine dump.
func (w *Watching) DisableThreadDump() *Watching {
	w.config.ThreadConfigs.Enable = false
	return w
}

// EnableGoroutineDump enables the goroutine dump.
func (w *Watching) EnableGoroutineDump() *Watching {
	w.config.GroupConfigs.Enable = true
	return w
}

// DisableGoroutineDump disables the goroutine dump.
func (w *Watching) DisableGoroutineDump() *Watching {
	w.config.GroupConfigs.Enable = false
	return w
}

// EnableCPUDump enables the CPU dump.
func (w *Watching) EnableCPUDump() *Watching {
	w.config.CpuConfigs.Enable = true
	return w
}

// DisableCPUDump disables the CPU dump.
func (w *Watching) DisableCPUDump() *Watching {
	w.config.CpuConfigs.Enable = false
	return w
}

// EnableGCHeapDump enables the GC heap dump.
func (w *Watching) EnableGCHeapDump() *Watching {
	w.config.GCHeapConfigs.Enable = true
	w.finCh = make(chan time.Time)
	return w
}

// EnableMemDump enables the mem dump.
func (w *Watching) EnableMemDump() *Watching {
	w.config.MemConfigs.Enable = true
	return w
}

// DisableMemDump disables the mem dump.
func (w *Watching) DisableMemDump() *Watching {
	w.config.MemConfigs.Enable = false
	return w
}

func finalizerCallback(gc *gcHeapFinalizer) {
	// register the finalizer again
	runtime.SetFinalizer(gc, finalizerCallback)

	select {
	case gc.w.finCh <- time.Time{}:
	default:
		gc.w.logf("can not send event to finalizer channel immediately, may be analyzer blocked?")
	}
}

type gcHeapFinalizer struct {
	w *Watching
}

func (w *Watching) startGCCycleLoop() {
	w.gcHeapStats = newRing(minCollectCyclesBeforeDumpStart)

	gc := &gcHeapFinalizer{
		w,
	}

	runtime.SetFinalizer(w, finalizerCallback)

	go gc.w.gcHeapCheckLoop()
}

// Start starts the dump loop of Watching.
func (w *Watching) Start() {
	atomic.StoreInt64(&w.stopped, 0)
	w.initEnvironment()
	go w.startDumpLoop()
	w.startGCCycleLoop()
}

func (w *Watching) startDumpLoop() {
	// init previous cool down time
	now := time.Now()
	w.cpuCoolDownTime = now
	w.memCoolDownTime = now
	w.grCoolDownTime = now

	// init stats ring
	w.cpuStats = newRing(minCollectCyclesBeforeDumpStart)
	w.memStats = newRing(minCollectCyclesBeforeDumpStart)
	w.grNumStats = newRing(minCollectCyclesBeforeDumpStart)
	w.threadStats = newRing(minCollectCyclesBeforeDumpStart)

	// dump loop
	ticker := time.NewTicker(w.config.CollectInterval)
	defer ticker.Stop()
	for range ticker.C {
		if atomic.LoadInt64(&w.stopped) == 1 {
			fmt.Println("[Watching] dump loop stopped")
			return
		}

		cpu, mem, gNum, tNum, err := collect()
		if err != nil {
			w.logf(err.Error())
			continue
		}

		w.cpuStats.push(cpu)
		w.memStats.push(mem)
		w.grNumStats.push(gNum)
		w.threadStats.push(tNum)

		w.collectCount++
		if w.collectCount < minCollectCyclesBeforeDumpStart {
			// at least collect some cycles
			// before start to judge and dump
			w.logf("[Watching] warming up cycle : %d", w.collectCount)
			continue
		}

		if err := w.EnableDump(cpu); err != nil {
			w.logf("[Watching] unable to dump: %v", err)
			continue
		}

		w.goroutineCheckAndDump(gNum)
		w.memCheckAndDump(mem)
		w.cpuCheckAndDump(cpu)
		w.threadCheckAndDump(tNum)
	}
}

// goroutine start.
func (w *Watching) goroutineCheckAndDump(gNum int) {
	if !w.config.GroupConfigs.Enable {
		return
	}

	if w.grCoolDownTime.After(time.Now()) {
		w.logf("[Watching] goroutine dump is in cooldown")
		return
	}

	if triggered := w.goroutineProfile(gNum); triggered {
		w.grCoolDownTime = time.Now().Add(w.config.CoolDown)
		w.grTriggerCount++
	}
}

func (w *Watching) goroutineProfile(gNum int) bool {
	c := w.config.GroupConfigs
	if !matchRule(w.grNumStats, gNum, c.GoroutineTriggerNumMin, c.GoroutineTriggerNumAbs, c.GoroutineTriggerPercentDiff, c.GoroutineTriggerNumMax) {
		w.debugf(UniformLogFormat, "NODUMP", type2name[goroutine],
			c.GoroutineTriggerNumMin, c.GoroutineTriggerPercentDiff, c.GoroutineTriggerNumAbs,
			c.GoroutineTriggerNumMax, w.grNumStats.data, gNum)
		return false
	}

	var buf bytes.Buffer
	_ = pprof.Lookup("goroutine").WriteTo(&buf, int(w.config.DumpProfileType)) // nolint: errcheck
	w.writeProfileDataToFile(buf, goroutine, gNum)

	return true
}

// memory start.
func (w *Watching) memCheckAndDump(mem int) {
	if !w.config.MemConfigs.Enable {
		return
	}

	if w.memCoolDownTime.After(time.Now()) {
		w.logf("[Watching] mem dump is in cooldown")
		return
	}

	if triggered := w.memProfile(mem); triggered {
		w.memCoolDownTime = time.Now().Add(w.config.CoolDown)
		w.memTriggerCount++
	}
}

func (w *Watching) memProfile(rss int) bool {
	c := w.config.MemConfigs
	if !matchRule(w.memStats, rss, c.MemTriggerPercentMin, c.MemTriggerPercentAbs, c.MemTriggerPercentDiff, NotSupportTypeMaxConfig) {
		// let user know why this should not dump
		w.debugf(UniformLogFormat, "NODUMP", type2name[mem],
			c.MemTriggerPercentMin, c.MemTriggerPercentDiff, c.MemTriggerPercentAbs, NotSupportTypeMaxConfig,
			w.memStats.data, rss)

		return false
	}

	var buf bytes.Buffer
	_ = pprof.Lookup("heap").WriteTo(&buf, int(w.config.DumpProfileType)) // nolint: errcheck
	w.writeProfileDataToFile(buf, mem, rss)
	return true
}

// thread start.
func (w *Watching) threadCheckAndDump(threadNum int) {
	if !w.config.ThreadConfigs.Enable {
		return
	}

	if w.threadCoolDownTime.After(time.Now()) {
		w.logf("[Watching] thread dump is in cooldown")
		return
	}

	if triggered := w.threadProfile(threadNum); triggered {
		w.threadCoolDownTime = time.Now().Add(w.config.CoolDown)
		w.threadTriggerCount++
	}
}

func (w *Watching) threadProfile(curThreadNum int) bool {
	c := w.config.ThreadConfigs
	if !matchRule(w.threadStats, curThreadNum, c.ThreadTriggerPercentMin, c.ThreadTriggerPercentAbs, c.ThreadTriggerPercentDiff, NotSupportTypeMaxConfig) {
		// let user know why this should not dump
		w.debugf(UniformLogFormat, "NODUMP", type2name[thread],
			c.ThreadTriggerPercentMin, c.ThreadTriggerPercentDiff, c.ThreadTriggerPercentAbs, NotSupportTypeMaxConfig,
			w.threadStats.data, curThreadNum)

		return false
	}

	var buf bytes.Buffer
	_ = pprof.Lookup("threadcreate").WriteTo(&buf, int(w.config.DumpProfileType)) // nolint: errcheck
	_ = pprof.Lookup("goroutine").WriteTo(&buf, int(w.config.DumpProfileType))    // nolint: errcheck

	w.writeProfileDataToFile(buf, thread, curThreadNum)

	return true
}

// cpu start.
func (w *Watching) cpuCheckAndDump(cpu int) {
	if !w.config.CpuConfigs.Enable {
		return
	}

	if w.cpuCoolDownTime.After(time.Now()) {
		w.logf("[Watching] cpu dump is in cooldown")
		return
	}

	if triggered := w.cpuProfile(cpu); triggered {
		w.cpuCoolDownTime = time.Now().Add(w.config.CoolDown)
		w.cpuTriggerCount++
	}
}

func (w *Watching) cpuProfile(curCPUUsage int) bool {
	c := w.config.CpuConfigs
	if !matchRule(w.cpuStats, curCPUUsage, c.CPUTriggerPercentMin, c.CPUTriggerPercentAbs, c.CPUTriggerPercentDiff, NotSupportTypeMaxConfig) {
		// let user know why this should not dump
		w.debugf(UniformLogFormat, "NODUMP", type2name[cpu],
			c.CPUTriggerPercentMin, c.CPUTriggerPercentDiff, c.CPUTriggerPercentAbs, NotSupportTypeMaxConfig,
			w.cpuStats.data, curCPUUsage)

		return false
	}

	binFileName := getBinaryFileName(w.config.DumpPath, cpu)

	bf, err := os.OpenFile(binFileName, defaultLoggerFlags, defaultLoggerPerm)
	if err != nil {
		w.logf("[Watching] failed to create cpu profile file: %v", err.Error())
		return false
	}
	defer bf.Close()

	err = pprof.StartCPUProfile(bf)
	if err != nil {
		w.logf("[Watching] failed to profile cpu: %v", err.Error())
		return false
	}

	time.Sleep(defaultCPUSamplingTime)
	pprof.StopCPUProfile()

	w.logf(UniformLogFormat, "pprof dump to log dir", type2name[cpu],
		c.CPUTriggerPercentMin, c.CPUTriggerPercentDiff, c.CPUTriggerPercentAbs, NotSupportTypeMaxConfig,
		w.cpuStats.data, curCPUUsage)

	return true
}

func (w *Watching) gcHeapCheckLoop() {
	for {
		// wait for the finalizer event
		<-w.finCh

		if !w.config.GCHeapConfigs.Enable {
			return
		}

		w.gcHeapCheckAndDump()
	}
}

func (w *Watching) gcHeapCheckAndDump() {
	memStats := new(runtime.MemStats)
	runtime.ReadMemStats(memStats)

	// TODO: we can only use NextGC for now since runtime haven't expose heapmarked yet
	// and we hard code the gcPercent is 100 here.
	// may introduce a new API debug.GCHeapMarked? it can also has better performance(no STW).
	nextGC := memStats.NextGC
	prevGC := nextGC / 2 //nolint:gomnd

	memoryLimit, err := w.getMemoryLimit()
	if memoryLimit == 0 || err != nil {
		w.logf("[Watching] get memory limit failed, memory limit: %v, error: %v", memoryLimit, err)
		return
	}

	ratio := int(100 * float64(prevGC) / float64(memoryLimit))
	w.gcHeapStats.push(ratio)

	w.gcCycleCount++
	if w.gcCycleCount < minCollectCyclesBeforeDumpStart {
		// at least collect some cycles
		// before start to judge and dump
		w.logf("[Watching] GC cycle warming up : %d", w.gcCycleCount)
		return
	}

	if w.gcHeapCoolDownTime.After(time.Now()) {
		w.logf("[Watching] GC heap dump is in cooldown")
		return
	}

	if triggered := w.gcHeapProfile(ratio, w.gcHeapTriggered); triggered {
		if w.gcHeapTriggered {
			// already dump twice, mark it false
			w.gcHeapTriggered = false
			w.gcHeapCoolDownTime = time.Now().Add(w.config.CoolDown)
			w.gcHeapTriggerCount++
		} else {
			// force dump next time
			w.gcHeapTriggered = true
		}
	}
}

// gcHeapProfile will dump profile twice when triggered once.
// since the current memory profile will be merged after next GC cycle.
// And we assume the finalizer will be called before next GC cycle(it will be usually).
func (w *Watching) gcHeapProfile(gc int, force bool) bool {
	c := w.config.GCHeapConfigs
	if !force && !matchRule(w.gcHeapStats, gc, c.GCHeapTriggerPercentMin, c.GCHeapTriggerPercentAbs, c.GCHeapTriggerPercentDiff, NotSupportTypeMaxConfig) {
		// let user know why this should not dump
		w.debugf(UniformLogFormat, "NODUMP", type2name[gcHeap],
			c.GCHeapTriggerPercentMin, c.GCHeapTriggerPercentDiff, c.GCHeapTriggerPercentAbs, NotSupportTypeMaxConfig,
			w.gcHeapStats.data, gc)

		return false
	}

	var buf bytes.Buffer
	_ = pprof.Lookup("heap").WriteTo(&buf, int(w.config.DumpProfileType)) // nolint: errcheck
	w.writeProfileDataToFile(buf, gcHeap, gc)

	return true
}

func (w *Watching) initEnvironment() {
	// choose whether the max memory is limited by cgroup
	if w.config.UseCGroup {
		// use cgroup
		getUsage = getUsageCGroup
		w.logf("[Watching] use cgroup to limit memory")
	} else {
		// not use cgroup
		getUsage = getUsageNormal
		w.logf("[Watching] use the default memory percent calculated by gopsutil")
	}
	if w.config.Logger == os.Stdout && w.config.logConfigs.RotateEnable {
		w.config.logConfigs.RotateEnable = false
	}
}

func (w *Watching) EnableDump(curCPU int) (err error) {
	if w.config.CPUMaxPercent != 0 && curCPU >= w.config.CPUMaxPercent {
		return fmt.Errorf("current cpu percent [%v] is greater than the CPUMaxPercent [%v]", cpu, w.config.CPUMaxPercent)
	}
	return nil
}

// Stop the dump loop.
func (w *Watching) Stop() {
	atomic.StoreInt64(&w.stopped, 1)
}

func (w *Watching) writeProfileDataToFile(data bytes.Buffer, dumpType configureType, currentStat int) {
	binFileName := getBinaryFileName(w.config.DumpPath, dumpType)

	switch dumpType {
	case mem:
		opts := w.config.MemConfigs
		w.logf(UniformLogFormat, "pprof", type2name[dumpType],
			opts.MemTriggerPercentMin, opts.MemTriggerPercentDiff, opts.MemTriggerPercentAbs, NotSupportTypeMaxConfig,
			w.memStats.data, currentStat)
	case gcHeap:
		opts := w.config.GCHeapConfigs
		w.logf(UniformLogFormat, "pprof", type2name[dumpType],
			opts.GCHeapTriggerPercentMin, opts.GCHeapTriggerPercentDiff, opts.GCHeapTriggerPercentAbs, NotSupportTypeMaxConfig,
			w.gcHeapStats.data, currentStat)
	case goroutine:
		opts := w.config.GroupConfigs
		w.logf(UniformLogFormat, "pprof", type2name[dumpType],
			opts.GoroutineTriggerNumMin, opts.GoroutineTriggerPercentDiff, opts.GoroutineTriggerNumAbs, opts.GoroutineTriggerNumMax,
			w.grNumStats.data, currentStat)
	case thread:
		opts := w.config.ThreadConfigs
		w.logf(UniformLogFormat, "pprof", type2name[dumpType],
			opts.ThreadTriggerPercentMin, opts.ThreadTriggerPercentDiff, opts.ThreadTriggerPercentAbs, NotSupportTypeMaxConfig,
			w.threadStats.data, currentStat)
	}

	if w.config.DumpProfileType == textDump {
		// write to log
		res := data.String()
		if !w.config.DumpFullStack {
			res = trimResult(data)
		}
		w.logf(res)
	} else {
		bf, err := os.OpenFile(binFileName, defaultLoggerFlags, defaultLoggerPerm)
		if err != nil {
			w.logf("[Watching] pprof %v write to file failed : %v", type2name[dumpType], err.Error())
			return
		}
		defer bf.Close()

		if _, err = bf.Write(data.Bytes()); err != nil {
			w.logf("[Watching] pprof %v write to file failed : %v", type2name[dumpType], err.Error())
		}
	}
}

func (w *Watching) getMemoryLimit() (uint64, error) {
	if w.config.memoryLimit > 0 {
		return w.config.memoryLimit, nil
	}

	if w.config.UseCGroup {
		return getCGroupMemoryLimit()
	}
	return getNormalMemoryLimit()
}

func NewWatching(opts ...options.Option) *Watching {
	watching := &Watching{config: defaultConfig()}
	for _, opt := range opts {
		opt(watching)
	}
	return watching
}
