package result

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"time"

	"github.com/songzhibin97/gkit/distributed/backend"
	"github.com/songzhibin97/gkit/distributed/task"
)

// ErrBackendEmpty ...
var ErrBackendEmpty = errors.New("backend is empty")

// AsyncResult 异步结果
type AsyncResult struct {
	Signature *task.Signature // Signature 任务签名
	state     *task.Status    // state 任务状态
	backend   backend.Backend // backend 执行的实现
}

// ChainAsyncResult 链式调用任务结果返回
type ChainAsyncResult struct {
	asyncResult []*AsyncResult
	backend     backend.Backend // backend 执行的实现
}

// GroupCallbackAsyncResult 具有回调任务的个任务组异步结果
type GroupCallbackAsyncResult struct {
	groupAsyncResult    []*AsyncResult
	callbackAsyncResult *AsyncResult
	backend             backend.Backend // backend 执行的实现
}

// NewAsyncResult 创建异步任务返回结果
func NewAsyncResult(signature *task.Signature, backend backend.Backend) *AsyncResult {
	return &AsyncResult{
		Signature: signature,
		state:     &task.Status{},
		backend:   backend,
	}
}

// NewChainAsyncResult 创建链式调用任务返回结果
func NewChainAsyncResult(chainAsyncResult []*task.Signature, backend backend.Backend) *ChainAsyncResult {
	asyncResults := make([]*AsyncResult, 0, len(chainAsyncResult))
	for _, signature := range chainAsyncResult {
		asyncResults = append(asyncResults, NewAsyncResult(signature, backend))
	}
	return &ChainAsyncResult{
		asyncResult: asyncResults,
		backend:     backend,
	}
}

// NewGroupCallbackAsyncResult 创建具有回调任务组的异步返回结果
func NewGroupCallbackAsyncResult(groupAsyncResult []*task.Signature, callbackAsyncResult *task.Signature, backend backend.Backend) *GroupCallbackAsyncResult {
	asyncResults := make([]*AsyncResult, 0, len(groupAsyncResult))
	for _, signature := range groupAsyncResult {
		asyncResults = append(asyncResults, NewAsyncResult(signature, backend))
	}
	return &GroupCallbackAsyncResult{
		groupAsyncResult:    asyncResults,
		callbackAsyncResult: NewAsyncResult(callbackAsyncResult, backend),
		backend:             backend,
	}
}

func waitForPoll(ctx context.Context, duration time.Duration) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	timer := time.NewTimer(duration)
	select {
	case <-ctx.Done():
		if !timer.Stop() {
			<-timer.C
		}
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

// Get 返回结果
func (asyncResult *AsyncResult) Get(sleepDuration time.Duration) ([]reflect.Value, error) {
	for {
		results, err := asyncResult.Monitor()
		if results == nil && err == nil {
			time.Sleep(sleepDuration)
		} else {
			return results, err
		}
	}
}

// GetWithTimeout returns results within the shared timeout budget when the
// backend implements backend.ContextStatusBackend. Legacy GetStatus is
// synchronous and may return late; any result arriving after the deadline is
// discarded. No background read goroutine is started.
func (asyncResult *AsyncResult) GetWithTimeout(timeoutDuration, sleepDuration time.Duration) ([]reflect.Value, error) {
	ctx, cancer := context.WithTimeout(context.Background(), timeoutDuration)
	defer cancer()
	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
			results, err := asyncResult.monitor(ctx)
			if results == nil && err == nil {
				if err := waitForPoll(ctx, sleepDuration); err != nil {
					return nil, err
				}
			} else {
				return results, err
			}
		}
	}
}

// Monitor 监视任务
func (asyncResult *AsyncResult) Monitor() ([]reflect.Value, error) {
	return asyncResult.monitor(context.Background())
}

func (asyncResult *AsyncResult) monitor(ctx context.Context) (values []reflect.Value, err error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	defer func() {
		if deadlineErr := ctx.Err(); deadlineErr != nil {
			values, err = nil, deadlineErr
		}
	}()
	if asyncResult.backend == nil {
		return nil, ErrBackendEmpty
	}

	state, err := asyncResult.getStateContext(ctx)
	if err != nil {
		return nil, err
	}
	if state.IsFailure() {
		return nil, errors.New(state.Error)
	}
	if state.IsSuccess() {
		return task.ReflectTaskResults(state.Results)
	}
	return nil, nil
}

// GetStateWithError gets the current task state and exposes backend read
// failures with task context. The last cached state is preserved on failure.
func (asyncResult *AsyncResult) GetStateWithError() (*task.Status, error) {
	return asyncResult.getStateContext(context.Background())
}

func (asyncResult *AsyncResult) getStateContext(ctx context.Context) (*task.Status, error) {
	if err := ctx.Err(); err != nil {
		return asyncResult.state, err
	}
	if asyncResult.state.IsCompleted() {
		return asyncResult.state, nil
	}
	if asyncResult.backend == nil {
		return asyncResult.state, ErrBackendEmpty
	}
	var taskState *task.Status
	var err error
	if reader, ok := asyncResult.backend.(backend.ContextStatusBackend); ok {
		taskState, err = reader.GetStatusContext(ctx, asyncResult.Signature.ID)
	} else {
		taskState, err = asyncResult.backend.GetStatus(asyncResult.Signature.ID)
	}
	if deadlineErr := ctx.Err(); deadlineErr != nil {
		return asyncResult.state, deadlineErr
	}
	if err != nil {
		return asyncResult.state, fmt.Errorf(
			"result: get state for task %q: %w",
			asyncResult.Signature.ID,
			err,
		)
	}
	asyncResult.state = taskState
	return asyncResult.state, nil
}

// GetState gets the current task state. This compatibility wrapper preserves
// the existing signature and deliberately discards backend read errors; use
// GetStateWithError when the caller must distinguish a read failure.
func (asyncResult *AsyncResult) GetState() *task.Status {
	state, _ := asyncResult.GetStateWithError()
	return state
}

// Get 返回结果
func (chainAsyncResult *ChainAsyncResult) Get(sleepDuration time.Duration) ([]reflect.Value, error) {
	if chainAsyncResult.backend == nil {
		return nil, ErrBackendEmpty
	}
	var (
		results []reflect.Value
		err     error
	)
	for _, result := range chainAsyncResult.asyncResult {
		results, err = result.Get(sleepDuration)
		if err != nil {
			return nil, err
		}
	}
	return results, err
}

// GetWithTimeout returns results within the shared timeout budget when the
// backend implements backend.ContextStatusBackend. Legacy GetStatus is
// synchronous and may return late; any result arriving after the deadline is
// discarded. No background read goroutine is started.
func (chainAsyncResult *ChainAsyncResult) GetWithTimeout(timeoutDuration, sleepDuration time.Duration) ([]reflect.Value, error) {
	if chainAsyncResult.backend == nil {
		return nil, ErrBackendEmpty
	}
	var (
		results    []reflect.Value
		err        error
		ln         = len(chainAsyncResult.asyncResult)
		lastResult = chainAsyncResult.asyncResult[ln-1]
	)
	ctx, cancel := context.WithTimeout(context.Background(), timeoutDuration)
	defer cancel()
	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
			for _, result := range chainAsyncResult.asyncResult {
				_, err = result.monitor(ctx)
				if err != nil {
					return nil, err
				}
			}
			results, err = lastResult.monitor(ctx)
			if err != nil {
				return nil, err
			}
			if results != nil {
				return results, err
			}
			if err := waitForPoll(ctx, sleepDuration); err != nil {
				return nil, err
			}
		}
	}
}

// Get 返回结果
func (groupCallbackAsyncResult *GroupCallbackAsyncResult) Get(sleepDuration time.Duration) ([]reflect.Value, error) {
	if groupCallbackAsyncResult.backend == nil {
		return nil, ErrBackendEmpty
	}
	var err error
	for _, result := range groupCallbackAsyncResult.groupAsyncResult {
		_, err = result.Get(sleepDuration)
		if err != nil {
			return nil, err
		}
	}
	return groupCallbackAsyncResult.callbackAsyncResult.Get(sleepDuration)
}

// GetWithTimeout returns results within the shared timeout budget when the
// backend implements backend.ContextStatusBackend. Legacy GetStatus is
// synchronous and may return late; any result arriving after the deadline is
// discarded. No background read goroutine is started.
func (groupCallbackAsyncResult *GroupCallbackAsyncResult) GetWithTimeout(timeoutDuration, sleepDuration time.Duration) ([]reflect.Value, error) {
	if groupCallbackAsyncResult.backend == nil {
		return nil, ErrBackendEmpty
	}
	var (
		results []reflect.Value
		err     error
	)
	ctx, cancel := context.WithTimeout(context.Background(), timeoutDuration)
	defer cancel()
	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
			for _, result := range groupCallbackAsyncResult.groupAsyncResult {
				_, err = result.monitor(ctx)
				if err != nil {
					return nil, err
				}
			}
			results, err = groupCallbackAsyncResult.callbackAsyncResult.monitor(ctx)
			if err != nil {
				return nil, err
			}
			if results != nil {
				return results, err
			}
			if err := waitForPoll(ctx, sleepDuration); err != nil {
				return nil, err
			}
		}
	}
}
