package task

import (
	"context"
	"errors"
	"reflect"
)

type signatureCtxType struct{}

var (
	ErrDispatching = errors.New("dispatch task err")
	signatureCtx   signatureCtxType
)

type Task struct {
	// TaskFunc 执行任务的函数
	TaskFunc reflect.Value
	// UseContext 是否使用上下文
	UseContext bool
	// Context 上下文信息
	Context context.Context
	// Args 执行任务需要的参数
	Args []reflect.Value
}

// TransformArgs 将[]Arg转化为[]reflect.Value
func (t *Task) TransformArgs(args []Arg) error {
	argValues := make([]reflect.Value, len(args))

	for i, arg := range args {
		argValue, err := ReflectValue(arg.Type, arg.Value)
		if err != nil {
			return err
		}
		argValues[i] = argValue
	}

	t.Args = argValues
	return nil
}

// Call 调用方法
func (t *Task) Call() (taskResults []*Result, err error) {
	// 防止意外panic
	defer func() {
		if e := recover(); e != nil {
			switch er := e.(type) {
			default:
				err = ErrDispatching
			case error:
				err = er
			case string:
				err = errors.New(er)
			}
		}
	}()

	args := t.Args
	if t.UseContext {
		ctxValue := reflect.ValueOf(t.Context)
		args = append([]reflect.Value{ctxValue}, args...)
	}

	// 调用任务
	results := t.TaskFunc.Call(args)

	if len(results) == 0 {
		return nil, ErrTaskReturnNoValue
	}
	// 按照规定最后一个参数是 err
	lastResult := results[len(results)-1]
	if !lastResult.IsNil() {
		// err 不为nil

		// 如果该错误实现了 Retrievable 接口
		if lastResult.Type().Implements(retrievableInterface) {
			return nil, lastResult.Interface().(ErrRetryTaskLater)
		}
		// 如果该错实现了 error 接口
		if lastResult.Type().Implements(errInterface) {
			return nil, lastResult.Interface().(error)
		}
		// 如果最后一个返回值没有满足error接口
		return nil, ErrTaskReturnNoErr
	}
	taskResults = make([]*Result, 0, len(results)-1)
	for i := 0; i < len(results)-1; i++ {
		val := results[i].Interface()
		typeStr := reflect.TypeOf(val).String()
		taskResults = append(taskResults, &Result{
			Type:  typeStr,
			Value: val,
		})
	}
	return taskResults, err
}

// SignatureFromContext 获取上下文任务签名
func SignatureFromContext(ctx context.Context) *Signature {
	if ctx == nil {
		return nil
	}
	v := ctx.Value(signatureCtx)
	if v == nil {
		return nil
	}
	signature, _ := v.(*Signature)
	return signature
}

// NewTaskWithSignature 初始化Task通过Signature
func NewTaskWithSignature(taskFunc interface{}, signature *Signature) (*Task, error) {
	ctx := context.WithValue(context.Background(), signatureCtx, signature)
	task := &Task{
		TaskFunc: reflect.ValueOf(taskFunc),
		Context:  ctx,
	}
	taskFuncType := reflect.TypeOf(taskFunc)
	if taskFuncType.NumIn() > 0 {
		if taskFuncType.In(0) == ctxTypeInterface {
			task.UseContext = true
		}
	}
	if err := task.TransformArgs(signature.Args); err != nil {
		return nil, err
	}
	return task, nil
}

// NewTask 初始化Task
func NewTask(taskFunc interface{}, args []Arg) (*Task, error) {
	task := &Task{
		TaskFunc: reflect.ValueOf(taskFunc),
		Context:  context.Background(),
	}
	taskFuncType := reflect.TypeOf(taskFunc)
	if taskFuncType.NumIn() > 0 {
		if taskFuncType.In(0) == ctxTypeInterface {
			task.UseContext = true
		}
	}
	if err := task.TransformArgs(args); err != nil {
		return nil, err
	}
	return task, nil
}
