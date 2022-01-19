package log

import (
	"context"
	"log"
)

// DefaultLogger is default logger.
var DefaultLogger Logger = NewStdLogger(log.Writer())

// Logger 操作日志对外的接口
// 实现该接口需要保证它是并发安全的
type Logger interface {
	Log(lever Lever, kv ...interface{}) error
}

type logger struct {
	// log: 输出对象
	logs []Logger
	// prefix: kv键值对 构建前缀
	prefix []interface{}
	// hasValuer 判断是否包含 Valuer 类型
	hasValuer bool
	// ctx 上下文
	ctx context.Context
}

// Log 实现 Logger 接口
func (l *logger) Log(lever Lever, kvs ...interface{}) error {
	nKvs := make([]interface{}, 0, len(l.prefix)+len(kvs))
	nKvs = append(nKvs, l.prefix...)
	if l.hasValuer && len(kvs) > 0 {
		// 特殊处理
		bindValues(l.ctx, nKvs)
	}
	if len(kvs) > 0 {
		nKvs = append(nKvs, kvs...)
	}

	for _, log := range l.logs {
		if err := log.Log(lever, nKvs...); err != nil {
			return err
		}
	}
	return nil
}

// With 生成 Logger
func With(l Logger, kvs ...interface{}) Logger {
	if c, ok := l.(*logger); ok {
		nKvs := make([]interface{}, 0, len(c.prefix)+len(kvs))
		nKvs = append(kvs, kvs...)
		nKvs = append(kvs, c.prefix...)
		return &logger{
			logs:      c.logs,
			prefix:    kvs,
			hasValuer: containsValuer(nKvs),
			ctx:       c.ctx,
		}
	}
	return &logger{logs: []Logger{l}, prefix: kvs, hasValuer: containsValuer(kvs)}
}

// WithContext 设置 Logger 上下文
func WithContext(ctx context.Context, l Logger) Logger {
	if c, ok := l.(*logger); ok {
		return &logger{
			logs:      c.logs,
			prefix:    c.prefix,
			hasValuer: c.hasValuer,
			ctx:       ctx,
		}
	}
	return &logger{logs: []Logger{l}, ctx: ctx}
}

// WithLogs 包装多个 Logger
func WithLogs(logs ...Logger) Logger {
	return &logger{logs: logs}
}
