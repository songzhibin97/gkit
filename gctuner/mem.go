package gctuner

import (
	"runtime"
)

var memStats runtime.MemStats

func readMemoryInuse() uint64 {
	runtime.ReadMemStats(&memStats)
	return memStats.HeapInuse
}
