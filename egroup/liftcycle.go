package egroup

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/songzhibin97/gkit/goroutine"
	"github.com/songzhibin97/gkit/options"
)

// LifeAdminer 生命周期管理接口
type LifeAdminer interface {
	Start(ctx context.Context) error
	Shutdown(ctx context.Context) error
}

// Member 成员
type Member struct {
	Start    func(ctx context.Context) error
	Shutdown func(ctx context.Context) error
}

type memberStartError struct {
	err error
}

func (e *memberStartError) Error() string { return e.err.Error() }
func (e *memberStartError) Unwrap() error { return e.err }

// LifeAdmin 生命周期管理
type LifeAdmin struct {
	opts     *config
	members  []Member
	shutdown func()
	g        *Group
}

// Add 添加成员表(通过内部 Member 对象添加)
func (l *LifeAdmin) Add(member Member) {
	l.members = append(l.members, member)
}

// AddMember 添加程序表(通过外部接口 LifeAdminer 添加)
func (l *LifeAdmin) AddMember(la LifeAdminer) {
	l.members = append(l.members, Member{
		Start:    la.Start,
		Shutdown: la.Shutdown,
	})
}

// Start 启动
func (l *LifeAdmin) Start() error {
	// Register every shutdown and signal controller before a member Start
	// can occupy the supplied pool's worker capacity.
	for _, m := range l.members {
		func(m Member) {
			// 如果有shutdown进行注册
			if m.Shutdown != nil {
				l.goController(func() error {
					// 等待异常或信号关闭触发
					<-l.g.ctx.Done()
					return goroutine.Delegate(context.Background(), l.opts.stopTimeout, m.Shutdown)
				})
			}
		}(m)
	}
	if len(l.opts.signals) > 0 && l.opts.handler != nil {
		c := make(chan os.Signal, len(l.opts.signals))
		signal.Notify(c, l.opts.signals...)
		// Stop notification even if the pool rejects the controller launch.
		defer signal.Stop(c)
		l.goController(func() error {
			defer signal.Stop(c)
			for {
				select {
				case <-l.g.ctx.Done():
					return nil
				case sig := <-c:
					l.opts.handler(l, sig)
				}
			}
		})
	}
	for _, m := range l.members {
		func(m Member) {
			if m.Start != nil {
				l.g.Go(func() error {
					err := goroutine.Delegate(l.g.ctx, l.opts.startTimeout, func(ctx context.Context) error {
						if err := m.Start(ctx); err != nil {
							return &memberStartError{err: err}
						}
						return nil
					})
					var memberErr *memberStartError
					if errors.As(err, &memberErr) {
						return memberErr.err
					}
					if errors.Is(err, context.Canceled) && l.g.ctx.Err() != nil {
						return nil
					}
					return err
				})
			}
		}(m)
	}
	return l.g.Wait()
}

// goController uses the pool only to launch a tracked shutdown or signal
// waiter. Their count is bounded by the registered members plus one signal
// watcher. Member Start callbacks retain Group.Go's pool concurrency limit.
func (l *LifeAdmin) goController(f func() error) {
	g := l.g
	g.stateMu.Lock()
	if g.closed {
		g.stateMu.Unlock()
		return
	}
	g.wg.Add(1)
	g.stateMu.Unlock()
	accepted := g.goroutine.AddTaskN(g.ctx, func() {
		go func() {
			defer g.wg.Done()
			completed := false
			defer func() {
				value := recover()
				// On Go 1.20, panic(nil) also makes recover return nil.
				if !completed {
					err, ok := value.(error)
					if !ok {
						err = fmt.Errorf("lifecycle panic: %v", value)
					}
					g.recordError(err)
				}
			}()
			g.recordError(f())
			completed = true
		}()
	})
	if !accepted {
		g.recordError(ErrGroupClosed)
		g.wg.Done()
	}
}

// Shutdown 停止服务
func (l *LifeAdmin) Shutdown() {
	if l.shutdown != nil {
		l.shutdown()
	}
}

// NewLifeAdmin 实例化方法
func NewLifeAdmin(opts ...options.Option) *LifeAdmin {
	// 默认参数
	o := &config{
		startTimeout: startTimeout,
		stopTimeout:  stopTimeout,
		signals:      []os.Signal{syscall.SIGTERM, syscall.SIGQUIT, syscall.SIGINT},
		handler: func(a *LifeAdmin, signal os.Signal) {
			switch signal {
			case syscall.SIGTERM, syscall.SIGQUIT, syscall.SIGINT:
				a.Shutdown()
			default:
			}
		},
	}
	// 选项模式填充参数
	for _, opt := range opts {
		opt(o)
	}

	l := &LifeAdmin{opts: o}

	if o.g == nil {
		ctx, cancel := context.WithCancel(context.Background())
		o.g = WithContext(ctx)
		l.shutdown = cancel
	} else {
		l.shutdown = o.g.cancel
	}
	l.g = o.g

	return l
}
