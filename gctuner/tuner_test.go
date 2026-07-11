package gctuner

import (
	"os"
	"os/exec"
	"runtime"
	"runtime/debug"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

var testHeap []byte

const gogcOffHelperEnv = "GKIT_GCTUNER_GOGC_OFF_HELPER"

func TestTuningZeroRestoresGOGCOff(t *testing.T) {
	if os.Getenv(gogcOffHelperEnv) == "1" {
		Tuning(0)
		got := debug.SetGCPercent(-1)
		debug.SetGCPercent(got)
		if got != -1 {
			t.Fatalf("runtime GC percent after Tuning(0) = %d, want -1 for GOGC=off", got)
		}
		return
	}

	cmd := exec.Command(os.Args[0], "-test.run=^TestTuningZeroRestoresGOGCOff$", "-test.v")
	env := make([]string, 0, len(os.Environ())+2)
	for _, value := range os.Environ() {
		if strings.HasPrefix(value, "GOGC=") || strings.HasPrefix(value, gogcOffHelperEnv+"=") {
			continue
		}
		env = append(env, value)
	}
	cmd.Env = append(env, "GOGC=off", gogcOffHelperEnv+"=1")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("GOGC=off helper failed: %v\n%s", err, output)
	}
}

func TestTuningZeroRestoresRuntimeGCPercent(t *testing.T) {
	originalGCPercent := debug.SetGCPercent(configuredRuntimeGCPercent)
	t.Cleanup(func() {
		tuningMu.Lock()
		if globalTuner != nil {
			globalTuner.stop()
			globalTuner = nil
		}
		tuningMu.Unlock()
		debug.SetGCPercent(originalGCPercent)
	})

	Tuning(50)
	tuningMu.Lock()
	if globalTuner == nil {
		tuningMu.Unlock()
		t.Fatal("Tuning(50) did not install a tuner")
	}
	globalTuner.setGCPercent(minGCPercent)
	tuningMu.Unlock()

	Tuning(0)

	gotRuntimeGCPercent := debug.SetGCPercent(configuredRuntimeGCPercent)
	debug.SetGCPercent(gotRuntimeGCPercent)
	assert.Equal(t, configuredRuntimeGCPercent, gotRuntimeGCPercent)
	assert.Equal(t, defaultGCPercent, GetGCPercent())

	tuningMu.Lock()
	defer tuningMu.Unlock()
	assert.Nil(t, globalTuner)
}

func TestTunerStopWaitsForInFlightTuning(t *testing.T) {
	tn := &tuner{finalizer: &finalizer{}}
	tn.tuningMu.Lock()

	stopReturned := make(chan struct{})
	go func() {
		tn.stop()
		close(stopReturned)
	}()

	deadline := time.Now().Add(time.Second)
	for atomic.LoadInt32(&tn.finalizer.stopped) == 0 && time.Now().Before(deadline) {
		runtime.Gosched()
	}
	if atomic.LoadInt32(&tn.finalizer.stopped) == 0 {
		tn.tuningMu.Unlock()
		t.Fatal("stop did not mark the finalizer as stopped")
	}

	select {
	case <-stopReturned:
		tn.tuningMu.Unlock()
		t.Fatal("stop returned while a tuning callback was still in flight")
	case <-time.After(100 * time.Millisecond):
	}

	tn.tuningMu.Unlock()
	select {
	case <-stopReturned:
	case <-time.After(time.Second):
		t.Fatal("stop did not return after the tuning callback completed")
	}
}

func TestTuner(t *testing.T) {
	is := assert.New(t)
	memLimit := uint64(100 * 1024 * 1024) // 100 MB
	threshold := memLimit / 2
	originalGCPercent := debug.SetGCPercent(int(defaultGCPercent))
	tn := newTuner(threshold)
	t.Cleanup(func() {
		tn.stop()
		testHeap = nil
		debug.SetGCPercent(originalGCPercent)
	})
	currentGCPercent := tn.getGCPercent()
	is.Equal(tn.threshold, threshold)
	is.Equal(defaultGCPercent, currentGCPercent)

	// wait for tuner set gcPercent to maxGCPercent
	t.Logf("old gc percent before gc: %d", tn.getGCPercent())
	for tn.getGCPercent() != maxGCPercent {
		runtime.GC()
		t.Logf("new gc percent after gc: %d", tn.getGCPercent())
	}

	// 1/4 threshold
	testHeap = make([]byte, threshold/4)
	// wait for tuner set gcPercent to ~= 300
	t.Logf("old gc percent before gc: %d", tn.getGCPercent())
	for tn.getGCPercent() == maxGCPercent {
		runtime.GC()
		t.Logf("new gc percent after gc: %d", tn.getGCPercent())
	}
	currentGCPercent = tn.getGCPercent()
	is.GreaterOrEqual(currentGCPercent, uint32(250))
	is.LessOrEqual(currentGCPercent, uint32(300))

	// 1/2 threshold
	testHeap = make([]byte, threshold/2)
	// wait for tuner set gcPercent to ~= 100
	t.Logf("old gc percent before gc: %d", tn.getGCPercent())
	for tn.getGCPercent() == currentGCPercent {
		runtime.GC()
		t.Logf("new gc percent after gc: %d", tn.getGCPercent())
	}
	currentGCPercent = tn.getGCPercent()
	is.GreaterOrEqual(currentGCPercent, uint32(50))
	is.LessOrEqual(currentGCPercent, uint32(100))

	// 3/4 threshold
	testHeap = make([]byte, threshold/4*3)
	// wait for tuner set gcPercent to minGCPercent
	t.Logf("old gc percent before gc: %d", tn.getGCPercent())
	for tn.getGCPercent() != minGCPercent {
		runtime.GC()
		t.Logf("new gc percent after gc: %d", tn.getGCPercent())
	}
	is.Equal(minGCPercent, tn.getGCPercent())

	// out of threshold
	testHeap = make([]byte, threshold+1024)
	t.Logf("old gc percent before gc: %d", tn.getGCPercent())
	runtime.GC()
	for i := 0; i < 8; i++ {
		runtime.GC()
		is.Equal(minGCPercent, tn.getGCPercent())
	}

	// no heap
	testHeap = nil
	// wait for tuner set gcPercent to maxGCPercent
	t.Logf("old gc percent before gc: %d", tn.getGCPercent())
	for tn.getGCPercent() != maxGCPercent {
		runtime.GC()
		t.Logf("new gc percent after gc: %d", tn.getGCPercent())
	}
}

func TestCalcGCPercent(t *testing.T) {
	is := assert.New(t)
	const gb = 1024 * 1024 * 1024
	// use default value when invalid params
	is.Equal(defaultGCPercent, calcGCPercent(0, 0))
	is.Equal(defaultGCPercent, calcGCPercent(0, 1))
	is.Equal(defaultGCPercent, calcGCPercent(1, 0))

	is.Equal(maxGCPercent, calcGCPercent(1, 3*gb))
	is.Equal(maxGCPercent, calcGCPercent(gb/10, 4*gb))
	is.Equal(maxGCPercent, calcGCPercent(gb/2, 4*gb))
	is.Equal(uint32(300), calcGCPercent(1*gb, 4*gb))
	is.Equal(uint32(166), calcGCPercent(1.5*gb, 4*gb))
	is.Equal(uint32(100), calcGCPercent(2*gb, 4*gb))
	is.Equal(minGCPercent, calcGCPercent(3*gb, 4*gb))
	is.Equal(minGCPercent, calcGCPercent(4*gb, 4*gb))
	is.Equal(minGCPercent, calcGCPercent(5*gb, 4*gb))
}
