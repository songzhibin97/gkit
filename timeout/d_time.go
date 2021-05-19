package timeout

import (
	"database/sql/driver"
	"fmt"
	"strings"
	"time"
)

type DTime time.Time

const DTimeFormat = "15:04:05"

func (d *DTime) UnmarshalJSON(src []byte) error {
	return d.UnmarshalText(strings.Replace(string(src), "\"", "", -1))
}

func (d DTime) MarshalJSON() ([]byte, error) {
	return []byte(`"` + d.String() + `"`), nil
}

func (d *DTime) UnmarshalText(value string) error {
	dd, err := time.Parse(DTimeFormat, value)
	if err != nil {
		return err
	}
	*d = DTime(dd)
	return nil
}

func (d DTime) String() string {
	return (time.Time)(d).Format(DTimeFormat)
}

func (d *DTime) Scan(value interface{}) error {
	switch v := value.(type) {
	case []byte:
		return d.UnmarshalText(string(v))
	case string:
		return d.UnmarshalText(v)
	case time.Time:
		*d = DTime(v)
	case nil:
		*d = DTime{}
	default:
		return fmt.Errorf("cannot sql.Scan() DBDate from: %#v", v)
	}
	return nil
}

func (d DTime) Value() (driver.Value, error) {
	return driver.Value(time.Time(d).Format(DTimeFormat)), nil
}

func (DTime) GormDataType() string {
	return "TIME"
}
