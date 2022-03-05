package watching

import (
	"bytes"
	"fmt"
	"os"
	"runtime"
	"runtime/pprof"
	"sync"
	"sync/atomic"
	"time"

	"github.com/songzhibin97/gkit/options"
)

type Watching struct {
	config *configs

	// stats
	changeLog                int32
	collectCount             int
	gcCycleCount             int
	threadTriggerCount       int
	cpuTriggerCount          int
	memTriggerCount          int
	grTriggerCount           int
	gcHeapTriggerCount       int
	shrinkThreadTriggerCount int
	// channel for GC sweep finalizer event
	finCh chan struct{}

	// cooldown
	threadCoolDownTime    time.Time
	cpuCoolDownTime       time.Time
	memCoolDownTime       time.Time
	gcHeapCoolDownTime    time.Time
	grCoolDownTime        time.Time
	shrinkThrCoolDownTime time.Time

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

// EnableGCHeapDump enables the GC heap dump.
func (w *Watching) EnableGCHeapDump() *Watching {
	w.config.GCHeapConfigs.Enable = true
	return w
}

// DisableGCHeapDump disables the GC heap dump.
func (w *Watching) DisableGCHeapDump() *Watching {
	w.config.GCHeapConfigs.Enable = false
	return w
}

func finalizerCallback(gc *gcHeapFinalizer) {
	// disable or stop gc clean up normally
	if atomic.LoadInt64(&gc.w.stopped) == 1 {
		return
	}

	// register the finalizer again
	runtime.SetFinalizer(gc, finalizerCallback)

	select {
	case gc.w.finCh <- struct{}{}:
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

	runtime.SetFinalizer(gc, finalizerCallback)

	go gc.w.gcHeapCheckLoop()
}

// Start starts the dump loop of Watching.
func (w *Watching) Start() {
	if !atomic.CompareAndSwapInt64(&w.stopped, 1, 0) {
		return
	}
	w.initEnvironment()
	go w.startDumpLoop()
	w.startGCCycleLoop()
}

// Stop the dump loop.
func (w *Watching) Stop() {
	atomic.StoreInt64(&w.stopped, 1)
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

	for {
		select {
		case <-w.config.intervalResetting:
			// wait for go version update to 1.15
			// can use Reset API directly here. pkg.go.dev/time#Ticker.Reset
			// we can't use the `for-range` here, because the range loop
			// caches the variable to be lopped and then it can't be overwritten
			itv := w.config.CollectInterval
			fmt.Printf("[Holmes] collect interval is resetting to [%v]\n", itv) //nolint:forbidigo
			ticker = time.NewTicker(itv)
		default:
			<-ticker.C
			if atomic.LoadInt64(&w.stopped) == 1 {
				fmt.Println("[Watching] dump loop stopped")
				return
			}
			cpuCore, err := w.getCPUCore()
			if cpuCore == 0 || err != nil {
				w.logf("[Watching] get CPU core failed, CPU core: %v, error: %v", cpuCore, err)
				return
			}
			memoryLimit, err := w.getMemoryLimit()
			if memoryLimit == 0 || err != nil {
				w.logf("[Watching] get memory limit failed, memory limit: %v, error: %v", memoryLimit, err)
				return
			}
			cpu, mem, gNum, tNum, err := collect(cpuCore, memoryLimit)
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
			w.threadCheckAndShrink(tNum)
		}
	}
}

// goroutine start.
func (w *Watching) goroutineCheckAndDump(gNum int) {
	// get a copy instead of locking it
	grConfigs := w.config.GetGroupConfigs()

	if !grConfigs.Enable {
		return
	}

	if w.grCoolDownTime.After(time.Now()) {
		w.logf("[Watching] goroutine dump is in cooldown")
		return
	}

	if triggered := w.goroutineProfile(gNum, grConfigs); triggered {
		w.grCoolDownTime = time.Now().Add(w.config.CoolDown)
		w.grTriggerCount++
	}
}

func (w *Watching) goroutineProfile(gNum int, c groupConfigs) bool {
	if !matchRule(w.grNumStats, gNum, c.TriggerMin, c.TriggerAbs, c.TriggerDiff, c.GoroutineTriggerNumMax) {
		w.debugf(UniformLogFormat, "NODUMP", type2name[goroutine],
			c.TriggerMin, c.TriggerDiff, c.TriggerAbs,
			c.GoroutineTriggerNumMax, w.grNumStats.data, gNum)
		return false
	}

	var buf bytes.Buffer
	_ = pprof.Lookup("goroutine").WriteTo(&buf, int(w.config.DumpProfileType)) // nolint: errcheck
	w.writeGrProfileDataToFile(buf, c, goroutine, gNum)

	return true
}

// memory start.
func (w *Watching) memCheckAndDump(mem int) {
	memConfig := w.config.GetMemConfigs()

	if !memConfig.Enable {
		return
	}

	if w.memCoolDownTime.After(time.Now()) {
		w.logf("[Watching] mem dump is in cooldown")
		return
	}

	if triggered := w.memProfile(mem, memConfig); triggered {
		w.memCoolDownTime = time.Now().Add(w.config.CoolDown)
		w.memTriggerCount++
	}
}

func (w *Watching) memProfile(rss int, c typeConfig) bool {
	if !matchRule(w.memStats, rss, c.TriggerMin, c.TriggerAbs, c.TriggerDiff, NotSupportTypeMaxConfig) {
		// let user know why this should not dump
		w.debugf(UniformLogFormat, "NODUMP", type2name[mem],
			c.TriggerMin, c.TriggerDiff, c.TriggerAbs, NotSupportTypeMaxConfig,
			w.memStats.data, rss)

		return false
	}

	var buf bytes.Buffer
	_ = pprof.Lookup("heap").WriteTo(&buf, int(w.config.DumpProfileType)) // nolint: errcheck
	w.writeProfileDataToFile(buf, c, mem, rss, w.memStats, "")
	return true
}

func (w *Watching) threadCheckAndShrink(threadNum int) {
	shrink := w.config.ShrinkThrConfigs

	if shrink == nil || !shrink.Enable {
		return
	}

	if w.shrinkThrCoolDownTime.After(time.Now()) {
		return
	}

	if threadNum > shrink.Threshold {
		// 100x Delay time a cooldown time
		w.shrinkThrCoolDownTime = time.Now().Add(shrink.Delay * 100)

		w.logf("current thread number(%v) larger than threshold(%v), will start to shrink thread after %v", threadNum, shrink.Threshold, shrink.Delay)
		time.AfterFunc(shrink.Delay, func() {
			w.startShrinkThread(shrink)
		})
	}
}

// TODO: better only shrink the threads that are idle.
func (w *Watching) startShrinkThread(c *ShrinkThrConfigs) {
	curThreadNum := getThreadNum()
	n := curThreadNum - c.Threshold

	// check again after the timer triggered
	if c.Enable && n > 0 {
		w.shrinkThreadTriggerCount++
		w.logf("start to shrink %v threads, now: %v", n, curThreadNum)

		var wg sync.WaitGroup
		wg.Add(n)
		for i := 0; i < n; i++ {
			// avoid close too much thread in batch.
			time.Sleep(time.Millisecond * 100)

			go func() {
				defer wg.Done()
				runtime.LockOSThread()
			}()
		}
		wg.Wait()

		w.logf("finished shrink threads, now: %v", getThreadNum())
	}
}

// thread start.
func (w *Watching) threadCheckAndDump(threadNum int) {
	threadConfig := w.config.GetThreadConfigs()

	if !threadConfig.Enable {
		return
	}

	if w.threadCoolDownTime.After(time.Now()) {
		w.logf("[Watching] thread dump is in cooldown")
		return
	}

	if triggered := w.threadProfile(threadNum, threadConfig); triggered {
		w.threadCoolDownTime = time.Now().Add(w.config.CoolDown)
		w.threadTriggerCount++
	}
}

func (w *Watching) threadProfile(curThreadNum int, c typeConfig) bool {
	if !matchRule(w.threadStats, curThreadNum, c.TriggerMin, c.TriggerAbs, c.TriggerDiff, NotSupportTypeMaxConfig) {
		// let user know why this should not dump
		w.debugf(UniformLogFormat, "NODUMP", type2name[thread],
			c.TriggerMin, c.TriggerDiff, c.TriggerAbs, NotSupportTypeMaxConfig,
			w.threadStats.data, curThreadNum)

		return false
	}

	eventID := fmt.Sprintf("thr-%d", w.threadTriggerCount)

	var buf bytes.Buffer
	_ = pprof.Lookup("threadcreate").WriteTo(&buf, int(w.config.DumpProfileType)) // nolint: errcheck
	w.writeProfileDataToFile(buf, c, thread, curThreadNum, w.threadStats, eventID)

	buf.Reset()
	_ = pprof.Lookup("goroutine").WriteTo(&buf, int(w.config.DumpProfileType)) // nolint: errcheck
	w.writeProfileDataToFile(buf, c, goroutine, curThreadNum, w.threadStats, eventID)

	return true
}

// cpu start.
func (w *Watching) cpuCheckAndDump(cpu int) {
	cpuConfig := w.config.GetCPUConfigs()
	if !cpuConfig.Enable {
		return
	}

	if w.cpuCoolDownTime.After(time.Now()) {
		w.logf("[Watching] cpu dump is in cooldown")
		return
	}

	if triggered := w.cpuProfile(cpu, cpuConfig); triggered {
		w.cpuCoolDownTime = time.Now().Add(w.config.CoolDown)
		w.cpuTriggerCount++
	}
}

func (w *Watching) cpuProfile(curCPUUsage int, c typeConfig) bool {
	if !matchRule(w.cpuStats, curCPUUsage, c.TriggerMin, c.TriggerAbs, c.TriggerDiff, NotSupportTypeMaxConfig) {
		// let user know why this should not dump
		w.debugf(UniformLogFormat, "NODUMP", type2name[cpu],
			c.TriggerMin, c.TriggerDiff, c.TriggerAbs, NotSupportTypeMaxConfig,
			w.cpuStats.data, curCPUUsage)

		return false
	}

	binFileName := getBinaryFileName(w.config.DumpPath, cpu, "")

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
		c.TriggerMin, c.TriggerDiff, c.TriggerAbs, NotSupportTypeMaxConfig,
		w.cpuStats.data, curCPUUsage)

	return true
}

func (w *Watching) gcHeapCheckLoop() {
	for {
		// wait for the finalizer event
		<-w.finCh

		w.gcHeapCheckAndDump()
	}
}

func (w *Watching) gcHeapCheckAndDump() {
	gcHeapConfig := w.config.GetGcHeapConfigs()
	if !gcHeapConfig.Enable || atomic.LoadInt64(&w.stopped) == 1 {
		return
	}

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

	if triggered := w.gcHeapProfile(ratio, w.gcHeapTriggered, gcHeapConfig); triggered {
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

func (w *Watching) getCPUCore() (float64, error) {
	if w.config.cpuCore > 0 {
		return w.config.cpuCore, nil
	}

	if w.config.UseGoProcAsCPUCore {
		return float64(runtime.GOMAXPROCS(-1)), nil
	}

	if w.config.UseCGroup {
		return getCGroupCPUCore()
	}

	return float64(runtime.NumCPU()), nil
}

// gcHeapProfile will dump profile twice when triggered once.
// since the current memory profile will be merged after next GC cycle.
// And we assume the finalizer will be called before next GC cycle(it will be usually).
func (w *Watching) gcHeapProfile(gc int, force bool, c typeConfig) bool {
	if !force && !matchRule(w.gcHeapStats, gc, c.TriggerMin, c.TriggerAbs, c.TriggerDiff, NotSupportTypeMaxConfig) {
		// let user know why this should not dump
		w.debugf(UniformLogFormat, "NODUMP", type2name[gcHeap],
			c.TriggerMin, c.TriggerDiff, c.TriggerAbs, NotSupportTypeMaxConfig,
			w.gcHeapStats.data, gc)

		return false
	}

	// gcTriggerCount only increased after got both two profiles
	eventID := fmt.Sprintf("heap-%d", w.grTriggerCount)

	var buf bytes.Buffer
	_ = pprof.Lookup("heap").WriteTo(&buf, int(w.config.DumpProfileType)) // nolint: errcheck
	w.writeProfileDataToFile(buf, c, gcHeap, gc, w.gcHeapStats, eventID)

	return true
}

func (w *Watching) initEnvironment() {
	// choose whether the max memory is limited by cgroup
	if w.config.UseCGroup {
		w.logf("[Watching] use cgroup to limit memory")
	} else {
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

func (w *Watching) writeGrProfileDataToFile(data bytes.Buffer, config groupConfigs, dumpType configureType, currentStat int) {
	w.logf(UniformLogFormat, "pprof", type2name[dumpType],
		config.TriggerMin, config.TriggerDiff, config.TriggerAbs,
		config.GoroutineTriggerNumMax,
		w.grNumStats.data, currentStat)

	if err := writeFile(data, dumpType, w.config.DumpConfigs, ""); err != nil {
		w.logf("%s", err.Error())
	}
}

func (w *Watching) writeProfileDataToFile(data bytes.Buffer, opts typeConfig, dumpType configureType, currentStat int, ringStats ring, eventID string) {
	w.logf(UniformLogFormat, "pprof", type2name[dumpType],
		opts.TriggerMin, opts.TriggerDiff, opts.TriggerAbs,
		NotSupportTypeMaxConfig, ringStats, currentStat)

	if err := writeFile(data, dumpType, w.config.DumpConfigs, eventID); err != nil {
		w.logf("%s", err.Error())
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
	watching := &Watching{config: defaultConfig(), finCh: make(chan struct{}), stopped: 1}
	for _, opt := range opts {
		opt(watching)
	}
	return watching
}
