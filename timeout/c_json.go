package timeout

import (
	"bytes"
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"strings"
)

type DbJSON []byte

func DBJSONFromObject(obj interface{}) DbJSON {
	bin, _ := json.Marshal(obj)
	return DbJSON(bin)
}

func (j *DbJSON) Scan(value interface{}) error {
	switch v := value.(type) {
	case []byte:
		return j.UnmarshalJSON(v)
	case string:
		return j.UnmarshalText(v)
	case nil:
		*j = nil
	default:
		return fmt.Errorf("cannot sql.Scan() DbJSON from: %#v", v)
	}
	return nil
}

func (j DbJSON) Value() (driver.Value, error) {
	str := string(j)
	if len(str) == 0 || strings.ToLower(str) == `null` {
		return driver.Value(nil), nil
	}
	return driver.Value(str), nil
}

func (j *DbJSON) UnmarshalJSON(src []byte) (err error) {
	if len(src) == 0 {
		*j = []byte(`null`)
	} else {
		buf := bytes.NewBuffer(make([]byte, 0))
		err = json.Compact(buf, src)
		*j = buf.Bytes()
	}
	return
}

func (j DbJSON) MarshalJSON() ([]byte, error) {
	if len(j) == 0 {
		return []byte("null"), nil
	}
	return []byte(j), nil
}

func (j *DbJSON) UnmarshalText(value string) error {
	return j.UnmarshalJSON([]byte(value))
}

func (j *DbJSON) Get(obj interface{}) error {
	if j == nil || *j == nil {
		return nil
	}
	return json.Unmarshal([]byte(*j), obj)
}

func (j *DbJSON) Set(obj interface{}) (err error) {
	*j, err = json.Marshal(obj)
	return
}

func (DbJSON) GormDataType() string {
	return "JSON"
}
