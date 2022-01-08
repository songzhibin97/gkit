package task

import (
	"time"

	"github.com/songzhibin97/gkit/options"
)

// SetGroupID 设置多群组中id
func SetGroupID(id string) options.Option {
	return func(t interface{}) {
		t.(*Task).GroupID = id
	}
}

// SetPriority 设置任务优先级
func SetPriority(priority uint8) options.Option {
	return func(t interface{}) {
		t.(*Task).Priority = priority
	}
}

// SetRetryCount 设置重试次数
func SetRetryCount(count int) options.Option {
	return func(t interface{}) {
		t.(*Task).RetryCount = count
	}
}

// SetRetryInterval 设置间隔时间
func SetRetryInterval(interval int) options.Option {
	return func(t interface{}) {
		t.(*Task).RetryInterval = interval
	}
}

// SetStopTaskDeletionOnError 设置任务出错后是否删除
func SetStopTaskDeletionOnError(deleteOnErr bool) options.Option {
	return func(t interface{}) {
		t.(*Task).StopTaskDeletionOnError = deleteOnErr
	}
}

// SetIgnoreNotRegisteredTask 设置任务未注册是否忽略
func SetIgnoreNotRegisteredTask(register bool) options.Option {
	return func(t interface{}) {
		t.(*Task).IgnoreNotRegisteredTask = register
	}
}

// SetRouter 设置路由
func SetRouter(router string) options.Option {
	return func(t interface{}) {
		t.(*Task).Router = router
	}
}

// SetArgs 设置参数
func SetArgs(args ...Arg) options.Option {
	return func(t interface{}) {
		t.(*Task).Args = args
	}
}

// AddArgs 追加参数
func AddArgs(args ...Arg) options.Option {
	return func(t interface{}) {
		t.(*Task).Args = append(t.(*Task).Args, args...)
	}
}

// SetMetaSafe 设置是否创建安全的meta
func SetMetaSafe(safe bool) options.Option {
	return func(t interface{}) {
		t.(*Task).MetaSafe = safe
	}
}

// SetMeta 设置Meta
func SetMeta(meta *Meta) options.Option {
	return func(t interface{}) {
		t.(*Task).Meta = meta
	}
}

// SetETATime 延时任务设置执行时间
func SetETATime(after *time.Time) options.Option {
	return func(t interface{}) {
		t.(*Task).ETA = after
	}
}

// SetCallbackOnSuccess 设置成功后回调
func SetCallbackOnSuccess(tasks ...*Task) options.Option {
	return func(t interface{}) {
		t.(*Task).CallbackOnSuccess = tasks
	}
}

// AddCallbackOnError 追加失败后回调
func AddCallbackOnError(tasks ...*Task) options.Option {
	return func(t interface{}) {
		t.(*Task).CallbackOnError = append(t.(*Task).CallbackOnError, tasks...)
	}
}

// SetCallbackOnError 设置失败后回调
func SetCallbackOnError(tasks ...*Task) options.Option {
	return func(t interface{}) {
		t.(*Task).CallbackOnError = tasks
	}
}

// AddCallbackOnSuccess 追加成功后回调
func AddCallbackOnSuccess(tasks ...*Task) options.Option {
	return func(t interface{}) {
		t.(*Task).CallbackOnSuccess = append(t.(*Task).CallbackOnSuccess, tasks...)
	}
}

// SetTriggerChord .
func SetTriggerChord(task *Task) options.Option {
	return func(t interface{}) {
		t.(*Task).TriggerChord = task
	}
}
