package group

import (
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGroupGet(t *testing.T) {
	count := 0
	g := NewGroup(func() interface{} {
		count++
		return count
	})
	v := g.Get("user")
	assert.Equal(t, 1, v.(int))

	v = g.Get("avatar")
	assert.Equal(t, 2, v.(int))

	v = g.Get("user")
	assert.Equal(t, 1, v.(int))
	assert.Equal(t, 2, count)
}

func TestGroupReset(t *testing.T) {
	g := NewGroup(func() interface{} {
		return 1
	})
	g.Get("user")
	call := false
	g.ReSet(func() interface{} {
		call = true
		return 1
	})

	length := 0
	for range g.(*Group).objs {
		length++
	}

	assert.Equal(t, 0, length)

	g.Get("user")
	assert.Equal(t, true, call)
}

func TestGroupClear(t *testing.T) {
	g := NewGroup(func() interface{} {
		return 1
	})
	g.Get("user")
	length := 0
	for range g.(*Group).objs {
		length++
	}
	assert.Equal(t, 1, length)

	g.Clear()
	length = 0
	for range g.(*Group).objs {
		length++
	}
	assert.Equal(t, 0, length)
}

func BenchmarkGroupGet(b *testing.B) {
	g := NewGroup(func() interface{} {
		return 1
	})
	for i := 0; i < b.N; i++ {
		g.Get(strconv.Itoa(i))
	}
}
