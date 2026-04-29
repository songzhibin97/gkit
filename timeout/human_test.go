package timeout

import (
	"testing"
	"time"
)

func TestHumanDurationFormat(t *testing.T) {
	base := time.Date(2026, 4, 30, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name string
		ago  time.Duration
		want string
	}{
		{"小于一分钟", 30 * time.Second, "刚刚"},
		{"分钟边界-1分钟", time.Minute, "1分钟前"},
		{"分钟边界-59分钟", 59 * time.Minute, "59分钟前"},
		{"小时边界-1小时", time.Hour, "1小时前"},
		{"小时边界-23小时", 23 * time.Hour, "23小时前"},
		{"天边界-1天", 24 * time.Hour, "1天前"},
		{"天边界-6天", 6 * 24 * time.Hour, "6天前"},
		{"周边界-7天", 7 * 24 * time.Hour, "1周前"},
		{"周边界-29天", 29 * 24 * time.Hour, "4周前"},
		{"月边界-30天", 30 * 24 * time.Hour, "1月前"},
		{"月边界-60天", 60 * 24 * time.Hour, "2月前"},
		{"月边界-359天", 359 * 24 * time.Hour, "11月前"},
		{"年边界-360天", 360 * 24 * time.Hour, "1年前"},
		{"年边界-400天", 400 * 24 * time.Hour, "1年前"},
		{"年边界-720天", 720 * 24 * time.Hour, "2年前"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stamp := base.Add(-tt.ago).Unix()
			got := HumanDurationFormat(stamp, base)
			if got != tt.want {
				t.Errorf("HumanDurationFormat(ago=%v) = %q, want %q", tt.ago, got, tt.want)
			}
		})
	}
}

func TestHumanDurationFormat_DefaultNow(t *testing.T) {
	stamp := time.Now().Add(-30 * time.Second).Unix()
	if got := HumanDurationFormat(stamp); got != "刚刚" {
		t.Errorf("HumanDurationFormat() = %q, want %q", got, "刚刚")
	}
}
