package distributed

import (
	"context"
	cryptorand "crypto/rand"
	"encoding/hex"
	stderrors "errors"
	"fmt"
	"sync"
	"time"

	"github.com/songzhibin97/gkit/options"
	"github.com/songzhibin97/gkit/tools/rand_string"

	"github.com/songzhibin97/gkit/log"

	"github.com/pkg/errors"

	"github.com/songzhibin97/gkit/distributed/backend/result"

	"github.com/songzhibin97/gkit/distributed/task"

	"github.com/robfig/cron/v3"

	"github.com/songzhibin97/gkit/distributed/backend"
	"github.com/songzhibin97/gkit/distributed/controller"
	"github.com/songzhibin97/gkit/distributed/locker"
)

// minLockTTLMs is the floor we apply when computing a timed-task lock TTL.
// `int(time.Until(next).Milliseconds())` can be negative (next already
// passed) or zero (sub-millisecond cadence), and Redis PEXPIRE rejects
// non-positive values — the SET fails, the cron fire skips, every
// subsequent fire repeats the failure. Clamping guarantees the lock
// genuinely tries to acquire.
const minLockTTLMs = 100

const timedRunSuffixLength = 16

const taskPublicationFailureMessage = "task publication outcome unknown"

const publicationAttemptIDBytes = 16

// timedTaskLockTTL clamps the user-requested TTL into a Redis-acceptable
// positive integer.
func timedTaskLockTTL(d time.Duration) int {
	ms := int(d.Milliseconds())
	if ms < minLockTTLMs {
		return minLockTTLMs
	}
	return ms
}

// Config distributed service配置文件
type Config struct {
	NoUnixSignals bool   `json:"no_unix_signals"`
	ResultExpire  int64  `json:"result_expire"`
	Concurrency   int64  `json:"concurrency"`
	ConsumeQueue  string `json:"consume_queue"`
	DelayedQueue  string `json:"delayed_queue"`
}

type Server struct {
	config            *Config
	registeredTasks   *sync.Map                  // registeredTasks 注册任务处理函数
	controller        controller.Controller      // controller 控制器
	backend           backend.Backend            // backend 后端引擎
	lock              locker.Locker              // lock 锁
	scheduler         *cron.Cron                 // scheduler 调度器
	prePublishHandler func(task *task.Signature) // prePublishHandler 预处理器
	helper            *log.Helper
	// publicationAttemptID is injectable only for deterministic failure tests.
	// Production servers use generatePublicationAttemptID.
	publicationAttemptID func() (string, error)
}

// GetConfig 获取配置文件
func (s *Server) GetConfig() *Config {
	return s.config
}

// GetController 获取 Controller
func (s *Server) GetController() controller.Controller {
	return s.controller
}

// GetBackend 获取 Backend
func (s *Server) GetBackend() backend.Backend {
	return s.backend
}

// GetLocker 获取 Locker
func (s *Server) GetLocker() locker.Locker {
	return s.lock
}

func (s *Server) bindDefaultRouter(signature *task.Signature) {
	if signature.Router == "" && s.config != nil {
		signature.Router = s.config.ConsumeQueue
	}
}

func generatePublicationAttemptID() (string, error) {
	var value [publicationAttemptIDBytes]byte
	if _, err := cryptorand.Read(value[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(value[:]), nil
}

func (s *Server) nextPublicationAttemptID() (string, error) {
	if s.publicationAttemptID != nil {
		return s.publicationAttemptID()
	}
	return generatePublicationAttemptID()
}

// RegisteredTasks 注册多个任务
// handelTaskMap map[string]interface{}
// interface 规则: 必须是func且必须有返回参数,最后一个出参是error
func (s *Server) RegisteredTasks(handelTaskMap map[string]interface{}) error {
	for name, fn := range handelTaskMap {
		if err := task.ValidateTask(fn); err != nil {
			return err
		}
		s.registeredTasks.Store(name, fn)
		s.controller.RegisterTask(name)
	}
	return nil
}

// RegisteredTask 注册多个任务
// interface 规则: 必须是func且必须有返回参数,最后一个出参是error
func (s *Server) RegisteredTask(name string, fn interface{}) error {
	if err := task.ValidateTask(fn); err != nil {
		return err
	}
	s.registeredTasks.Store(name, fn)
	s.controller.RegisterTask(name)
	return nil
}

// IsRegisteredTask 判断任务是否注册
func (s *Server) IsRegisteredTask(name string) bool {
	_, ok := s.registeredTasks.Load(name)
	return ok
}

// GetRegisteredTask 获取注册的任务
func (s *Server) GetRegisteredTask(name string) (interface{}, bool) {
	return s.registeredTasks.Load(name)
}

// SendTaskWithContext 发送任务,可以传入ctx
func (s *Server) SendTaskWithContext(ctx context.Context, signature *task.Signature) (*result.AsyncResult, error) {
	attemptBackend, supportsAttemptCompensation := s.backend.(backend.PublicationAttemptBackend)
	var attemptID string
	var err error
	if supportsAttemptCompensation {
		attemptID, err = s.nextPublicationAttemptID()
		if err != nil {
			return nil, fmt.Errorf("generate publication attempt ID: %w", err)
		}
		err = attemptBackend.SetStatePendingAttempt(signature, attemptID)
	} else {
		err = s.backend.SetStatePending(signature)
	}
	if err != nil {
		return nil, errors.Wrap(err, "set state pending")
	}
	// 是否预处理
	if s.prePublishHandler != nil {
		s.prePublishHandler(signature)
	}
	s.bindDefaultRouter(signature)
	// 任务发布
	if err := s.controller.Publish(ctx, signature); err != nil {
		return nil, s.withTaskPublicationCompensation(signature, attemptBackend, attemptID, err)
	}
	return result.NewAsyncResult(signature, s.backend), nil
}

func (s *Server) withTaskPublicationCompensation(
	signature *task.Signature,
	attemptBackend backend.PublicationAttemptBackend,
	attemptID string,
	publishErr error,
) error {
	primaryErr := fmt.Errorf("publish task %s: %w", signature.ID, publishErr)
	if attemptBackend == nil {
		return primaryErr
	}
	if _, err := attemptBackend.FailPendingAttempt(signature, attemptID, taskPublicationFailureMessage); err != nil {
		return stderrors.Join(
			primaryErr,
			fmt.Errorf("converge task %s after publication failure: %w", signature.ID, err),
		)
	}
	return primaryErr
}

// SendTask 发送任务
func (s *Server) SendTask(signature *task.Signature) (*result.AsyncResult, error) {
	return s.SendTaskWithContext(context.Background(), signature)
}

// SendChain 发送链式调用任务
func (s *Server) SendChain(chain *task.Chain) (*result.ChainAsyncResult, error) {
	_, err := s.SendTask(chain.Tasks[0])
	if err != nil {
		return nil, err
	}
	return result.NewChainAsyncResult(chain.Tasks, s.backend), nil
}

// SendGroupWithContext 发送并行执行的任务组
func (s *Server) SendGroupWithContext(ctx context.Context, group *task.Group, concurrency int) ([]*result.AsyncResult, error) {
	if concurrency < 1 {
		concurrency = 1
	}
	asyncResults := make([]*result.AsyncResult, len(group.Tasks))
	if err := ctx.Err(); err != nil {
		return asyncResults, err
	}

	// 接管任务
	if err := s.backend.GroupTakeOver(group.GroupID, group.Name, group.GetTaskIDs()...); err != nil {
		return nil, err
	}

	// Publish only after every task has a durable pending state. If setup fails,
	// roll back the group and the states that were already initialized so the
	// caller can retry the same group ID immediately.
	initializedTaskIDs := make([]string, 0, len(group.Tasks))
	for _, signature := range group.Tasks {
		if err := ctx.Err(); err != nil {
			return asyncResults, s.withGroupInitializationCleanup(err, group.GroupID, initializedTaskIDs)
		}
		if err := s.backend.SetStatePending(signature); err != nil {
			primaryErr := errors.Wrapf(err, "set state pending task %s", signature.ID)
			return asyncResults, s.withGroupInitializationCleanup(primaryErr, group.GroupID, initializedTaskIDs)
		}
		initializedTaskIDs = append(initializedTaskIDs, signature.ID)
	}
	for _, signature := range group.Tasks {
		s.bindDefaultRouter(signature)
	}

	publishErrs := make([]error, len(group.Tasks))
	pool := make(chan struct{}, concurrency)
	var wg sync.WaitGroup
	startedPublishers := 0

	markCanceled := func(start int, err error) {
		for index := start; index < len(group.Tasks); index++ {
			publishErrs[index] = errors.Wrapf(err, "publish task %s", group.Tasks[index].ID)
		}
	}

admission:
	for index, signature := range group.Tasks {
		if err := ctx.Err(); err != nil {
			markCanceled(index, err)
			break
		}
		select {
		case pool <- struct{}{}:
		case <-ctx.Done():
			markCanceled(index, ctx.Err())
			break admission
		}
		// Cancellation can race with acquiring the admission slot. Recheck before
		// declaring this publisher started, and leave all later slots untouched.
		if err := ctx.Err(); err != nil {
			<-pool
			markCanceled(index, err)
			break
		}

		wg.Add(1)
		startedPublishers++
		go func(t *task.Signature, index int) {
			defer wg.Done()
			defer func() { <-pool }()
			if err := s.controller.Publish(ctx, t); err != nil {
				publishErrs[index] = errors.Wrapf(err, "publish task %s", t.ID)
				return
			}
			asyncResults[index] = result.NewAsyncResult(t, s.backend)
		}(signature, index)
	}

	// The result and error slices are caller-owned after return, so every writer
	// must be joined before either slice is inspected or returned.
	wg.Wait()
	var primaryErr error
	for _, err := range publishErrs {
		if err != nil {
			primaryErr = err
			break
		}
	}
	if primaryErr != nil {
		// No call to Publish means no queue side effect can have occurred, so the
		// complete initialization is safe to roll back and the caller can retry
		// the same group ID. Once a publish attempt starts, retain the group and
		// converge every unsuccessful or unadmitted member to a terminal failure;
		// confirmed successful publishers are never touched.
		if startedPublishers == 0 {
			return asyncResults, s.withGroupInitializationCleanup(primaryErr, group.GroupID, initializedTaskIDs)
		}
		if convergenceErr := s.convergeGroupPublicationFailures(group, publishErrs); convergenceErr != nil {
			return asyncResults, stderrors.Join(primaryErr, convergenceErr)
		}
		return asyncResults, primaryErr
	}
	if err := ctx.Err(); err != nil {
		if startedPublishers == 0 {
			return asyncResults, s.withGroupInitializationCleanup(err, group.GroupID, initializedTaskIDs)
		}
		return asyncResults, err
	}
	return asyncResults, nil
}

func (s *Server) convergeGroupPublicationFailures(group *task.Group, publishErrs []error) error {
	var convergenceErrs []error
	for index, publishErr := range publishErrs {
		if publishErr == nil {
			continue
		}
		failureMessage := "group publication failed before task execution"
		if stderrors.Is(publishErr, context.Canceled) || stderrors.Is(publishErr, context.DeadlineExceeded) {
			failureMessage = "group publication canceled before task execution"
		}
		if err := s.backend.SetStateFailure(group.Tasks[index], failureMessage); err != nil {
			convergenceErrs = append(convergenceErrs, fmt.Errorf(
				"converge group task %s after publication failure: %w",
				group.Tasks[index].ID,
				err,
			))
		}
	}
	return stderrors.Join(convergenceErrs...)
}

func (s *Server) withGroupInitializationCleanup(primaryErr error, groupID string, taskIDs []string) error {
	var resetTaskErr error
	if len(taskIDs) > 0 {
		resetTaskErr = s.backend.ResetTask(taskIDs...)
	}
	resetGroupErr := s.backend.ResetGroup(groupID)
	errs := []error{primaryErr}
	if resetTaskErr != nil {
		errs = append(errs, fmt.Errorf("reset initialized tasks: %w", resetTaskErr))
	}
	if resetGroupErr != nil {
		errs = append(errs, fmt.Errorf("reset group: %w", resetGroupErr))
	}
	return stderrors.Join(errs...)
}

// SendGroup 发送并行任务组
func (s *Server) SendGroup(group *task.Group, concurrency int) ([]*result.AsyncResult, error) {
	return s.SendGroupWithContext(context.Background(), group, concurrency)
}

// SendGroupCallbackWithContext 发送具有回调任务的任务组
func (s *Server) SendGroupCallbackWithContext(ctx context.Context, groupCallback *task.GroupCallback, concurrency int) (*result.GroupCallbackAsyncResult, error) {
	_, err := s.SendGroupWithContext(ctx, groupCallback.Group, concurrency)
	if err != nil {
		return nil, err
	}
	return result.NewGroupCallbackAsyncResult(groupCallback.Group.Tasks, groupCallback.Callback, s.backend), nil
}

// SendGroupCallback 发送具有回调任务的任务组
func (s *Server) SendGroupCallback(groupCallback *task.GroupCallback, concurrency int) (*result.GroupCallbackAsyncResult, error) {
	return s.SendGroupCallbackWithContext(context.Background(), groupCallback, concurrency)
}

// RegisteredTimedTask 注册定时任务
func (s *Server) RegisteredTimedTask(spec, name string, signature *task.Signature) error {
	// 检查spec是否合法
	schedule, err := cron.ParseStandard(spec)
	if err != nil {
		return err
	}
	f := func() {
		key := getLockName(name, spec)
		// Each fire generates its own random mark so that this instance's
		// UnLock cannot release a lock that another instance happened to
		// acquire under the same key after our TTL expired. Previously the
		// mark was the deterministic key string itself, so any instance
		// could steal any other's lock.
		mark := rand_string.RandomLetter(16)
		ttl := timedTaskLockTTL(time.Until(schedule.Next(time.Now())))
		err := s.lock.Lock(key, ttl, mark)
		if err != nil {
			return
		}
		defer s.lock.UnLock(key, mark)

		// send task
		_, err = s.SendTask(task.CopySignature(signature))
		if err != nil {
			s.helper.Errorf("timed task failed. task name is: %s. error is %s", name, err.Error())
		}
	}
	_, err = s.scheduler.AddFunc(spec, f)
	return err
}

// RegisteredTimedChain 注册定时链式任务
func (s *Server) RegisteredTimedChain(spec, name string, signatures ...*task.Signature) error {
	// 检查spec是否合法
	schedule, err := cron.ParseStandard(spec)
	if err != nil {
		return err
	}
	f := func() {
		chain, _ := task.NewChain(name, task.CopySignatures(signatures...)...)

		// get lock
		key := getLockName(name, spec)
		mark := rand_string.RandomLetter(16)
		ttl := timedTaskLockTTL(time.Until(schedule.Next(time.Now())))
		err := s.lock.Lock(key, ttl, mark)
		if err != nil {
			return
		}
		defer s.lock.UnLock(key, mark)

		// send task
		_, err = s.SendChain(chain)
		if err != nil {
			s.helper.Errorf("timed task failed. task name is: %s. error is %s", name, err.Error())
		}
	}
	_, err = s.scheduler.AddFunc(spec, f)
	return err
}

// RegisteredTimedGroup 注册定时任务组
func (s *Server) RegisteredTimedGroup(spec, name string, groupID string, concurrency int, signatures ...*task.Signature) error {
	// 检查spec是否合法
	schedule, err := cron.ParseStandard(spec)
	if err != nil {
		return err
	}
	f := func() {
		runSuffix := rand_string.RandomLetter(timedRunSuffixLength)
		group := newTimedGroupRun(groupID, name, runSuffix, signatures...)
		// get lock
		key := getLockName(name, spec)
		mark := rand_string.RandomLetter(16)
		ttl := timedTaskLockTTL(time.Until(schedule.Next(time.Now())))
		err := s.lock.Lock(key, ttl, mark)
		if err != nil {
			return
		}
		defer s.lock.UnLock(key, mark)

		_, err = s.SendGroup(group, concurrency)
		if err != nil {
			s.helper.Errorf("timed task failed. task name is: %s. error is %s", name, err.Error())
		}
	}
	_, err = s.scheduler.AddFunc(spec, f)
	return err
}

// RegisteredTimedGroupCallback 注册具有回调的组任务
func (s *Server) RegisteredTimedGroupCallback(spec, name string, groupID string, concurrency int, callback *task.Signature, signatures ...*task.Signature) error {
	// 检查spec是否合法
	schedule, err := cron.ParseStandard(spec)
	if err != nil {
		return err
	}
	f := func() {
		runSuffix := rand_string.RandomLetter(timedRunSuffixLength)
		c := newTimedGroupCallbackRun(groupID, name, runSuffix, callback, signatures...)
		// get lock
		key := getLockName(name, spec)
		mark := rand_string.RandomLetter(16)
		ttl := timedTaskLockTTL(time.Until(schedule.Next(time.Now())))
		err := s.lock.Lock(key, ttl, mark)
		if err != nil {
			return
		}
		defer s.lock.UnLock(key, mark)

		_, err = s.SendGroupCallback(c, concurrency)
		if err != nil {
			s.helper.Errorf("timed task failed. task name is: %s. error is %s", name, err.Error())
		}
	}
	_, err = s.scheduler.AddFunc(spec, f)
	return err
}

func newTimedGroupRun(groupID, name, runSuffix string, signatures ...*task.Signature) *task.Group {
	runtimeSignatures := task.CopySignatures(signatures...)
	for index, signature := range runtimeSignatures {
		rekeyTimedSignature(signature, runSuffix, fmt.Sprintf("task-%d", index), make(map[*task.Signature]struct{}))
	}
	group, _ := task.NewGroup(groupID+":"+runSuffix, name, runtimeSignatures...)
	return group
}

func newTimedGroupCallbackRun(groupID, name, runSuffix string, callback *task.Signature, signatures ...*task.Signature) *task.GroupCallback {
	group := newTimedGroupRun(groupID, name, runSuffix, signatures...)
	var runtimeCallback *task.Signature
	if callback != nil {
		runtimeCallback = task.CopySignature(callback)
		rekeyTimedSignature(runtimeCallback, runSuffix, "callback", make(map[*task.Signature]struct{}))
	}
	groupCallback, _ := task.NewGroupCallback(group, name, runtimeCallback)
	return groupCallback
}

func rekeyTimedSignature(signature *task.Signature, runSuffix, path string, visited map[*task.Signature]struct{}) {
	if signature == nil {
		return
	}
	if _, ok := visited[signature]; ok {
		return
	}
	visited[signature] = struct{}{}
	signature.ID = fmt.Sprintf("%s:%s:%s", signature.ID, runSuffix, path)
	for index, callback := range signature.CallbackOnSuccess {
		rekeyTimedSignature(callback, runSuffix, fmt.Sprintf("%s.success-%d", path, index), visited)
	}
	for index, callback := range signature.CallbackOnError {
		rekeyTimedSignature(callback, runSuffix, fmt.Sprintf("%s.error-%d", path, index), visited)
	}
	rekeyTimedSignature(signature.CallbackChord, runSuffix, path+".chord", visited)
}

func getLockName(name, spec string) string {
	return name + spec
}

// NewServer 创建服务
func NewServer(controller controller.Controller, backend backend.Backend, lock locker.Locker, helper *log.Helper, prePublishHandler func(task *task.Signature), options ...options.Option) *Server {
	server := &Server{
		config: &Config{
			NoUnixSignals: false,
			ResultExpire:  0,
			Concurrency:   1,
			ConsumeQueue:  "consume_queue",
			DelayedQueue:  "delayed_queue",
		},
		registeredTasks:   &sync.Map{},
		controller:        controller,
		backend:           backend,
		lock:              lock,
		scheduler:         cron.New(),
		prePublishHandler: prePublishHandler,
		helper:            helper,
	}
	for _, option := range options {
		option(server.config)
	}
	server.EnforcementConf()
	go server.scheduler.Run()
	return server
}

func (s *Server) EnforcementConf() {
	s.backend.SetResultExpire(s.config.ResultExpire)
	s.controller.SetConsumingQueue(s.config.ConsumeQueue)
	s.controller.SetDelayedQueue(s.config.DelayedQueue)
}

// Shutdown stops the scheduler started by NewServer and waits up to the
// supplied context's deadline for any in-flight cron jobs to finish. Without
// this, the goroutine launched by `go server.scheduler.Run()` survives until
// process exit, holding timers for every registered job.
func (s *Server) Shutdown(ctx context.Context) error {
	stopCtx := s.scheduler.Stop()
	select {
	case <-stopCtx.Done():
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (s *Server) NewWorker(consumerTag string, concurrency int, queue string) *Worker {
	return &Worker{
		bindService: s,
		Concurrency: concurrency,
		ConsumerTag: consumerTag,
		Queue:       queue,
	}
}
