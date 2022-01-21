package task

import (
	"errors"
	"reflect"
)

var (
	ErrTaskMustFunc      = errors.New("task type must func")
	ErrTaskReturnNoValue = errors.New("task return is no value")
	ErrTaskReturnNoErr   = errors.New("task return last values is must be error")
)

func ValidateTask(task interface{}) error {
	v := reflect.ValueOf(task)
	t := v.Type()
	if t.Kind() != reflect.Func {
		return ErrTaskMustFunc
	}

	if t.NumOut() < 1 {
		return ErrTaskReturnNoValue
	}
	lastReturnType := t.Out(t.NumOut() - 1)
	errorInterface := reflect.TypeOf((*error)(nil)).Elem()
	if !lastReturnType.Implements(errorInterface) {
		return ErrTaskReturnNoErr
	}
	return nil
}
