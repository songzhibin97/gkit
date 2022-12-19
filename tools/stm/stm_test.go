package stm

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

type testS struct {
	S1 string `stm:"S_1"  stm2:"-"`
	s2 string `stm:"s_2"`
	T1 struct {
		Name string `stm:"name"`
		age  int    `stm:"age" stm2:"-"`
	} `stm:"T_1"`
	t2 struct {
		Gender bool `stm:"gender"`
	} `stm2:"-"`
	List1 []string `stm:"list1"  stm2:"-"`
	list2 []int    `stm:"list2"`
}

func Test_StructToMapExtraExport(t *testing.T) {
	ts := testS{
		S1: "s1",
		s2: "s2",
		T1: struct {
			Name string `stm:"name"`
			age  int    `stm:"age" stm2:"-"`
		}{
			Name: "name",
			age:  18,
		},
		t2: struct {
			Gender bool `stm:"gender"`
		}{
			Gender: true,
		},
		List1: []string{"1", "2", "3"},
		list2: []int{1, 2, 3},
	}
	type args struct {
		tag string
	}
	tests := []struct {
		name string
		args args
		want map[string]interface{}
	}{
		{
			name: "",
			args: args{
				tag: "",
			},
			want: map[string]interface{}{
				"S1": "s1",
				"s2": "s2",
				"T1": map[string]interface{}{
					"Name": "name",
					"age":  18,
				},
				"t2": map[string]interface{}{
					"Gender": true,
				},
				"List1": []string{"1", "2", "3"},
				"list2": []int{1, 2, 3},
			},
		},
		{
			name: "",
			args: args{
				tag: "stm",
			},
			want: map[string]interface{}{
				"S_1": "s1",
				"s_2": "s2",
				"T_1": map[string]interface{}{
					"name": "name",
					"age":  18,
				},
				"t2": map[string]interface{}{
					"gender": true,
				},
				"list1": []string{"1", "2", "3"},
				"list2": []int{1, 2, 3},
			},
		},
		{
			name: "",
			args: args{
				tag: "stm2",
			},
			want: map[string]interface{}{
				"s2": "s2",
				"T1": map[string]interface{}{
					"Name": "name",
				},
				"list2": []int{1, 2, 3},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, StructToMapExtraExport(ts, tt.args.tag), tt.want)
		})
	}
}

func Test_StructToMap(t *testing.T) {
	ts := testS{
		S1: "s1",
		s2: "s2",
		T1: struct {
			Name string `stm:"name"`
			age  int    `stm:"age" stm2:"-"`
		}{
			Name: "name",
			age:  18,
		},
		t2: struct {
			Gender bool `stm:"gender"`
		}{
			Gender: true,
		},
		List1: []string{"1", "2", "3"},
		list2: []int{1, 2, 3},
	}
	type args struct {
		tag string
	}
	tests := []struct {
		name string
		args args
		want map[string]interface{}
	}{
		{
			name: "",
			args: args{
				tag: "",
			},
			want: map[string]interface{}{
				"S1": "s1",
				"T1": map[string]interface{}{
					"Name": "name",
				},
				"List1": []string{"1", "2", "3"},
			},
		},
		{
			name: "",
			args: args{
				tag: "stm",
			},
			want: map[string]interface{}{
				"S_1": "s1",
				"T_1": map[string]interface{}{
					"name": "name",
				},
				"list1": []string{"1", "2", "3"},
			},
		},
		{
			name: "",
			args: args{
				tag: "stm2",
			},
			want: map[string]interface{}{
				"T1": map[string]interface{}{
					"Name": "name",
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, StructToMap(ts, tt.args.tag), tt.want)
		})
	}
}
