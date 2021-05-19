package timeout

import (
	"database/sql/driver"
	"fmt"
	"strings"
	"time"
)

type Date time.Time

const DateFormat = "2006-01-02"

func (d *Date) UnmarshalJSON(src []byte) error {
	return d.UnmarshalText(strings.Replace(string(src), "\"", "", -1))
}

func (d Date) MarshalJSON() ([]byte, error) {
	return []byte(`"` + d.String() + `"`), nil
}

func (d *Date) UnmarshalText(value string) error {
	dd, err := time.Parse(DateFormat, value)
	if err != nil {
		return err
	}
	*d = Date(dd)
	return nil
}

func (d Date) String() string {
	return (time.Time)(d).Format(DateFormat)
}

func (d *Date) Scan(value interface{}) error {
	switch v := value.(type) {
	case []byte:
		return d.UnmarshalText(string(v))
	case string:
		return d.UnmarshalText(v)
	case time.Time:
		*d = Date(v)
	case nil:
		*d = Date{}
	default:
		return fmt.Errorf("cannot sql.Scan() DBDate from: %#v", v)
	}
	return nil
}

func (d Date) Value() (driver.Value, error) {
	return driver.Value(time.Time(d).Format(DateFormat)), nil
}

func (Date) GormDataType() string {
	return "DATE"
}
