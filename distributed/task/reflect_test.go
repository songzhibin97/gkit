package task

import (
	"reflect"
	"testing"
)

func TestReflectValue(t *testing.T) {
	t.Parallel()
	type args struct {
		valueType string
		value     interface{}
	}
	tests := []struct {
		name    string
		args    args
		want    reflect.Value
		wantErr bool
	}{
		{
			name: "bool",
			args: args{
				valueType: "bool",
				value:     true,
			},
			want:    reflect.ValueOf(true),
			wantErr: false,
		},
		{
			name: "int",
			args: args{
				valueType: "int",
				value:     int(0),
			},
			want:    reflect.ValueOf(int(0)),
			wantErr: false,
		},
		{
			name: "int8",
			args: args{
				valueType: "int8",
				value:     int8(0),
			},
			want:    reflect.ValueOf(int8(0)),
			wantErr: false,
		},
		{
			name: "int16",
			args: args{
				valueType: "int16",
				value:     int16(0),
			},
			want:    reflect.ValueOf(int16(0)),
			wantErr: false,
		},
		{
			name: "int32",
			args: args{
				valueType: "int32",
				value:     int32(0),
			},
			want:    reflect.ValueOf(int32(0)),
			wantErr: false,
		},
		{
			name: "int64",
			args: args{
				valueType: "int64",
				value:     int64(0),
			},
			want:    reflect.ValueOf(int64(0)),
			wantErr: false,
		},

		{
			name: "uint",
			args: args{
				valueType: "uint",
				value:     uint(0),
			},
			want:    reflect.ValueOf(uint(0)),
			wantErr: false,
		},
		{
			name: "uint8",
			args: args{
				valueType: "uint8",
				value:     uint8(0),
			},
			want:    reflect.ValueOf(uint8(0)),
			wantErr: false,
		},
		{
			name: "uint16",
			args: args{
				valueType: "uint16",
				value:     uint16(0),
			},
			want:    reflect.ValueOf(uint16(0)),
			wantErr: false,
		},
		{
			name: "uint32",
			args: args{
				valueType: "uint32",
				value:     uint32(0),
			},
			want:    reflect.ValueOf(uint32(0)),
			wantErr: false,
		},
		{
			name: "uint64",
			args: args{
				valueType: "uint64",
				value:     uint64(0),
			},
			want:    reflect.ValueOf(uint64(0)),
			wantErr: false,
		},

		{
			name: "float32",
			args: args{
				valueType: "float32",
				value:     float32(0),
			},
			want:    reflect.ValueOf(float32(0)),
			wantErr: false,
		},
		{
			name: "float64",
			args: args{
				valueType: "float64",
				value:     float64(0),
			},
			want:    reflect.ValueOf(float64(0)),
			wantErr: false,
		},

		{
			name: "string",
			args: args{
				valueType: "string",
				value:     string(""),
			},
			want:    reflect.ValueOf(string("")),
			wantErr: false,
		},

		{
			name: "[]bool",
			args: args{
				valueType: "[]bool",
				value:     make([]bool, 0),
			},
			want:    reflect.ValueOf(make([]bool, 0)),
			wantErr: false,
		},
		{
			name: "[]byte",
			args: args{
				valueType: "[]bool",
				value:     make([]bool, 0),
			},
			want:    reflect.ValueOf(make([]bool, 0)),
			wantErr: false,
		},
		{
			name: "[]rune",
			args: args{
				valueType: "[]bool",
				value:     make([]bool, 0),
			},
			want:    reflect.ValueOf(make([]bool, 0)),
			wantErr: false,
		},

		{
			name: "[]int",
			args: args{
				valueType: "[]int",
				value:     make([]int, 0),
			},
			want:    reflect.ValueOf(make([]int, 0)),
			wantErr: false,
		},
		{
			name: "[]int8",
			args: args{
				valueType: "[]int8",
				value:     make([]int8, 0),
			},
			want:    reflect.ValueOf(make([]int8, 0)),
			wantErr: false,
		},
		{
			name: "[]int16",
			args: args{
				valueType: "[]int16",
				value:     make([]int16, 0),
			},
			want:    reflect.ValueOf(make([]int16, 0)),
			wantErr: false,
		},
		{
			name: "[]int32",
			args: args{
				valueType: "[]int32",
				value:     make([]int32, 0),
			},
			want:    reflect.ValueOf(make([]int32, 0)),
			wantErr: false,
		},
		{
			name: "[]int64",
			args: args{
				valueType: "[]int64",
				value:     make([]int64, 0),
			},
			want:    reflect.ValueOf(make([]int64, 0)),
			wantErr: false,
		},

		{
			name: "[]uint",
			args: args{
				valueType: "[]uint",
				value:     make([]uint, 0),
			},
			want:    reflect.ValueOf(make([]uint, 0)),
			wantErr: false,
		},
		{
			name: "[]uint8",
			args: args{
				valueType: "[]uint8",
				value:     make([]uint8, 0),
			},
			want:    reflect.ValueOf(make([]uint8, 0)),
			wantErr: false,
		},
		{
			name: "[]uint16",
			args: args{
				valueType: "[]uint16",
				value:     make([]uint16, 0),
			},
			want:    reflect.ValueOf(make([]uint16, 0)),
			wantErr: false,
		},
		{
			name: "[]uint32",
			args: args{
				valueType: "[]uint32",
				value:     make([]uint32, 0),
			},
			want:    reflect.ValueOf(make([]uint32, 0)),
			wantErr: false,
		},
		{
			name: "[]uint64",
			args: args{
				valueType: "[]uint64",
				value:     make([]uint64, 0),
			},
			want:    reflect.ValueOf(make([]uint64, 0)),
			wantErr: false,
		},

		{
			name: "[]float32",
			args: args{
				valueType: "[]float32",
				value:     make([]float32, 0),
			},
			want:    reflect.ValueOf(make([]float32, 0)),
			wantErr: false,
		},
		{
			name: "[]float64",
			args: args{
				valueType: "[]float64",
				value:     make([]float64, 0),
			},
			want:    reflect.ValueOf(make([]float64, 0)),
			wantErr: false,
		},

		{
			name: "[]string",
			args: args{
				valueType: "[]string",
				value:     make([]string, 0),
			},
			want:    reflect.ValueOf(make([]string, 0)),
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := ReflectValue(tt.args.valueType, tt.args.value)
			if (err != nil) != tt.wantErr {
				t.Errorf("ReflectValue() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got.Type().String() != tt.args.valueType {
				t.Errorf("type is %v, want %s", got.Type().String(), tt.args.valueType)
				return
			}
			if tt.args.value != nil {
				if !reflect.DeepEqual(got.Interface(), tt.args.value) {
					t.Errorf("value is %v, want %v", got.Interface(), tt.args.value)
				}
			}
		})
	}
}
