package task

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestTaskCallError(t *testing.T) {
	t.Parallel()
	retrievable := func() error { return NewErrRetryTaskLater("error", time.Minute) }

	task, err := NewTask(retrievable, []Arg{})
	assert.NoError(t, err)

	results, err := task.Call()
	assert.Nil(t, results)
	assert.NotNil(t, err)
	_, ok := interface{}(err).(ErrRetryTaskLater)
	assert.True(t, ok, "err must is ErrRetryTaskLater type")

	errFn := func() error { return errors.New("error") }
	task, err = NewTask(errFn, []Arg{})
	assert.NoError(t, err)
	results, err = task.Call()
	assert.Nil(t, results)
	assert.NotNil(t, err)
	assert.Equal(t, "error", err.Error())
}

func TestTransformArgs(t *testing.T) {
	t.Parallel()
	task := &Task{}
	args := []Arg{
		{
			Type:  "[]int64",
			Value: []int64{1, 2},
		},
	}
	err := task.TransformArgs(args)
	assert.NoError(t, err)
	assert.Equal(t, 1, len(task.Args))
	assert.Equal(t, "[]int64", task.Args[0].Type().String())
}

func TestTaskCallBeyondExpectationErr(t *testing.T) {
	t.Parallel()
	f1 := func(x int) error { return nil }
	args := []Arg{
		{
			Type:  "bool",
			Value: true,
		},
	}
	task, err := NewTask(f1, args)
	assert.NoError(t, err)
	results, err := task.Call()
	assert.Equal(t, "reflect: Call using bool as type int", err.Error())
	assert.Nil(t, results)

	f2 := func() (interface{}, error) { return 1.11, nil }
	task, err = NewTask(f2, []Arg{})
	assert.NoError(t, err)

	results, err = task.Call()
	assert.NoError(t, err)
	assert.Equal(t, "float64", results[0].Type)
	assert.Equal(t, 1.11, results[0].Value)
}

func TestTaskCallWithContext(t *testing.T) {
	t.Parallel()
	f := func(ctx context.Context) (interface{}, error) {
		assert.NotNil(t, ctx)
		assert.Nil(t, SignatureFromContext(ctx))
		return 1.11, nil
	}
	task, err := NewTask(f, []Arg{})
	assert.NoError(t, err)
	results, err := task.Call()
	assert.NoError(t, err)
	assert.Equal(t, "float64", results[0].Type)
	assert.Equal(t, 1.11, results[0].Value)
}

func TestTaskCallWithSignatureInContext(t *testing.T) {
	t.Parallel()

	f := func(ctx context.Context) (interface{}, error) {
		assert.NotNil(t, ctx)
		signature := SignatureFromContext(ctx)
		assert.NotNil(t, signature)
		assert.Equal(t, "bar", signature.Name)
		return 1.11, nil
	}
	signature := NewSignature("", "bar")
	task, err := NewTaskWithSignature(f, signature)
	assert.NoError(t, err)
	results, err := task.Call()
	assert.NoError(t, err)
	assert.Equal(t, "float64", results[0].Type)
	assert.Equal(t, 1.11, results[0].Value)
}
