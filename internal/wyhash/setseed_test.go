package wyhash

import (
	"fmt"
	"testing"
)

func TestDigestSetSeedMatchesOneShot(t *testing.T) {
	seeds := []uint64{0, 1, DefaultSeed, 0x0123456789abcdef}
	lengths := []int{64, 65, 128, 129}
	writes := []struct {
		name   string
		chunks []int
	}{
		{name: "whole"},
		{name: "chunked", chunks: []int{1, 7, 31, 3, 64}},
	}

	for _, seed := range seeds {
		for _, length := range lengths {
			data := makeSeedTestData(length)
			for _, write := range writes {
				t.Run(fmt.Sprintf("%s/seed=%016x/length=%d", write.name, seed, length), func(t *testing.T) {
					digest := New(seed ^ 0xffffffffffffffff)
					digest.SetSeed(seed)
					writeSeedTestData(t, digest, data, write.chunks)

					if got, want := digest.Sum64(), Sum64WithSeed(data, seed); got != want {
						t.Fatalf("Sum64() = %d, want %d", got, want)
					}
				})
			}
		}
	}
}

func TestDigestResetRestoresInitialSeedAfterSetSeed(t *testing.T) {
	const (
		initialSeed = uint64(0x1020304050607080)
		activeSeed  = uint64(0x8877665544332211)
	)
	data := makeSeedTestData(129)

	digest := New(initialSeed)
	digest.SetSeed(activeSeed)
	writeSeedTestData(t, digest, data, []int{17, 48, 1, 63})
	if got, want := digest.Sum64(), Sum64WithSeed(data, activeSeed); got != want {
		t.Fatalf("Sum64() before Reset = %d, want %d", got, want)
	}

	digest.Reset()
	if got := digest.Seed(); got != initialSeed {
		t.Fatalf("Seed() after Reset = %d, want initial seed %d", got, initialSeed)
	}
	if got := digest.InitSeed(); got != initialSeed {
		t.Fatalf("InitSeed() after Reset = %d, want %d", got, initialSeed)
	}
	writeSeedTestData(t, digest, data, []int{64, 1, 64})
	if got, want := digest.Sum64(), Sum64WithSeed(data, initialSeed); got != want {
		t.Fatalf("Sum64() after Reset = %d, want %d", got, want)
	}
}

func makeSeedTestData(length int) []byte {
	data := make([]byte, length)
	for i := range data {
		data[i] = byte(i*37 + 11)
	}
	return data
}

func writeSeedTestData(t *testing.T, digest *Digest, data []byte, chunks []int) {
	t.Helper()
	if chunks == nil {
		if n, err := digest.Write(data); err != nil || n != len(data) {
			t.Fatalf("Write() = (%d, %v), want (%d, nil)", n, err, len(data))
		}
		return
	}

	for offset, chunk := 0, 0; offset < len(data); chunk++ {
		size := chunks[chunk%len(chunks)]
		if size > len(data)-offset {
			size = len(data) - offset
		}
		n, err := digest.Write(data[offset : offset+size])
		if err != nil || n != size {
			t.Fatalf("Write() = (%d, %v), want (%d, nil)", n, err, size)
		}
		offset += size
	}
}
