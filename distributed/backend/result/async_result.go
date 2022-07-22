package result

import (
	"context"
	"errors"
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

// GetWithTimeout 返回结果 带有超时时间
func (asyncResult *AsyncResult) GetWithTimeout(timeoutDuration, sleepDuration time.Duration) ([]reflect.Value, error) {
	ctx, cancer := context.WithTimeout(context.Background(), timeoutDuration)
	defer cancer()
	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
			results, err := asyncResult.Monitor()
			if results == nil && err == nil {
				time.Sleep(sleepDuration)
			} else {
				return results, err
			}
		}
	}
}

// Monitor 监视任务
func (asyncResult *AsyncResult) Monitor() ([]reflect.Value, error) {
	if asyncResult.backend == nil {
		return nil, ErrBackendEmpty
	}

	asyncResult.GetState()
	if asyncResult.state.IsFailure() {
		return nil, errors.New(asyncResult.state.Error)
	}
	if asyncResult.state.IsSuccess() {
		return task.ReflectTaskResults(asyncResult.state.Results)
	}
	return nil, nil
}

// GetState 获取任务状态
func (asyncResult *AsyncResult) GetState() *task.Status {
	if asyncResult.state.IsCompleted() {
		return asyncResult.state
	}
	taskState, err := asyncResult.backend.GetStatus(asyncResult.Signature.ID)
	if err == nil {
		asyncResult.state = taskState
	}
	return asyncResult.state
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

// GetWithTimeout 返回结果 带有超时时间
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
				_, err = result.Monitor()
				if err != nil {
					return nil, err
				}
			}
			results, err = lastResult.Monitor()
			if err != nil {
				return nil, err
			}
			if results != nil {
				return results, err
			}
			time.Sleep(sleepDuration)
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

// GetWithTimeout 返回结果 带有超时时间
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
				_, err = result.Monitor()
				if err != nil {
					return nil, err
				}
			}
			results, err = groupCallbackAsyncResult.callbackAsyncResult.Monitor()
			if err != nil {
				return nil, err
			}
			if results != nil {
				return results, err
			}
			time.Sleep(sleepDuration)
		}
	}
}
