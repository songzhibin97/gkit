package timeout

import (
	"fmt"
	"time"
)

func HumanDurationFormat(stamp int64, designation ...time.Time) string {
	designation = append(designation, time.Now())
	now := designation[0].Sub(time.Unix(stamp, 0))

	if now < time.Minute {
		return "刚刚"
	}

	if now < time.Hour {
		return fmt.Sprintf("%d分钟前", now/(time.Minute))
	}

	if now < 24*time.Hour {
		return fmt.Sprintf("%d小时前", now/(time.Hour))
	}

	if now < 7*24*time.Hour {
		return fmt.Sprintf("%d天前", now/(24*time.Hour))
	}

	if now < 30*7*24*time.Hour {
		return fmt.Sprintf("%d周前", now/(7*24*time.Hour))
	}

	if now < 12*30*7*24*time.Hour {
		return fmt.Sprintf("%d月前", now/(30*7*24*time.Hour))
	}

	return fmt.Sprintf("%d年前", now/(12*30*7*24*time.Hour))
}
