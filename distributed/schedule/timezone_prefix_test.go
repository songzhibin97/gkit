package schedule

import (
	"strings"
	"testing"
	"time"
)

// Regression for #166 / 03-04: a timezone directive without a cron expression
// is invalid input, not a negative slice boundary or an executable schedule.
func TestTimezonePrefixRequiresSchedule(t *testing.T) {
	for _, prefix := range []string{"TZ=UTC", "CRON_TZ=Asia/Shanghai"} {
		for _, suffix := range []string{"", " ", "\t", " \t\n"} {
			for name, parse := range map[string]func(string) (Schedule, error){
				"standard": NewParseWithStandard, "seconds": NewParseWithSecondParser,
			} {
				t.Run(name+"/"+prefix+"/"+suffix, func(t *testing.T) {
					defer func() {
						if value := recover(); value != nil {
							t.Fatalf("parser panicked: %v", value)
						}
					}()
					if schedule, err := parse(prefix + suffix); err == nil || schedule != nil {
						t.Fatalf("Parse(%q) = (%v, %v), want nil schedule and error", prefix+suffix, schedule, err)
					}
				})
			}
		}
	}
}

func TestTimezonePrefixKeepsValidSchedules(t *testing.T) {
	anchor := time.Date(2026, time.January, 1, 12, 34, 56, 0, time.UTC)
	for _, test := range []struct {
		spec  string
		parse func(string) (Schedule, error)
		want  time.Time
	}{
		{"TZ=UTC 0 0 * * *", NewParseWithStandard, time.Date(2026, time.January, 2, 0, 0, 0, 0, time.UTC)},
		{"CRON_TZ=Asia/Shanghai 0 0 * * *", NewParseWithStandard, time.Date(2026, time.January, 1, 16, 0, 0, 0, time.UTC)},
		{"TZ=UTC  0 0 0 * * *", NewParseWithSecondParser, time.Date(2026, time.January, 2, 0, 0, 0, 0, time.UTC)},
		{"CRON_TZ=Asia/Shanghai @daily", NewParseWithSecondParser, time.Date(2026, time.January, 1, 16, 0, 0, 0, time.UTC)},
	} {
		t.Run(test.spec, func(t *testing.T) {
			schedule, err := test.parse(test.spec)
			if err != nil {
				t.Fatal(err)
			}
			if next := schedule.Next(anchor); !next.Equal(test.want) {
				t.Fatalf("Next = %v, want %v", next, test.want)
			}
		})
	}
	if schedule, err := NewParseWithStandard("TZ=not/a-zone * * * * *"); schedule != nil || err == nil || !strings.Contains(err.Error(), "provided bad location") {
		t.Fatalf("invalid location = (%v, %v), want location error", schedule, err)
	}
}
