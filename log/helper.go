package log

import (
	"fmt"
)

var Terminal = &terminal{}

type terminal struct{}

func (t *terminal) Print(i ...interface{}) {
	fmt.Println(i)
}

type Helper struct {
	Logger
	lever Lever
}

// NewHelper 实例化函数
func NewHelper(log Logger, lever Lever) *Helper {
	return &Helper{log, lever}
}

func (h *Helper) Debug(kv ...interface{}) {
	if h.lever.Allow(LevelDebug) {
		Debug(h.Logger).Print(kv...)
	}
}

func (h *Helper) Info(kv ...interface{}) {
	if h.lever.Allow(LevelInfo) {
		Info(h.Logger).Print(kv...)
	}
}

func (h *Helper) Warn(kv ...interface{}) {
	if h.lever.Allow(LevelWarn) {
		Warn(h.Logger).Print(kv...)
	}
}

func (h *Helper) Error(kv ...interface{}) {
	if h.lever.Allow(LevelError) {
		Error(h.Logger).Print(kv...)
	}
}

func (h *Helper) Debugf(format string, kv ...interface{}) {
	if h.lever.Allow(LevelDebug) {
		Debug(h.Logger).Print(fmt.Sprintf(format, kv...))
	}
}

func (h *Helper) Infof(format string, kv ...interface{}) {
	if h.lever.Allow(LevelInfo) {
		Info(h.Logger).Print(fmt.Sprintf(format, kv...))
	}
}

func (h *Helper) Warnf(format string, kv ...interface{}) {
	if h.lever.Allow(LevelWarn) {
		Warn(h.Logger).Print(fmt.Sprintf(format, kv...))
	}
}

func (h *Helper) Errorf(format string, kv ...interface{}) {
	if h.lever.Allow(LevelError) {
		Error(h.Logger).Print(fmt.Sprintf(format, kv...))
	}
}
