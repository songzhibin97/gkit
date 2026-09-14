package gctuner

import (
	"sync"
	"testing"
)

func TestCalcGCPercentConcurrentBounds(t *testing.T) {
	oldMin, oldMax := GetMinGCPercent(), GetMaxGCPercent()
	t.Cleanup(func() { SetMinGCPercent(oldMin); SetMaxGCPercent(oldMax) })
	var wg sync.WaitGroup
	start := make(chan struct{})
	wg.Add(2)
	go func() {
		defer wg.Done()
		<-start
		for i := 0; i < 1024; i++ {
			SetMinGCPercent(uint32(50 + i%2))
			SetMaxGCPercent(uint32(500 + i%2))
		}
	}()
	go func() {
		defer wg.Done()
		<-start
		for i := 0; i < 1024; i++ {
			for _, v := range [][2]uint64{{100, 99}, {100, 101}, {100, 200}, {1, 10000}, {0, 0}} {
				got := calcGCPercent(v[0], v[1])
				if got < 50 || got > 501 {
					t.Errorf("GC percent %d outside configured bounds", got)
				}
			}
		}
	}()
	close(start)
	wg.Wait()
}
