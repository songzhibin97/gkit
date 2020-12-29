package timeout

import (
	"context"
	"testing"
	"time"
)

func TestShrink(t *testing.T) {
	c1, cancel := context.WithTimeout(context.Background(), 5)
	defer cancel()
	type args struct {
		c context.Context
		d time.Duration
	}
	tests := []struct {
		name string
		args args
		want time.Duration
	}{
		{"上游没有设置链路超时时间", args{c: context.Background(), d: 10}, 10},

		{"上游链路设置链路时间且当前时间超时时间大于流转链路时间", args{c: c1, d: 10}, 5},
		{"上路链路设置超时时间事件小于当前时间", args{c: c1, d: 3}, 3},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, _, _ := Shrink(tt.args.c, tt.args.d)
			if got > tt.want {
				t.Errorf("Shrink() got = %v, want %v", got, tt.want)
			}
		})
	}
}
