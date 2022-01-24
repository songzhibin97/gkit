package distributed

import (
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/songzhibin97/gkit/distributed/retry"

	"github.com/pkg/errors"

	"github.com/songzhibin97/gkit/distributed/task"
)

var (
	ErrWorkerGracefullyQuit = errors.New("worker quit gracefully")
	ErrWorkerAbruptlyQuit   = errors.New("worker quit abruptly")
)

// Worker 任务处理
type Worker struct {
	NoUnixSignals     bool `json:"no_unix_signals"` // 是否使用unix信号控制
	bindService       *Server
	Concurrency       int                        `json:"concurrency"`  // 并发数
	ConsumerTag       string                     `json:"consumer_tag"` // 消费者标签
	Queue             string                     `json:"queue"`        // 队列名称
	errorHandler      func(err error)            // 错误处理
	beforeTaskHandler func(task *task.Signature) // 在任务执行前执行
	afterTaskHandler  func(task *task.Signature) // 在任务结束后执行
	preConsumeHandler func(worker *Worker) bool  // 判断是否需要预处理
}

// SetErrorHandler 设置处理错误函数
func (w *Worker) SetErrorHandler(f func(err error)) {
	w.errorHandler = f
}

// SetBeforeTaskHandler 设置执行任务前执行函数
func (w *Worker) SetBeforeTaskHandler(f func(task *task.Signature)) {
	w.beforeTaskHandler = f
}

// SetAfterTaskHandler 设置执行任务后执行函数
func (w *Worker) SetAfterTaskHandler(f func(task *task.Signature)) {
	w.afterTaskHandler = f
}

// SetPreConsumeHandler 设置预处理函数
func (w *Worker) SetPreConsumeHandler(f func(worker *Worker) bool) {
	w.preConsumeHandler = f
}

// Quit 触发强制退出
func (w *Worker) Quit() {
	w.bindService.GetController().StopConsuming()
}

// Process 任务处理
func (w *Worker) Process(signature *task.Signature) error {
	// 如果任务没有注册,快速返回
	if !w.bindService.IsRegisteredTask(signature.Name) {
		return nil
	}
	// 获取注册执行任务的函数
	handler, ok := w.bindService.GetRegisteredTask(signature.Name)
	if !ok {
		return nil
	}
	// 设置任务状态,改为接收状态
	if err := w.bindService.GetBackend().SetStateReceived(signature); err != nil {
		return errors.Wrap(err, "worker set task state to 'received' error, signature id:"+signature.ID)
	}
	exec, err := task.NewTaskWithSignature(handler, signature)
	if err != nil {
		_ = w.handlerFailed(signature, err)
		return err
	}

	// 设置任务状态,改为开始执行
	if err = w.bindService.GetBackend().SetStateStarted(signature); err != nil {
		return errors.Wrap(err, "worker set task state to 'started' error, signature id:"+signature.ID)
	}

	// 生命周期前执行
	if w.beforeTaskHandler != nil {
		w.beforeTaskHandler(signature)
	}
	// 生命周期后执行,注册defer
	if w.afterTaskHandler != nil {
		defer w.afterTaskHandler(signature)
	}
	// 任务调用
	results, err := exec.Call()
	if err != nil {
		// 判断err是否是可重试错误
		retryErr, ok := (interface{})(err).(task.ErrRetryTaskLater)
		if ok {
			// 重试
			return w.handlerRetryIn(signature, retryErr.RetryIn())
		}
		// 根据自定义重试次数开始重试
		if signature.RetryCount > 0 {
			return w.handlerRetry(signature)
		}
		// 置为失败
		return w.handlerFailed(signature, err)
	}
	// 任务完成
	return w.handlerSucceeded(signature, results)
}

// handlerRetry 处理程序重试
func (w *Worker) handlerRetry(signature *task.Signature) error {
	// 设置重试状态
	if err := w.bindService.GetBackend().SetStateRetry(signature); err != nil {
		return errors.Wrap(err, "worker set task state to 'retry' error, signature id:"+signature.ID)
	}
	signature.RetryCount--

	// 获取间隔时间
	signature.RetryInterval = retry.FibonacciNext(signature.RetryInterval)

	eta := time.Now().Add(time.Second * time.Duration(signature.RetryInterval))
	signature.ETA = &eta
	w.bindService.helper.Warnf("Task %s failed. Going to retry in %d seconds.", signature.ID, signature.RetryInterval)
	_, err := w.bindService.SendTask(signature)
	return err
}

// handlerRetry 处理指定错误程序重试
func (w *Worker) handlerRetryIn(signature *task.Signature, retryIn time.Duration) error {
	// 设置重试状态
	if err := w.bindService.GetBackend().SetStateRetry(signature); err != nil {
		return errors.Wrap(err, "worker set task state to 'retry' error, signature id:"+signature.ID)
	}
	eta := time.Now().Add(retryIn)
	signature.ETA = &eta
	w.bindService.helper.Warnf("Task %s failed. Going to retry in %.0f seconds.", signature.ID, retryIn.Seconds())
	_, err := w.bindService.SendTask(signature)
	return err
}

// handlerSucceeded 处理程序成功状态
func (w *Worker) handlerSucceeded(signature *task.Signature, results []*task.Result) error {
	if err := w.bindService.GetBackend().SetStateSuccess(signature, results); err != nil {
		return errors.Wrap(err, "worker set task state to 'succeeded' error, signature id:"+signature.ID)
	}

	// 执行任务成功回调
	for _, success := range signature.CallbackOnSuccess {
		for _, result := range results {
			success.Args = append(success.Args,
				task.Arg{
					Type:  result.Type,
					Value: result.Value,
				},
			)
		}
		_, _ = w.bindService.SendTask(success)
	}

	// 如果没有回调,完成
	if signature.CallbackChord == nil {
		return nil
	}

	// 不是任务组,完成
	if signature.GroupID == "" {
		return nil
	}

	// 检查组内任务是否完成
	groupCompleted, err := w.bindService.GetBackend().GroupCompleted(signature.GroupID)
	if err != nil {
		return errors.Wrap(err, "group completed id:"+signature.ID+"group id:"+signature.GroupID)
	}
	// 不是组内最后一个任务
	if !groupCompleted {
		return nil
	}

	// 调用组回调
	call, err := w.bindService.GetBackend().TriggerCompleted(signature.GroupID)
	if err != nil {
		return fmt.Errorf("TriggerCompleted group %s returned error: %s", signature.GroupID, err)
	}

	if !call {
		// 已经调用过了
		return nil
	}

	// 获取组任务状态
	taskStatus, err := w.bindService.GetBackend().GroupTaskStatus(signature.GroupID)
	if err != nil {
		w.bindService.helper.Errorf(
			"Failed to get tasks states for group:[%s]. Task count:[%d]. The chord may not be triggered. Error:[%s]",
			signature.ID,
			signature.GroupTaskCount,
			err,
		)
		return nil
	}
	// 遍历任务状态
	for _, status := range taskStatus {
		if !status.IsSuccess() {
			// 如果有未成功的任务,组任务失败
			return nil
		}
		for _, result := range status.Results {
			signature.CallbackChord.Args = append(signature.CallbackChord.Args,
				task.Arg{Type: result.Type, Value: result.Value})
		}
	}
	_, err = w.bindService.SendTask(signature.CallbackChord)
	return err
}

// handlerFailed 处理任务失败状态
func (w *Worker) handlerFailed(signature *task.Signature, err error) error {
	if err := w.bindService.GetBackend().SetStateFailure(signature, err.Error()); err != nil {
		return errors.Wrap(err, "worker set task state to 'succeeded' error, signature id:"+signature.ID)
	}
	if w.errorHandler != nil {
		w.errorHandler(err)
	} else {
		w.bindService.helper.Errorf("Failed processing task %s. Error = %v", signature.ID, err)
	}
	for _, _error := range signature.CallbackOnError {
		_error.Args = append([]task.Arg{{Type: "string", Value: err.Error()}}, _error.Args...)
		_, _ = w.bindService.SendTask(_error)
	}
	if signature.StopTaskDeletionOnError {
		return errors.New("StopTaskDeletionOnError")
	}
	return nil
}

func (w *Worker) ConsumeQueue() string {
	return w.Queue
}

func (w *Worker) PreConsumeHandler() bool {
	if w.preConsumeHandler != nil {
		return true
	}
	return w.preConsumeHandler(w)
}

// Start 启动
func (w *Worker) Start() error {
	errChan := make(chan error, 1)
	w.StartSync(errChan)
	return <-errChan
}

// StartSync 异步启动
func (w *Worker) StartSync(errChan chan<- error) {
	w.bindService.helper.Info("worker start")
	w.bindService.helper.Info("worker tag", w.ConsumerTag)
	w.bindService.helper.Info("use queue", w.Queue)
	controller := w.bindService.GetController()

	var wg sync.WaitGroup
	go func() {
		for {
			retry, err := controller.StartConsuming(w.Concurrency, w)
			if retry {
				if w.errorHandler != nil {
					w.errorHandler(err)
				} else {
					w.bindService.helper.Warnf("controller consume err: %s", err)
				}
			} else {
				wg.Wait()
				errChan <- err
				return
			}
		}
	}()
	if !w.NoUnixSignals {
		sign := make(chan os.Signal, 1)
		signal.Notify(sign, os.Interrupt, syscall.SIGTERM)

		var acceptCount uint
		go func() {
			for s := range sign {
				w.bindService.helper.Warnf("signal received: %v", s)
				acceptCount++
				if acceptCount < 2 {
					// 正常退出
					w.bindService.helper.Warn("Waiting for running tasks to finish before shutting down")
					wg.Add(1)
					go func() {
						w.Quit()
						errChan <- ErrWorkerGracefullyQuit
						wg.Done()
					}()
				} else {
					// 重复收到退出信号
					errChan <- ErrWorkerAbruptlyQuit
				}
			}
		}()
	}
}
