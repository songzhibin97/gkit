package gctuner

import (
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
)

var testHeap []byte

func TestTuner(t *testing.T) {
	is := assert.New(t)
	memLimit := uint64(100 * 1024 * 1024) // 100 MB
	threshold := memLimit / 2
	tn := newTuner(threshold)
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
