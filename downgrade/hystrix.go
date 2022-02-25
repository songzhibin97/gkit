package downgrade

import (
	"context"

	"github.com/afex/hystrix-go/hystrix"
)

type Hystrix struct{}

func (h *Hystrix) Do(name string, run RunFunc, fallback FallbackFunc) error {
	return hystrix.Do(name, run, fallback)
}

func (h *Hystrix) DoC(ctx context.Context, name string, run RunFuncC, fallback FallbackFuncC) error {
	return hystrix.DoC(ctx, name, run, fallback)
}

func (h *Hystrix) Go(name string, run RunFunc, fallback FallbackFunc) chan error {
	return hystrix.Go(name, run, fallback)
}

func (h *Hystrix) GoC(ctx context.Context, name string, run RunFuncC, fallback FallbackFuncC) chan error {
	return hystrix.GoC(ctx, name, run, fallback)
}

func (h *Hystrix) ConfigureCommand(name string, config hystrix.CommandConfig) {
	hystrix.ConfigureCommand(name, config)
}

// NewFuse 实例化方法
func NewFuse() Fuse {
	return &Hystrix{}
}
