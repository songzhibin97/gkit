package concurrent

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func generateStreamNumber(n int) (ret []interface{}) {
	for i := 0; i < n; i++ {
		ret = append(ret, i)
	}
	return ret
}

var flagSlice = []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9}

func filter(i interface{}) bool {
	return i.(int)&1 == 0
}

func TestStream(t *testing.T) {
	c := Stream(context.Background(), generateStreamNumber(10)...)
	var ret []int
	for v := range c {
		ret = append(ret, v.(int))
	}
	assert.Equal(t, flagSlice, ret)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	c = Stream(ctx, generateStreamNumber(10)...)
	ret = nil
	for v := range c {
		ret = append(ret, v.(int))
		time.Sleep(time.Second)
	}
	assert.Equal(t, []int{0, 1, 2}, ret)
}

func TestTaskN(t *testing.T) {
	c := Stream(context.Background(), generateStreamNumber(10)...)
	taskN := TaskN(context.Background(), c, 3)
	var ret []int
	for v := range taskN {
		ret = append(ret, v.(int))
	}
	assert.Equal(t, []int{0, 1, 2}, ret)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	c = Stream(context.Background(), generateStreamNumber(10)...)
	taskN = TaskN(ctx, c, 10)
	ret = nil
	for v := range taskN {
		ret = append(ret, v.(int))
		time.Sleep(time.Second)
	}
	assert.Equal(t, []int{0, 1, 2}, ret)
}

func TestTaskFn(t *testing.T) {
	c := Stream(context.Background(), generateStreamNumber(10)...)
	taskFn := TaskFn(context.Background(), c, filter)
	var ret []int
	for v := range taskFn {
		ret = append(ret, v.(int))
	}
	assert.Equal(t, []int{0, 2, 4, 6, 8}, ret)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	c = Stream(context.Background(), generateStreamNumber(10)...)
	taskFn = TaskFn(ctx, c, filter)
	ret = nil
	for v := range taskFn {
		ret = append(ret, v.(int))
		time.Sleep(time.Second)
	}
	assert.Equal(t, []int{0, 2, 4}, ret)
}

func TestTaskWhile(t *testing.T) {
	c := Stream(context.Background(), generateStreamNumber(10)...)
	taskWhile := TaskWhile(context.Background(), c, func(v interface{}) bool {
		return v.(int) == 5
	})
	var ret []int
	for v := range taskWhile {
		ret = append(ret, v.(int))
	}
	assert.Equal(t, []int{5}, ret)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	c = Stream(context.Background(), generateStreamNumber(10)...)
	taskWhile = TaskWhile(ctx, c, func(v interface{}) bool {
		return v.(int) == 5
	})
	ret = nil
	for v := range taskWhile {
		ret = append(ret, v.(int))
		time.Sleep(time.Second)
	}
	assert.Equal(t, []int{5}, ret)
}

func TestSkipN(t *testing.T) {
	c := Stream(context.Background(), generateStreamNumber(10)...)
	skipN := SkipN(context.Background(), c, 3)
	var ret []int
	for v := range skipN {
		ret = append(ret, v.(int))
	}
	assert.Equal(t, []int{3, 4, 5, 6, 7, 8, 9}, ret)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	c = Stream(context.Background(), generateStreamNumber(10)...)
	skipN = SkipN(ctx, c, 3)
	ret = nil
	for v := range skipN {
		ret = append(ret, v.(int))
		time.Sleep(time.Second)
	}
	assert.Equal(t, []int{3, 4, 5}, ret)
}

func TestSkipFn(t *testing.T) {
	c := Stream(context.Background(), generateStreamNumber(10)...)
	skipFn := SkipFn(context.Background(), c, filter)
	var ret []int
	for v := range skipFn {
		ret = append(ret, v.(int))
	}
	assert.Equal(t, []int{1, 3, 5, 7, 9}, ret)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	c = Stream(context.Background(), generateStreamNumber(10)...)
	skipFn = SkipFn(ctx, c, filter)
	ret = nil
	for v := range skipFn {
		ret = append(ret, v.(int))
		time.Sleep(time.Second)
	}
	assert.Equal(t, []int{1, 3, 5}, ret)
}

func TestSkipWhile(t *testing.T) {
	c := Stream(context.Background(), generateStreamNumber(10)...)
	skipWhile := SkipWhile(context.Background(), c, func(v interface{}) bool {
		return v.(int) == 0
	})
	var ret []int
	for v := range skipWhile {
		ret = append(ret, v.(int))
	}
	assert.Equal(t, []int{1, 2, 3, 4, 5, 6, 7, 8, 9}, ret)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	c = Stream(context.Background(), generateStreamNumber(10)...)
	skipWhile = SkipWhile(ctx, c, func(v interface{}) bool {
		return v.(int) == 0
	})
	ret = nil
	for v := range skipWhile {
		ret = append(ret, v.(int))
		time.Sleep(time.Second)
	}
	assert.Equal(t, []int{1, 2, 3}, ret)
}
