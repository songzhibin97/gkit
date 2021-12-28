package timeout

import (
	"database/sql/driver"
	"fmt"
	"strings"
	"time"
)

type DateStruct struct {
	time.Time
}

// UnmarshalText 通过 string 序列化成 Date
// Author [SliverHorn](https://github.com/SliverHorn)
func (m *DateStruct) UnmarshalText(value string) error {
	dd, err := time.Parse(DateFormat, value)
	if err != nil {
		return err
	}
	m.Time = dd
	return nil
}

// UnmarshalTextByLayout 通过 string 序列化成 Date by layout
// Author [SliverHorn](https://github.com/SliverHorn)
func (m *DateStruct) UnmarshalTextByLayout(layout, value string) error {
	dd, err := time.Parse(layout, value)
	if err != nil {
		return err
	}
	m.Time = dd
	return nil
}

// ToDateTime Date to Datetime
// Author [SliverHorn](https://github.com/SliverHorn)
func (m *DateStruct) ToDateTime() *DateTimeStruct {
	return &DateTimeStruct{Time: m.Time}
}

// UnmarshalJSON 反序列化
// Author [SliverHorn](https://github.com/SliverHorn)
func (m *DateStruct) UnmarshalJSON(src []byte) error {
	return m.UnmarshalText(strings.Replace(string(src), "\"", "", -1))
}

// MarshalJSON 序列化
// Author [SliverHorn](https://github.com/SliverHorn)
func (m *DateStruct) MarshalJSON() ([]byte, error) {
	return []byte(`"` + m.String() + `"`), nil
}

// String 输出 DateTime 变量为字符串
// Author [SliverHorn](https://github.com/SliverHorn)
func (m *DateStruct) String() string {
	return m.Format(DateFormat)
}

// Scan 扫描
// Author [SliverHorn](https://github.com/SliverHorn)
func (m *DateStruct) Scan(value interface{}) error {
	switch v := value.(type) {
	case []byte:
		return m.UnmarshalText(string(v))
	case string:
		return m.UnmarshalText(v)
	case time.Time:
		m.Time = v
	case nil:
		*m = DateStruct{}
	default:
		return fmt.Errorf("cannot sql.Scan() DBDate from: %#v", v)
	}
	return nil
}

// Value 值
// Author [SliverHorn](https://github.com/SliverHorn)
func (m *DateStruct) Value() (driver.Value, error) {
	return driver.Value(m.Format(DateFormat)), nil
}

// GormDataType gorm 定义数据库字段类型
// Author [SliverHorn](https://github.com/SliverHorn)
func (m *DateStruct) GormDataType() string {
	return "DATE"
}
