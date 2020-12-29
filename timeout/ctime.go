package timeout

import (
	"context"
	"database/sql/driver"
	"strconv"
	"time"
)

// ctime GKIT时间模块
// 主要提供context超时控制

// Shrink: 用于链路超时时间以及当前节点的超时时间控制
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

// Time 用于MySQL时间戳转换
// 实现了 sql.Scanner 接口
type Time int64

// Scan 扫描赋值
func (jt *Time) Scan(src interface{}) (err error) {
	// 断言,只处理string以及原生的time.Time
	switch sc := src.(type) {
	case time.Time:
		*jt = Time(sc.Unix())
	case string:
		var i int64
		i, err = strconv.ParseInt(sc, 10, 64)
		*jt = Time(i)
	}
	return
}

// Value 获取driver.Value
func (jt Time) Value() driver.Value {
	return time.Unix(int64(jt), 0)
}

// Time 转化time.Time
func (jt Time) Time() time.Time {
	return time.Unix(int64(jt), 0)
}
