package middleware

import (
	"context"
	"errors"
	"reflect"
	"testing"
)

func TestChainOrderAndResult(t *testing.T) {
	var events []string
	wantResponse := &struct{ value string }{value: "response"}
	wantErr := errors.New("endpoint error")
	wantContext := context.WithValue(context.Background(), struct{}{}, "context")
	wantRequest := &struct{ value string }{value: "request"}

	endpoint := func(ctx context.Context, request interface{}) (interface{}, error) {
		if ctx != wantContext {
			t.Fatalf("endpoint context = %v, want original context", ctx)
		}
		if request != wantRequest {
			t.Fatalf("endpoint request = %v, want original request", request)
		}
		events = append(events, "endpoint")
		return wantResponse, wantErr
	}

	chained := Chain(
		recordEvents(&events, "first"),
		recordEvents(&events, "second"),
		recordEvents(&events, "third"),
	)(endpoint)

	gotResponse, gotErr := chained(wantContext, wantRequest)
	if gotResponse != wantResponse {
		t.Fatalf("response = %v, want original response %v", gotResponse, wantResponse)
	}
	if gotErr != wantErr {
		t.Fatalf("error = %v, want original error %v", gotErr, wantErr)
	}
	wantEvents := []string{
		"first pre",
		"second pre",
		"third pre",
		"endpoint",
		"third post",
		"second post",
		"first post",
	}
	if !reflect.DeepEqual(events, wantEvents) {
		t.Fatalf("events = %v, want %v", events, wantEvents)
	}
}

func recordEvents(events *[]string, name string) MiddleWare {
	return func(next Endpoint) Endpoint {
		return func(ctx context.Context, request interface{}) (interface{}, error) {
			*events = append(*events, name+" pre")
			response, err := next(ctx, request)
			*events = append(*events, name+" post")
			return response, err
		}
	}
}
