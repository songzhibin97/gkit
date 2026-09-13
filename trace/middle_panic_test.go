package trace

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/songzhibin97/gkit/middleware"
	"github.com/songzhibin97/gkit/options"
	"go.opentelemetry.io/otel/codes"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	oteltrace "go.opentelemetry.io/otel/trace"
)

// A completion flag detects panic(nil) even on Go 1.20, where recover returns
// nil for it. Comparison with a direct panic also covers newer Go semantics.
func captureEndpointPanic(call func()) (returned bool, value interface{}) {
	defer func() { value = recover() }()
	call()
	return true, nil
}

func TestMiddlewareRecordsCompletionAndPropagatesPanics(t *testing.T) {
	for _, side := range []struct {
		name      string
		wrap      func(...options.Option) middleware.MiddleWare
		transport func(context.Context, Transporter) context.Context
		kind      oteltrace.SpanKind
	}{
		{"server", WithServer, NewServerTransportContext, oteltrace.SpanKindServer},
		{"client", WithClient, NewClientTransportContext, oteltrace.SpanKindClient},
	} {
		for _, withTransport := range []bool{true, false} {
			for _, mode := range []string{"success", "error", "panic", "panic nil"} {
				t.Run(fmt.Sprintf("%s/transport=%t/%s", side.name, withTransport, mode), func(t *testing.T) {
					recorder := tracetest.NewSpanRecorder()
					provider := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(recorder))
					t.Cleanup(func() {
						if err := provider.Shutdown(context.Background()); err != nil {
							t.Error(err)
						}
					})
					ctx := context.Background()
					if withTransport {
						ctx = side.transport(ctx, &_Transport{kind: KindGRPC, operation: "/test.Service/Method", header: headerCarrier{}})
					}
					request := &struct{ secret string }{"request-secret-marker"}
					response := &struct{ value string }{"response"}
					endpointErr := errors.New("endpoint error")
					panicValue := &struct{ secret string }{"panic-secret-marker"}
					handler := func(callCtx context.Context, req interface{}) (interface{}, error) {
						if req != request {
							t.Error("request identity changed")
						}
						if withTransport && !oteltrace.SpanFromContext(callCtx).IsRecording() {
							t.Error("handler has no recording span")
						}
						switch mode {
						case "error":
							return response, endpointErr
						case "panic":
							panic(panicValue)
						case "panic nil":
							panic(nil)
						default:
							return response, nil
						}
					}
					endpoint := side.wrap(WithTracerProvider(provider))(handler)
					var gotReply interface{}
					var gotErr error
					returned, recovered := captureEndpointPanic(func() { gotReply, gotErr = endpoint(ctx, request) })
					panicking := strings.HasPrefix(mode, "panic")
					if returned == panicking {
						t.Fatalf("returned=%t for %s", returned, mode)
					}
					if mode == "panic" && recovered != panicValue {
						t.Fatal("original panic value was replaced")
					}
					if mode == "panic nil" {
						directReturned, directValue := captureEndpointPanic(func() { panic(nil) })
						if directReturned || reflect.TypeOf(recovered) != reflect.TypeOf(directValue) {
							t.Fatal("panic(nil) propagation differs from a direct panic")
						}
					}
					if !panicking {
						if gotReply != response {
							t.Error("response identity changed")
						}
						if mode == "error" && gotErr != endpointErr {
							t.Error("original endpoint error was replaced")
						}
						if mode == "success" && gotErr != nil {
							t.Errorf("success returned error: %v", gotErr)
						}
					}
					spans := recorder.Ended()
					if !withTransport {
						if len(spans) != 0 {
							t.Fatalf("unexpected spans without transport: %d", len(spans))
						}
						return
					}
					if len(spans) != 1 {
						t.Fatalf("ended spans = %d, want 1", len(spans))
					}
					span := spans[0]
					if span.SpanKind() != side.kind {
						t.Errorf("span kind = %v, want %v", span.SpanKind(), side.kind)
					}
					wantCode := codes.Error
					if mode == "success" {
						wantCode = codes.Ok
					}
					if span.Status().Code != wantCode {
						t.Errorf("status = %v, want %v", span.Status(), wantCode)
					}
					if mode == "success" {
						if len(span.Events()) != 0 {
							t.Errorf("success recorded events: %v", span.Events())
						}
					} else {
						if len(span.Events()) != 1 || span.Events()[0].Name != "exception" {
							t.Errorf("failure events = %v, want one exception", span.Events())
						}
						wantMessage := "handler panicked"
						if mode == "error" {
							wantMessage = endpointErr.Error()
						}
						if span.Status().Description != wantMessage {
							t.Errorf("status description = %q, want %q", span.Status().Description, wantMessage)
						}
						for _, event := range span.Events() {
							foundMessage := false
							for _, attr := range event.Attributes {
								if string(attr.Key) == "exception.message" {
									foundMessage = true
									if attr.Value.AsString() != wantMessage {
										t.Errorf("exception message = %q, want %q", attr.Value.AsString(), wantMessage)
									}
								}
								if string(attr.Key) == "exception.stacktrace" {
									t.Error("panic stack must not be recorded")
								}
							}
							if !foundMessage {
								t.Error("exception message missing")
							}
						}
					}
					recorded := fmt.Sprintf("%v %v %v", span.Status(), span.Attributes(), span.Events())
					if strings.Contains(recorded, "panic-secret-marker") || strings.Contains(recorded, "request-secret-marker") {
						t.Error("span leaked request or panic contents")
					}
				})
			}
		}
	}
}
