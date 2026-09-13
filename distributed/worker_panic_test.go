package distributed_test

import (
	"context"
	"errors"
	"fmt"
	"github.com/alicebob/miniredis/v2"
	"github.com/go-redis/redis/v8"
	"github.com/songzhibin97/gkit/distributed"
	"github.com/songzhibin97/gkit/distributed/backend"
	backendredis "github.com/songzhibin97/gkit/distributed/backend/backend_redis"
	"github.com/songzhibin97/gkit/distributed/controller"
	"github.com/songzhibin97/gkit/distributed/task"
	"os"
	"testing"
	"time"
)

type panicOutcomeController struct{ controller.Controller }

func (*panicOutcomeController) RegisterTask(...string)   {}
func (*panicOutcomeController) SetConsumingQueue(string) {}
func (*panicOutcomeController) SetDelayedQueue(string)   {}

type panicOutcomeBackend struct{ backend.Backend }

func TestWorkerPersistsPanicsAsFailures(t *testing.T) {
	addr := os.Getenv("GKIT_REDIS_ADDR")
	if addr == "" {
		addr = miniredis.RunT(t).Addr()
		t.Log("using miniredis")
	} else {
		t.Log("using configured Redis")
	}
	client := redis.NewClient(&redis.Options{Addr: addr})
	defer client.Close()
	if err := client.Ping(context.Background()).Err(); err != nil {
		t.Fatal(err)
	}
	b := backendredis.NewBackendRedis(client, -1)
	server, err := distributed.NewServerE(&panicOutcomeController{}, panicOutcomeBackend{b}, nil, nil, nil, distributed.SetNoUnixSignals(true))
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		if err := server.Shutdown(ctx); err != nil {
			t.Error(err)
		}
	}()
	worker := server.NewWorker("review", 1, "unused")
	sentinel := errors.New("controlled task panic")
	for _, mode := range []string{"normal", "error_panic", "nil_panic"} {
		t.Run(mode, func(t *testing.T) {
			signature := task.NewSignature(fmt.Sprintf("review-nil-%s-%d", mode, time.Now().UnixNano()), mode)
			signature.RetryCount = 0
			defer func() {
				if err := b.ResetTask(signature.ID); err != nil {
					t.Error(err)
				}
			}()
			var observed error
			worker.SetErrorHandler(func(err error) { observed = err })
			if err := server.RegisteredTask(mode, func() (string, error) {
				if mode == "error_panic" {
					panic(sentinel)
				}
				if mode == "nil_panic" {
					panic(nil)
				}
				return "completed", nil
			}); err != nil {
				t.Fatal(err)
			}
			if err := worker.Process(signature); err != nil {
				t.Fatal(err)
			}
			state, err := b.GetStatus(signature.ID)
			if err != nil {
				t.Fatal(err)
			}
			raw, rawErr := client.Get(context.Background(), signature.ID).Result()
			if rawErr != nil {
				t.Fatal(rawErr)
			}
			t.Logf("raw Redis value=%s", raw)
			t.Logf("persisted status=%s results=%v observedError=%v", state.Status, state.Results, observed)
			if mode == "normal" {
				if state.Status != task.StateSuccess || len(state.Results) != 1 || state.Results[0].Value != "completed" || observed != nil {
					t.Fatalf("normal result wrong: %#v error=%v", state, observed)
				}
			} else {
				if state.Status != task.StateFailure || observed == nil {
					t.Fatalf("panicked task recorded as %s; error=%v", state.Status, observed)
				}
				if mode == "error_panic" && observed != sentinel {
					t.Fatalf("original error lost: %v", observed)
				}
			}
		})
	}
}
