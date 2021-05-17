package restrictor

import (
	"context"
	"time"
)

type Allow interface {
	// Allow AllowN(time.Now(),1)
	Allow() bool
	// AllowN 截止到某一时刻，目前桶中数目是否至少为 n 个，满足则返回 true，同时从桶中消费 n 个 token
	AllowN(now time.Time, n int) bool
}

type Wait interface {
	// Wait WaitN(ctx,1)
	Wait(ctx context.Context) error
	// WaitN 如果此时桶内 Token 数组不足 (小于 N)，那么 Wait 方法将会阻塞一段时间，直至 Token 满足条件。如果充足则直接返回
	// 我们可以设置 context 的 Deadline 或者 Timeout，来决定此次 Wait 的最长时间。
	WaitN(ctx context.Context, n int) error
}

// AllowFunc 实现 Allow 接口
type AllowFunc func(now time.Time, n int) bool

func (a AllowFunc) Allow() bool {
	return a.AllowN(time.Now(), 1)
}

func (a AllowFunc) AllowN(now time.Time, n int) bool {
	return a(now, n)
}

// WaitFunc 实现 Wait 接口
type WaitFunc func(ctx context.Context, n int) error

func (w WaitFunc) Wait(ctx context.Context) error {
	return w.WaitN(ctx, 1)
}

func (w WaitFunc) WaitN(ctx context.Context, n int) error {
	return w(ctx, n)
}
