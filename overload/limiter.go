package overload

import (
	"context"
)

// Op 操作类型
type Op int

const (
	Success Op = iota
	Ignore
	Drop
)

type allowOptions struct{}

// AllowOption AllowOptions allow options.
type AllowOption interface {
	Apply(*allowOptions)
}

// DoneInfo 完成信息
type DoneInfo struct {
	Err error
	Op  Op
}

// DefaultAllowOpts 返回默认选项
func DefaultAllowOpts() allowOptions {
	return allowOptions{}
}

// Limiter 限制接口
type Limiter interface {
	Allow(ctx context.Context, opts ...AllowOption) (func(info DoneInfo), error)
}
