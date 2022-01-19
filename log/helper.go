package log

import (
	"context"
	"fmt"
)

type Helper struct {
	Logger
}

// NewHelper 实例化函数
func NewHelper(log Logger) *Helper {
	return &Helper{log}
}

// WithContext 调用 logger WithContext 刷新ctx
func (h *Helper) WithContext(ctx context.Context) *Helper {
	return &Helper{Logger: WithContext(ctx, h.Logger)}
}

// Log .
func (h *Helper) Log(lever Lever, kvs ...interface{}) {
	_ = h.Logger.Log(lever, kvs...)
}

// Debug .
func (h *Helper) Debug(a ...interface{}) {
	_ = h.Logger.Log(LevelDebug, "message", fmt.Sprint(a...))
}

// Debugf .
func (h *Helper) Debugf(format string, a ...interface{}) {
	h.Logger.Log(LevelDebug, "message", fmt.Sprintf(format, a...))
}

// Debugw .
func (h *Helper) Debugw(keyvals ...interface{}) {
	h.Logger.Log(LevelDebug, keyvals...)
}

// Info .
func (h *Helper) Info(a ...interface{}) {
	h.Logger.Log(LevelInfo, "message", fmt.Sprint(a...))
}

// Infof .
func (h *Helper) Infof(format string, a ...interface{}) {
	h.Logger.Log(LevelInfo, "message", fmt.Sprintf(format, a...))
}

// Infow .
func (h *Helper) Infow(keyvals ...interface{}) {
	h.Logger.Log(LevelInfo, keyvals...)
}

// Warn .
func (h *Helper) Warn(a ...interface{}) {
	h.Logger.Log(LevelWarn, "message", fmt.Sprint(a...))
}

// Warnf .
func (h *Helper) Warnf(format string, a ...interface{}) {
	h.Logger.Log(LevelWarn, "message", fmt.Sprintf(format, a...))
}

// Warnw .
func (h *Helper) Warnw(keyvals ...interface{}) {
	h.Logger.Log(LevelWarn, keyvals...)
}

// Error .
func (h *Helper) Error(a ...interface{}) {
	h.Logger.Log(LevelError, "message", fmt.Sprint(a...))
}

// Errorf .
func (h *Helper) Errorf(format string, a ...interface{}) {
	h.Logger.Log(LevelError, "message", fmt.Sprintf(format, a...))
}

// Errorw .
func (h *Helper) Errorw(keyvals ...interface{}) {
	h.Logger.Log(LevelError, keyvals...)
}
