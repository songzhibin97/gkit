package log

// Logger 操作日志对外的接口
// 实现该接口需要保证它是并发安全的
type Logger interface {
	Print(kv ...interface{})
}

type log struct {
	// log: 输出对象
	log Logger
	// kv: kv键值对
	kv []interface{}
}

// Print 实现接口
func (l *log) Print(kv ...interface{}) {
	l.log.Print(append(l.kv, kv...)...)
}

// newLogger 实例化 log 对象
func newLogger(l Logger, kv ...interface{}) *log {
	return &log{
		log: l,
		kv:  kv,
	}
}

// With .
func With(l Logger, kv ...interface{}) Logger {
	return &log{log: l, kv: kv}
}

// Debug .
func Debug(l Logger) Logger {
	return With(l, LevelDebug)
}

// Info .
func Info(l Logger) Logger {
	return With(l, LevelInfo)
}

// Warn .
func Warn(l Logger) Logger {
	return With(l, LevelWarn)
}

// Error .
func Error(l Logger) Logger {
	return With(l, LevelError)
}
