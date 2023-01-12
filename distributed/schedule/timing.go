package schedule

import "time"

// Schedule 返回下一次执行的时间
type Schedule interface {
	Next(time time.Time) time.Time
}
