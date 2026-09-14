package distributed

import (
	"fmt"
	"testing"
	"time"

	"github.com/robfig/cron/v3"
	"github.com/songzhibin97/gkit/distributed/task"
)

// #166 / 03-04 also reaches the pinned external cron parser through these
// public registration methods. Invalid input must not panic or add a job.
func TestTimedRegistrationRejectsTimezoneOnlySpec(t *testing.T) {
	for name, register := range map[string]func(*Server, string) error{
		"task": func(s *Server, spec string) error {
			return s.RegisteredTimedTask(spec, "job", task.NewSignature("root", "task"))
		},
		"chain": func(s *Server, spec string) error {
			return s.RegisteredTimedChain(spec, "job", task.NewSignature("root", "task"))
		},
		"group": func(s *Server, spec string) error {
			return s.RegisteredTimedGroup(spec, "job", "group", 1, task.NewSignature("root", "task"))
		},
		"group_callback": func(s *Server, spec string) error {
			return s.RegisteredTimedGroupCallback(spec, "job", "group", 1, task.NewSignature("callback", "callback"), task.NewSignature("root", "task"))
		},
	} {
		for _, prefix := range []string{"TZ=UTC", "CRON_TZ=Asia/Shanghai"} {
			for _, suffix := range []string{"", " ", "\t"} {
				t.Run(fmt.Sprintf("%s/%q", name, prefix+suffix), func(t *testing.T) {
					defer func() {
						if value := recover(); value != nil {
							t.Fatalf("registration panicked: %v", value)
						}
					}()
					server := &Server{scheduler: cron.New()}
					if err := register(server, prefix+suffix); err == nil {
						t.Fatal("timezone-only registration succeeded")
					}
					if got := len(server.scheduler.Entries()); got != 0 {
						t.Fatalf("invalid registration added %d jobs", got)
					}
				})
			}
		}
		t.Run(name+"/valid", func(t *testing.T) {
			server := &Server{scheduler: cron.New()}
			if err := register(server, "CRON_TZ=Asia/Shanghai @daily"); err != nil {
				t.Fatal(err)
			}
			entries := server.scheduler.Entries()
			if len(entries) != 1 {
				t.Fatalf("registered jobs = %d, want 1", len(entries))
			}
			anchor := time.Date(2026, time.January, 1, 12, 34, 56, 0, time.UTC)
			want := time.Date(2026, time.January, 1, 16, 0, 0, 0, time.UTC)
			if next := entries[0].Schedule.Next(anchor); !next.Equal(want) {
				t.Fatalf("Next = %v, want %v", next, want)
			}
		})
	}
}
