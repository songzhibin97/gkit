package group

import (
	"github.com/stretchr/testify/assert"
	"reflect"
	"strconv"
	"testing"
)

func TestGroup_Get(t *testing.T) {
	count := 0
	g := NewGroup(func() interface{} {
		count++
		return count
	})
	type args struct {
		key string
	}
	tests := []struct {
		name string
		args args
		want interface{}
	}{
		{
			"t1", args{key: "user"}, 1,
		},
		{
			"t2", args{key: "avatar"}, 2,
		},
		{
			"t2", args{key: "user"}, 1,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := g.Get(tt.args.key); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Get() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGroup_ReSet(t *testing.T) {
	g := NewGroup(func() interface{} {
		return 1
	})
	v := g.Get("user")
	assert.Equal(t, v.(int), 1)
	g.ReSet(func() interface{} {
		return 2
	})
	v = g.Get("user")
	assert.Equal(t, v.(int), 2)
}

func BenchmarkGroup_Get(b *testing.B) {
	g := NewGroup(func() interface{} {
		return 1
	})
	for i := 0; i < b.N; i++ {
		for j := 0; j < 100; j++ {
			g.Get(strconv.Itoa(j))
		}
	}
}
