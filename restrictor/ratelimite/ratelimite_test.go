package ratelimite

import (
	"context"
	"testing"
	"time"

	"github.com/juju/ratelimit"
)

func TestRateLimit(t *testing.T) {
	// 创建指定填充速率和容量大小的令牌桶
	// func NewBucket(fillInterval time.Duration, capacity int64) *Bucket
	// 创建指定填充速率、容量大小和每次填充的令牌数的令牌桶
	// func NewBucketWithQuantum(fillInterval time.Duration, capacity, quantum int64) *Bucket
	// 创建填充速度为指定速率和容量大小的令牌桶
	// NewBucketWithRate(0.1, 200) 表示每秒填充20个令牌
	// func NewBucketWithRate(rate float64, capacity int64) *Bucket

	bucket := ratelimit.NewBucket(time.Second/2, 4)
	af, wf := NewRateLimit(bucket)
	// 暂停3秒,等待桶满
	time.Sleep(3 * time.Second)
	for i := 0; i < 10; i++ {
		t.Log("i:", i, af.Allow())
	}

	// 暂停3秒,等待桶满
	time.Sleep(3 * time.Second)
	for i := 0; i < 5; i++ {
		t.Log("i:", i, af.AllowN(time.Now(), 2))
	}

	// 暂停3秒,等待桶满
	time.Sleep(3 * time.Second)
	for i := 0; i < 10; i++ {
		func(i int) {
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			t.Log("i:", i, wf.Wait(ctx))
		}(i)
	}
	// 暂停3秒,等待桶满
	time.Sleep(3 * time.Second)
	for i := 0; i < 5; i++ {
		func(i int) {
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			t.Log("i:", i, wf.WaitN(ctx, 2))
		}(i)
	}
}
