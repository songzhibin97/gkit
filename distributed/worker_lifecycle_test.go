package distributed

import (
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"github.com/songzhibin97/gkit/distributed/task"
	"github.com/songzhibin97/gkit/log"
)

type workerLifecycleController struct {
	groupTestController
	start func() (bool, error)
	stop  func()
}

func (c *workerLifecycleController) StartConsuming(int, task.Processor) (bool, error) {
	return c.start()
}
func (c *workerLifecycleController) StopConsuming() {
	if c.stop != nil {
		c.stop()
	}
}

// #166 / 03-08: all real signals are sent only by isolated Go test children to
// themselves. Subscription removal is verified separately from goroutine exit.
func TestWorkerLifecycleSubprocess(t *testing.T) {
	const childEnv = "GKIT_WORKER_LIFECYCLE_CHILD"
	if scenario := os.Getenv(childEnv); scenario != "" {
		runWorkerLifecycleScenario(t, scenario)
		if t.Failed() {
			os.Exit(1)
		}
		os.Exit(0)
	}
	for _, scenario := range []string{"normal", "error", "retry", "external_quit", "graceful", "abrupt", "repeated", "no_signals", "subscription_cleanup", "no_signal_ownership"} {
		t.Run(scenario, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
			defer cancel()
			cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestWorkerLifecycleSubprocess$", "-test.v=true")
			cmd.Env = append(os.Environ(), childEnv+"="+scenario)
			output, err := cmd.CombinedOutput()
			if scenario == "subscription_cleanup" || scenario == "no_signal_ownership" {
				var exitErr *exec.ExitError
				if ctx.Err() != nil || !errors.As(err, &exitErr) {
					t.Fatalf("default SIGTERM exit not restored: err=%v context=%v output=%s", err, ctx.Err(), output)
				}
				status, ok := exitErr.Sys().(syscall.WaitStatus)
				if !ok || !status.Signaled() || status.Signal() != syscall.SIGTERM {
					t.Fatalf("child exit = %v, want SIGTERM; output=%s", exitErr, output)
				}
				return
			}
			if err != nil {
				t.Fatalf("worker child failed: %v\n%s", err, output)
			}
		})
	}
}

func lifecycleWorker(controller *workerLifecycleController, noSignals bool) *Worker {
	server := &Server{config: &Config{NoUnixSignals: noSignals}, controller: controller, helper: log.NewHelper(log.NewStdLogger(io.Discard))}
	return server.NewWorker("lifecycle", 1, "test-queue")
}

func runWorkerLifecycleScenario(t *testing.T, scenario string) {
	if scenario == "repeated" {
		var completions []chan error
		for i := 0; i < 8; i++ {
			worker := lifecycleWorker(&workerLifecycleController{start: func() (bool, error) { return false, nil }}, false)
			result := make(chan error, 4)
			worker.StartSync(result)
			if err := awaitWorkerCompletion(t, result); err != nil {
				t.Fatal(err)
			}
			completions = append(completions, result)
		}
		assertWorkerLifecycleExited(t)
		for _, result := range completions {
			assertNoExtraWorkerCompletion(t, result)
		}
		return
	}
	started, consumeRelease, consumeReturned := make(chan struct{}), make(chan struct{}), make(chan struct{})
	stopEntered, stopRelease, stopReturned := make(chan struct{}), make(chan struct{}), make(chan struct{})
	var stopOnce sync.Once
	var calls, stops atomic.Int32
	retryErr := errors.New("synthetic retry requested")
	var finalErr error
	if scenario == "error" || scenario == "retry" {
		finalErr = errors.New("synthetic consume result")
	}
	expectedCalls := int32(1)
	if scenario == "retry" {
		expectedCalls = 2
	}
	controller := &workerLifecycleController{
		start: func() (bool, error) {
			call := calls.Add(1)
			if scenario == "retry" && call == 1 {
				return true, retryErr
			}
			if call != expectedCalls {
				return false, errors.New("unexpected late consume attempt")
			}
			close(started)
			<-consumeRelease
			close(consumeReturned)
			if scenario == "abrupt" {
				return true, retryErr
			}
			return false, finalErr
		},
		stop: func() {
			if stops.Add(1) == 1 {
				close(stopEntered)
			}
			<-stopRelease
			stopOnce.Do(func() { close(stopReturned) })
		},
	}
	worker := lifecycleWorker(controller, scenario == "no_signals" || scenario == "no_signal_ownership")
	reported := make(chan error, 4)
	worker.SetErrorHandler(func(err error) { reported <- err })
	result := make(chan error, 4)
	worker.StartSync(result)
	waitWorkerEvent(t, started, "consume start")
	if scenario == "no_signal_ownership" {
		terminateWorkerTestChild(t)
	}
	var got error
	switch scenario {
	case "graceful", "abrupt":
		signalWorkerTestChild(t, syscall.SIGTERM)
		waitWorkerEvent(t, stopEntered, "first signal Quit")
		if scenario == "abrupt" {
			signalWorkerTestChild(t, syscall.SIGINT)
			got = awaitWorkerCompletion(t, result)
			if got != ErrWorkerAbruptlyQuit {
				t.Errorf("second signal result = %v, want abrupt quit", got)
			}
			close(stopRelease)
			close(consumeRelease)
		} else {
			close(consumeRelease)
			waitWorkerEvent(t, consumeReturned, "consume return before Quit completion")
			select {
			case early := <-result:
				t.Fatalf("completion before Quit returned: %v", early)
			case <-time.After(50 * time.Millisecond):
			}
			close(stopRelease)
			got = awaitWorkerCompletion(t, result)
			if got != ErrWorkerGracefullyQuit {
				t.Errorf("first signal result = %v, want graceful quit", got)
			}
		}
		waitWorkerEvent(t, stopReturned, "Quit return")
	case "external_quit":
		quitDone := make(chan struct{})
		go func() { worker.Quit(); close(quitDone) }()
		waitWorkerEvent(t, stopEntered, "external Quit")
		close(stopRelease)
		close(consumeRelease)
		waitWorkerEvent(t, quitDone, "external Quit return")
		got = awaitWorkerCompletion(t, result)
		if got != finalErr {
			t.Errorf("external Quit changed consume result: %v", got)
		}
	default:
		close(consumeRelease)
		got = awaitWorkerCompletion(t, result)
		if got != finalErr {
			t.Errorf("normal completion = %v, want %v", got, finalErr)
		}
	}
	if scenario == "subscription_cleanup" {
		terminateWorkerTestChild(t)
	}
	waitWorkerEvent(t, consumeReturned, "consume return")
	assertWorkerLifecycleExited(t)
	assertNoExtraWorkerCompletion(t, result)
	if got := calls.Load(); got != expectedCalls {
		t.Errorf("consume calls = %d, want %d (no retry after completion)", got, expectedCalls)
	}
	wantStops := int32(0)
	if scenario == "graceful" || scenario == "abrupt" || scenario == "external_quit" {
		wantStops = 1
	}
	if got := stops.Load(); got != wantStops {
		t.Errorf("StopConsuming calls = %d, want %d", got, wantStops)
	}
	if scenario == "retry" {
		select {
		case err := <-reported:
			if err != retryErr {
				t.Errorf("retry error = %v, want %v", err, retryErr)
			}
		default:
			t.Error("active retry error was not reported")
		}
	}
}

func signalWorkerTestChild(t *testing.T, value syscall.Signal) {
	t.Helper()
	if os.Getenv("GKIT_WORKER_LIFECYCLE_CHILD") == "" {
		t.Fatal("refusing to signal a non-child test process")
	}
	if err := syscall.Kill(os.Getpid(), value); err != nil {
		t.Fatal(err)
	}
}

func terminateWorkerTestChild(t *testing.T) {
	t.Helper()
	signalWorkerTestChild(t, syscall.SIGTERM)
	select {} // Parent watchdog fails if a leaked subscription swallows SIGTERM.
}

func waitWorkerEvent(t *testing.T, event <-chan struct{}, name string) {
	t.Helper()
	select {
	case <-event:
	case <-time.After(2 * time.Second):
		t.Fatalf("timed out: %s", name)
	}
}

func awaitWorkerCompletion(t *testing.T, result <-chan error) error {
	t.Helper()
	select {
	case err := <-result:
		return err
	case <-time.After(2 * time.Second):
		t.Fatal("worker did not report completion")
		return nil
	}
}

func assertNoExtraWorkerCompletion(t *testing.T, result <-chan error) {
	t.Helper()
	for {
		select {
		case err := <-result:
			t.Errorf("duplicate completion: %v", err)
		default:
			return
		}
	}
}

func assertWorkerLifecycleExited(t *testing.T) {
	t.Helper()
	stack := make([]byte, 1<<20)
	deadline := time.Now().Add(time.Second)
	for {
		n := runtime.Stack(stack, true)
		if n == len(stack) {
			t.Fatal("worker stack snapshot truncated")
		}
		count := 0
		for _, goroutine := range strings.Split(string(stack[:n]), "\n\n") {
			if strings.Contains(goroutine, "github.com/songzhibin97/gkit/distributed.(*Worker).") {
				count++
			}
		}
		if count == 0 {
			return
		}
		if time.Now().After(deadline) {
			t.Errorf("worker lifecycle goroutines remaining = %d, want 0", count)
			return
		}
		runtime.Gosched()
	}
}
