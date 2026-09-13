package clock

import (
	"testing"
	"time"
)

func TestFormatMillisFullRange(t *testing.T) {
	for _, tt := range []struct {
		name    string
		millis  uint64
		seconds int64
	}{
		{"epoch", 0, 0},
		{"subsecond", 999, 0},
		{"current era", 1789257600123, 1789257600},
		{"nanosecond boundary", 9223372036854, 9223372036},
		{"past nanosecond boundary", 9223372037000, 9223372037},
		{"year 2300", 10413792000000, 10413792000},
		{"uint64 upper half", 9223372036854775808, 9223372036854775},
		{"uint64 maximum", 18446744073709551615, 18446744073709551},
	} {
		t.Run(tt.name, func(t *testing.T) {
			// These formats discard milliseconds; literal Unix seconds are an independent oracle.
			want := time.Unix(tt.seconds, 0)
			if got := FormatTimeMillis(tt.millis); got != want.Format(TimeFormat) {
				t.Errorf("FormatTimeMillis(%d)=%q, want %q", tt.millis, got, want.Format(TimeFormat))
			}
			if got := FormatDate(tt.millis); got != want.Format(DateFormat) {
				t.Errorf("FormatDate(%d)=%q, want %q", tt.millis, got, want.Format(DateFormat))
			}
		})
	}
}
