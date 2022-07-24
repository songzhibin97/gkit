package task

import (
	"reflect"
	"testing"
)

func TestConvertResult(t *testing.T) {
	type args struct {
		result []*Result
	}
	tests := []struct {
		name    string
		args    args
		want    []reflect.Value
		wantErr bool
	}{
		{
			name: "string",
			args: args{result: []*Result{{
				Type:  "string",
				Value: "gkit",
			}}},
			want:    []reflect.Value{reflect.ValueOf("gkit")},
			wantErr: false,
		},
		{
			name: "mix",
			args: args{result: []*Result{{
				Type:  "string",
				Value: "gkit",
			}, {
				Type:  "int",
				Value: 1,
			}, {
				Type:  "[]string",
				Value: []string{"6", "66", "666"},
			}}},
			want: []reflect.Value{reflect.ValueOf("gkit"), reflect.ValueOf(int(1)), reflect.ValueOf([]string{"6", "66", "666"})},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ConvertResult(tt.args.result)
			if (err != nil) != tt.wantErr {
				t.Errorf("ConvertResult() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if len(got) != len(tt.want) {
				t.Errorf("got len no equal want: got len = %d  want len = %d", len(got), len(tt.want))

			}
			for i := range got {
				if !reflect.DeepEqual(got[i].Interface(), tt.want[i].Interface()) {
					t.Errorf("ConvertResult() got = %v, want %v", got, tt.want)
				}
			}
			t.Log(FormatResult(got))
		})
	}
}
