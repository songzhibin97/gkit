package goroutine_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/songzhibin97/gkit/goroutine"
)

func TestDelegateCompletesAfterPanics(t *testing.T) {
	sentinel := errors.New("controlled callback error")
	for _, mode := range []string{"normal", "returned_error", "error_panic", "nil_panic", "string_panic", "value_panic"} {
		t.Run(mode, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			entered := make(chan struct{})
			result := make(chan error, 1)
			go func() {
				result <- goroutine.Delegate(ctx, 0, func(context.Context) error {
					close(entered)
					switch mode {
					case "returned_error":
						return sentinel
					case "error_panic":
						panic(sentinel)
					case "nil_panic":
						panic(nil)
					case "string_panic":
						panic("controlled string panic")
					case "value_panic":
						panic(42)
					}
					return nil
				})
			}()
			select {
			case <-entered:
			case <-time.After(time.Second):
				t.Fatal("callback did not start")
			}
			select {
			case err := <-result:
				if mode == "normal" {
					if err != nil {
						t.Fatalf("normal callback returned %v", err)
					}
					return
				}
				if err == nil || errors.Is(err, context.Canceled) {
					t.Fatalf("callback error was lost or replaced by cancellation: %v", err)
				}
				if (mode == "error_panic" || mode == "returned_error") && err != sentinel {
					t.Fatalf("original error identity lost: %v", err)
				}
				if mode == "string_panic" && err.Error() != "controlled string panic" {
					t.Fatalf("string panic changed: %v", err)
				}
				if mode == "value_panic" && err.Error() != "42" {
					t.Fatalf("value panic changed: %v", err)
				}
			case <-time.After(time.Second):
				// Reclaim the original broken call without letting cancellation
				// turn its missing completion into a passing result.
				cancel()
				select {
				case err := <-result:
					t.Fatalf("Delegate needed external cancellation after %s: %v", mode, err)
				case <-time.After(time.Second):
					t.Fatal("Delegate did not return even after cancellation")
				}
			}
		})
	}
}
