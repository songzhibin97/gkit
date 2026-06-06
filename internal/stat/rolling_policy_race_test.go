package stat

import (
	"sync"
	"testing"
	"time"
)

// TestRollingCounter_TimespanAddNoRace covers the data race where
// rollingCounter.Timespan() read RollingPolicy.lastAppendTime without the lock
// while Add() mutated it under the write lock. Run under -race; before the fix
// (Timespan taking the read lock) this trips the race detector.
func TestRollingCounter_TimespanAddNoRace(t *testing.T) {
	rc := NewRollingCounter(10, time.Millisecond)

	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 2000; j++ {
				rc.Add(1)
			}
		}()
	}
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 2000; j++ {
				_ = rc.Timespan()
			}
		}()
	}
	wg.Wait()
}
