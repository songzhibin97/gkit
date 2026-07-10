package timeout

import (
	"database/sql/driver"
	"fmt"
	"strconv"
	"time"
)

// Stamp 用于MySQL时间戳转换
// 实现了 sql.Scanner 接口
type Stamp int64

var _ driver.Valuer = Stamp(0)

// Scan 扫描赋值
func (jt *Stamp) Scan(src interface{}) (err error) {
	// 断言,只处理string以及原生的time.Time
	switch sc := src.(type) {
	case []byte:
		var value int64
		value, err = strconv.ParseInt(string(sc), 10, 64)
		if err == nil {
			*jt = Stamp(value)
		}
	case int64:
		*jt = Stamp(sc)
	case time.Time:
		*jt = Stamp(sc.Unix())
	case string:
		var value int64
		value, err = strconv.ParseInt(sc, 10, 64)
		if err == nil {
			*jt = Stamp(value)
		}
	default:
		err = fmt.Errorf("timeout.Stamp: cannot scan value of type %T", src)
	}
	return
}

// Value 获取driver.Value
func (jt Stamp) Value() (driver.Value, error) {
	return time.Unix(int64(jt), 0), nil
}

// Time 转化time.Time
func (jt Stamp) Time() time.Time {
	return time.Unix(int64(jt), 0)
}
