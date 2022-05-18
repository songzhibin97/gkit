package wyhash

import (
	"fmt"
	"runtime"
	"testing"
)

func BenchmarkWyhash(b *testing.B) {
	sizes := []int{17, 21, 24, 29, 32,
		33, 64, 69, 96, 97, 128, 129, 240, 241,
		512, 1024, 100 * 1024,
	}

	for size := 0; size <= 16; size++ {
		b.Run(fmt.Sprintf("%d", size), func(b *testing.B) {
			b.SetBytes(int64(size))
			var (
				x    uint64
				data = string(make([]byte, size))
			)
			b.ReportAllocs()
			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				x = Sum64String(data)
			}
			runtime.KeepAlive(x)
		})
	}

	for _, size := range sizes {
		b.Run(fmt.Sprintf("%d", size), func(b *testing.B) {
			b.SetBytes(int64(size))
			var x uint64
			data := string(make([]byte, size))
			b.ReportAllocs()
			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				x = Sum64String(data)
			}
			runtime.KeepAlive(x)
		})
	}
}
