//go:build ignore
// +build ignore

package main

import "flag"

var (
	avx = flag.Bool("avx2", false, "avx2")
	sse = flag.Bool("sse2", false, "sse2")
)

func main() {
	flag.Parse()

	if *avx {
		AVX2()
	} else if *sse {
		SSE2()
	}
}
