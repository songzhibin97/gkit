#!/bin/sh
set -eu

cd "$(dirname "$0")"
go run -mod=readonly sse.go avx.go gen.go -avx2 -out ../../avx2_amd64.s
go run -mod=readonly sse.go avx.go gen.go -sse2 -out ../../sse2_amd64.s
