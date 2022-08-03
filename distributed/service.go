package distributed

import (
	"context"
	"sync"
	"time"

	"github.com/songzhibin97/gkit/options"

	"github.com/songzhibin97/gkit/log"

	"github.com/pkg/errors"

	"github.com/songzhibin97/gkit/distributed/backend/result"

	"github.com/songzhibin97/gkit/distributed/task"

	"github.com/robfig/cron/v3"

	"github.com/songzhibin97/gkit/distributed/backend"
	"github.com/songzhibin97/gkit/distributed/controller"
	"github.com/songzhibin97/gkit/distributed/locker"
)

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
	// 设置任务状态为pending
	if err := s.backend.SetStatePending(signature); err != nil {
		return nil, errors.Wrap(err, "set state pending")
	}
	// 是否预处理
	if s.prePublishHandler != nil {
		s.prePublishHandler(signature)
	}
	// 任务发布
	if err := s.controller.Publish(ctx, signature); err != nil {
		return nil, errors.Wrap(err, "publish err")
	}
	return result.NewAsyncResult(signature, s.backend), nil
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
	if concurrency < 0 {
		concurrency = 1
	}
	var (
		asyncResults = make([]*result.AsyncResult, len(group.Tasks))
		wg           sync.WaitGroup
		ln           = len(group.Tasks)
		errChan      = make(chan error, ln*2)
		pool         = make(chan struct{}, concurrency)
		done         = make(chan struct{})
	)

	// 接管任务
	err := s.backend.GroupTakeOver(group.GroupID, group.Name, group.GetTaskIDs()...)
	if err != nil {
		return nil, err
	}

	// 初始化任务
	for _, signature := range group.Tasks {
		if err = s.backend.SetStatePending(signature); err != nil {
			errChan <- err
			continue
		}
	}

	// 初始化并发池
	go func() {
		for i := 0; i < concurrency; i++ {
			pool <- struct{}{}
		}
	}()

	wg.Add(ln)
	// 执行任务
	for i, signature := range group.Tasks {
		<-pool
		go func(t *task.Signature, index int) {
			defer wg.Done()
			// 发布任务
			err := s.controller.Publish(ctx, t)
			pool <- struct{}{}
			if err != nil {
				errChan <- errors.Wrap(err, "set state pending")
				return
			}
			asyncResults[index] = result.NewAsyncResult(t, s.backend)
		}(signature, i)
	}
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-ctx.Done():
		return asyncResults, ctx.Err()
	case err = <-errChan:
		return asyncResults, err
	case <-done:
		return asyncResults, nil
	}
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
		err := s.lock.Lock(key, int(schedule.Next(time.Now()).UnixNano()-1), key)
		if err != nil {
			return
		}
		defer s.lock.UnLock(key, key)

		// send task
		_, err = s.SendTask(task.CopySignature(signature))
		if err != nil {
			s.helper.Errorf("timed task failed. task name is: %s. error is %s\", name, err.Error()\n", name, err.Error())
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
		err := s.lock.Lock(key, int(schedule.Next(time.Now()).UnixNano()-1), key)
		if err != nil {
			return
		}
		defer s.lock.UnLock(key, key)

		// send task
		_, err = s.SendChain(chain)
		if err != nil {
			s.helper.Errorf("timed task failed. task name is: %s. error is %s\", name, err.Error()\n", name, err.Error())
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
		group, _ := task.NewGroup(groupID, name, task.CopySignatures(signatures...)...)
		// get lock
		key := getLockName(name, spec)
		err := s.lock.Lock(key, int(schedule.Next(time.Now()).UnixNano()-1), key)
		if err != nil {
			return
		}
		defer s.lock.UnLock(key, key)

		_, err = s.SendGroup(group, concurrency)
		if err != nil {
			s.helper.Errorf("timed task failed. task name is: %s. error is %s\", name, err.Error()\n", name, err.Error())
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
		group, _ := task.NewGroup(groupID, name, task.CopySignatures(signatures...)...)
		c, _ := task.NewGroupCallback(group, name, callback)
		// get lock
		key := getLockName(name, spec)
		err := s.lock.Lock(key, int(schedule.Next(time.Now()).UnixNano()-1), key)
		if err != nil {
			return
		}
		defer s.lock.UnLock(key, key)

		_, err = s.SendGroupCallback(c, concurrency)
		if err != nil {
			s.helper.Errorf("timed task failed. task name is: %s. error is %s\", name, err.Error()\n", name, err.Error())
		}
	}
	_, err = s.scheduler.AddFunc(spec, f)
	return err
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

func (s *Server) NewWorker(consumerTag string, concurrency int, queue string) *Worker {
	return &Worker{
		bindService: s,
		Concurrency: concurrency,
		ConsumerTag: consumerTag,
		Queue:       queue,
	}
}
