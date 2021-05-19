package timeout

import (
	"database/sql/driver"
	"fmt"
	"strings"
	"time"
)

type DateTime time.Time

const DateTimeFormat = "2006-01-02 15:04:05"

func (d *DateTime) UnmarshalJSON(src []byte) error {
	return d.UnmarshalText(strings.Replace(string(src), "\"", "", -1))
}

func (d DateTime) MarshalJSON() ([]byte, error) {
	return []byte(`"` + d.String() + `"`), nil
}

func (d *DateTime) UnmarshalText(value string) error {
	dd, err := time.Parse(DateTimeFormat, value)
	if err != nil {
		return err
	}
	*d = DateTime(dd)
	return nil
}

func (d DateTime) String() string {
	return (time.Time)(d).Format(DateTimeFormat)
}

func (d *DateTime) Scan(value interface{}) error {
	switch v := value.(type) {
	case []byte:
		return d.UnmarshalText(string(v))
	case string:
		return d.UnmarshalText(v)
	case time.Time:
		*d = DateTime(v)
	case nil:
		*d = DateTime{}
	default:
		return fmt.Errorf("cannot sql.Scan() DBDate from: %#v", v)
	}
	return nil
}

func (d DateTime) Value() (driver.Value, error) {
	return driver.Value(time.Time(d).Format(DateTimeFormat)), nil
}

func (DateTime) GormDataType() string {
	return "DATETIME"
}
