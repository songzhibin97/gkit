package timeout

import (
	"database/sql/driver"
	"strconv"
	"time"
)

// Stamp 用于MySQL时间戳转换
// 实现了 sql.Scanner 接口
type Stamp int64

// Scan 扫描赋值
func (jt *Stamp) Scan(src interface{}) (err error) {
	// 断言,只处理string以及原生的time.Time
	switch sc := src.(type) {
	case []byte:
		var i int64
		i, err = strconv.ParseInt(string(sc), 10, 64)
		*jt = Stamp(i)
	case time.Time:
		*jt = Stamp(sc.Unix())
	case string:
		var i int64
		i, err = strconv.ParseInt(sc, 10, 64)
		*jt = Stamp(i)
	}
	return
}

// Value 获取driver.Value
func (jt Stamp) Value() driver.Value {
	return time.Unix(int64(jt), 0)
}

// Time 转化time.Time
func (jt Stamp) Time() time.Time {
	return time.Unix(int64(jt), 0)
}
