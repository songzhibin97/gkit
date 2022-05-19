package fastrand

import (
	"math/rand"
	"testing"
)

func TestAll(t *testing.T) {
	_ = Uint32()

	p := make([]byte, 1000)
	n, err := Read(p)
	if n != len(p) || err != nil || (p[0] == 0 && p[1] == 0 && p[2] == 0) {
		t.Fatal()
	}

	a := Perm(100)
	for i := range a {
		var find bool
		for _, v := range a {
			if v == i {
				find = true
			}
		}
		if !find {
			t.Fatal()
		}
	}

	Shuffle(len(a), func(i, j int) {
		a[i], a[j] = a[j], a[i]
	})
	for i := range a {
		var find bool
		for _, v := range a {
			if v == i {
				find = true
			}
		}
		if !find {
			t.Fatal()
		}
	}
}

func BenchmarkSingleCore(b *testing.B) {
	b.Run("math/rand-Uint32()", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = rand.Uint32()
		}
	})
	b.Run("fast-rand-Uint32()", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = Uint32()
		}
	})

	b.Run("math/rand-Uint64()", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = rand.Uint64()
		}
	})
	b.Run("fast-rand-Uint64()", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = Uint64()
		}
	})

	b.Run("math/rand-Read1000", func(b *testing.B) {
		p := make([]byte, 1000)
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			rand.Read(p)
		}
	})
	b.Run("fast-rand-Read1000", func(b *testing.B) {
		p := make([]byte, 1000)
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			Read(p)
		}
	})

}

func BenchmarkMultipleCore(b *testing.B) {
	b.Run("math/rand-Uint32()", func(b *testing.B) {
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				_ = rand.Uint32()
			}
		})
	})
	b.Run("fast-rand-Uint32()", func(b *testing.B) {
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				_ = Uint32()
			}
		})
	})

	b.Run("math/rand-Uint64()", func(b *testing.B) {
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				_ = rand.Uint64()
			}
		})
	})
	b.Run("fast-rand-Uint64()", func(b *testing.B) {
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				_ = Uint64()
			}
		})
	})

	b.Run("math/rand-Read1000", func(b *testing.B) {
		p := make([]byte, 1000)
		b.ResetTimer()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				rand.Read(p)
			}
		})
	})
	b.Run("fast-rand-Read1000", func(b *testing.B) {
		p := make([]byte, 1000)
		b.ResetTimer()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				Read(p)
			}
		})
	})
}
