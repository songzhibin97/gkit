package timeout

import (
	"database/sql/driver"
	"fmt"
	"strings"
	"time"
)

type DateTimeStruct struct {
	time.Time
}

// UnmarshalText 通过 string 反序列化成 DateTime
// Author [SliverHorn](https://github.com/SliverHorn)
func (m *DateTimeStruct) UnmarshalText(value string) error {
	dd, err := time.Parse(DateTimeFormat, value)
	if err != nil {
		return err
	}
	m.Time = dd
	return nil
}

// UnmarshalTextByLayout 通过 string 序列化成 DateTime
// Author [SliverHorn](https://github.com/SliverHorn)
func (m *DateTimeStruct) UnmarshalTextByLayout(layout, value string) error {
	dd, err := time.Parse(layout, value)
	if err != nil {
		return err
	}
	m.Time = dd
	return nil
}

// ToDate DateTime 2 Date
// Author [SliverHorn](https://github.com/SliverHorn)
func (m *DateTimeStruct) ToDate() *DateStruct {
	if m == nil {
		return nil
	}
	return &DateStruct{Time: m.Time}
}

// UnmarshalJSON 反序列化
// Author [SliverHorn](https://github.com/SliverHorn)
func (m *DateTimeStruct) UnmarshalJSON(src []byte) error {
	return m.UnmarshalText(strings.Replace(string(src), "\"", "", -1))
}

// MarshalJSON 序列化
// Author [SliverHorn](https://github.com/SliverHorn)
func (m *DateTimeStruct) MarshalJSON() ([]byte, error) {
	return []byte(`"` + m.String() + `"`), nil
}

// String 输出 DateTime 变量为字符串
// Author [SliverHorn](https://github.com/SliverHorn)
func (m *DateTimeStruct) String() string {
	return m.Format(DateTimeFormat)
}

// Scan 扫描
// Author [SliverHorn](https://github.com/SliverHorn)
func (m *DateTimeStruct) Scan(value interface{}) error {
	switch v := value.(type) {
	case []byte:
		return m.UnmarshalText(string(v))
	case string:
		return m.UnmarshalText(v)
	case time.Time:
		m.Time = v
	case nil:
		*m = DateTimeStruct{}
	default:
		return fmt.Errorf("cannot sql.Scan() DBDate from: %#v", v)
	}
	return nil
}

// Value 值
// Author [SliverHorn](https://github.com/SliverHorn)
func (m *DateTimeStruct) Value() (driver.Value, error) {
	return driver.Value(m.Format(DateTimeFormat)), nil
}

// GormDataType gorm 定义数据库字段类型
// Author [SliverHorn](https://github.com/SliverHorn)
func (m *DateTimeStruct) GormDataType() string {
	return "DATETIME"
}
