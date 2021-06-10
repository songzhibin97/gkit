package log

import (
	"context"
	"runtime"
	"strconv"
	"strings"
	"time"

	"go.opentelemetry.io/otel/trace"
)

var (
	// DefaultCaller is a Valuer that returns the file and line.
	DefaultCaller = Caller(3)

	// DefaultTimestamp is a Valuer that returns the current wallclock time.
	DefaultTimestamp = Timestamp(time.RFC3339)
)

type (
	// Valuer log返回携带值
	Valuer func(ctx context.Context) interface{}
)

// Value 尝试调用Valuer接口返回
func Value(ctx context.Context, value interface{}) interface{} {
	if v, ok := value.(Valuer); ok {
		return v(ctx)
	}
	return value
}

// Caller 返回调用方的堆信息
func Caller(depth int) Valuer {
	return func(ctx context.Context) interface{} {
		_, file, line, _ := runtime.Caller(depth)
		if strings.LastIndex(file, "gkit/log") > 0 {
			_, file, line, _ = runtime.Caller(depth + 1)
		}
		idx := strings.LastIndexByte(file, '/')
		return file[idx+1:] + ":" + strconv.Itoa(line)
	}
}

// Timestamp 返回指定layout的时间戳 Valuer
func Timestamp(layout string) Valuer {
	return func(context.Context) interface{} {
		return time.Now().Format(layout)
	}
}

// TraceID 返回链路追踪使用的tranceID Valuer
// TraceID 来将一个请求在各个服务器上的调用日志串联起来
func TraceID() Valuer {
	return func(ctx context.Context) interface{} {
		if span := trace.SpanContextFromContext(ctx); span.HasTraceID() {
			return span.TraceID().String()
		}
		return ""
	}
}

// SpanID 返回链路定位的 SpanID Valuer
// SpanID 代表本次调用在整个调用链路树中的位置
func SpanID() Valuer {
	return func(ctx context.Context) interface{} {
		if span := trace.SpanContextFromContext(ctx); span.HasSpanID() {
			return span.SpanID().String()
		}
		return ""
	}
}

// bindValues 判断 kvs 的 v 是否是 Valuer对象 如果是的话将ctx传入保存
func bindValues(ctx context.Context, kvs []interface{}) {
	for i := 1; i < len(kvs); i += 2 {
		if v, ok := kvs[i].(Valuer); ok {
			kvs[i] = v(ctx)
		}
	}
}

// containsValuer 判断 kvs 中 v是否是 Valuer 如果有立即返回true
func containsValuer(kvs []interface{}) bool {
	for i := 1; i < len(kvs); i += 2 {
		if _, ok := kvs[i].(Valuer); ok {
			return true
		}
	}
	return false
}
