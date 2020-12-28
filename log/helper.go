package log

import "fmt"

type Helper struct {
	Logger
	lever Lever
}

// NewHelper: 实例化函数
func NewHelper(log Logger, lever Lever) *Helper {
	return &Helper{log, lever}
}

// Debug
func (h *Helper) Debug(kv ...interface{}) {
	if h.lever.Allow(LevelDebug) {
		Debug(h.Logger).Print(kv...)
	}
}

// Info
func (h *Helper) Info(kv ...interface{}) {
	if h.lever.Allow(LevelInfo) {
		Info(h.Logger).Print(kv...)
	}
}

// Warn
func (h *Helper) Warn(kv ...interface{}) {
	if h.lever.Allow(LevelWarn) {
		Warn(h.Logger).Print(kv...)
	}
}

// Error
func (h *Helper) Error(kv ...interface{}) {
	if h.lever.Allow(LevelError) {
		Error(h.Logger).Print(kv...)
	}
}

// Debugf
func (h *Helper) Debugf(format string, kv ...interface{}) {
	if h.lever.Allow(LevelDebug) {
		Debug(h.Logger).Print(fmt.Sprintf(format, kv...))
	}
}

// Infof
func (h *Helper) Infof(format string, kv ...interface{}) {
	if h.lever.Allow(LevelInfo) {
		Info(h.Logger).Print(fmt.Sprintf(format, kv...))
	}
}

// Warnf
func (h *Helper) Warnf(format string, kv ...interface{}) {
	if h.lever.Allow(LevelWarn) {
		Warn(h.Logger).Print(fmt.Sprintf(format, kv...))
	}
}

// Errorf
func (h *Helper) Errorf(format string, kv ...interface{}) {
	if h.lever.Allow(LevelError) {
		Error(h.Logger).Print(fmt.Sprintf(format, kv...))
	}
}
