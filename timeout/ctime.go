package timeout

import (
	"context"
	"time"
)

// ctime GKIT时间模块
// 主要提供context超时控制

// Shrink 用于链路超时时间以及当前节点的超时时间控制
func Shrink(c context.Context, d time.Duration) (time.Duration, context.Context, context.CancelFunc) {
	if deadline, ok := c.Deadline(); ok {
		if timeout := time.Until(deadline); timeout < d {
			// 链路超时时间已经小于当前节点的超时时间了,所以以上流链路为准,不重新设置
			return timeout, c, func() {}
		}
	}
	// 说明没有设置timeout或者deadline
	ctx, cancel := context.WithTimeout(c, d)
	return d, ctx, cancel
}
