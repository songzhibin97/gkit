package log

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"sync"
)

var _ Logger = (*stdLogger)(nil)

type stdLogger struct {
	log      *log.Logger
	pool     *sync.Pool
	minLevel Lever
}

// NewStdLogger new a logger with writer. The minimum level filter is
// LevelDebug (i.e. everything is emitted).
func NewStdLogger(w io.Writer) Logger {
	return NewStdLoggerWithLevel(w, LevelDebug)
}

// NewStdLoggerWithLevel new a stdlib-backed logger that drops messages
// below `min`. Previously stdLogger.Log emitted every level regardless of
// the supplied `level` argument because the matching `Lever.Allow` filter
// was never invoked.
func NewStdLoggerWithLevel(w io.Writer, min Lever) Logger {
	return &stdLogger{
		log: log.New(w, "", 0),
		pool: &sync.Pool{
			New: func() interface{} {
				return new(bytes.Buffer)
			},
		},
		minLevel: min,
	}
}

// Log print the kv pairs log.
func (l *stdLogger) Log(level Lever, kvs ...interface{}) error {
	if !l.minLevel.Allow(level) {
		return nil
	}
	if len(kvs) == 0 {
		return nil
	}
	if len(kvs)%2 != 0 {
		kvs = append(kvs, "")
	}
	buf := l.pool.Get().(*bytes.Buffer)
	buf.WriteString(level.String())
	for i := 0; i < len(kvs); i += 2 {
		fmt.Fprintf(buf, " %s=%v", kvs[i], kvs[i+1])
	}
	_ = l.log.Output(4, buf.String())
	buf.Reset()
	l.pool.Put(buf)
	return nil
}
