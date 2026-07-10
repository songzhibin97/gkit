package timeout

import (
	"database/sql/driver"
	"encoding/json"
	"testing"
	"time"
)

func TestStampImplementsValuerAndScansDriverInt64(t *testing.T) {
	if _, ok := interface{}(Stamp(123)).(driver.Valuer); !ok {
		t.Fatal("Stamp does not implement driver.Valuer")
	}
	value, err := interface{}(Stamp(123)).(driver.Valuer).Value()
	if err != nil {
		t.Fatal(err)
	}
	wantTime := time.Unix(123, 0)
	gotTime, ok := value.(time.Time)
	if !ok || !gotTime.Equal(wantTime) {
		t.Fatalf("Stamp.Value() = %#v, want %v", value, wantTime)
	}

	var stamp Stamp
	if err := stamp.Scan(int64(1700000000)); err != nil {
		t.Fatal(err)
	}
	if stamp != Stamp(1700000000) {
		t.Fatalf("Stamp.Scan(int64) = %d, want 1700000000", stamp)
	}
	stamp = 99
	if err := stamp.Scan(float64(123)); err == nil {
		t.Fatal("Stamp.Scan(float64) returned nil error")
	}
	if stamp != 99 {
		t.Fatalf("unsupported Scan mutated Stamp to %d", stamp)
	}
}

func TestWallClockDatabaseTypesRoundTripInLocalLocation(t *testing.T) {
	originalLocal := time.Local
	time.Local = time.FixedZone("UTC+8", 8*60*60)
	t.Cleanup(func() { time.Local = originalLocal })

	tests := []struct {
		name     string
		original time.Time
		value    func() (driver.Value, error)
		scan     func(driver.Value) (time.Time, error)
	}{
		{
			name:     "Date",
			original: time.Date(2024, 1, 15, 0, 0, 0, 0, time.Local),
			value:    func() (driver.Value, error) { return Date(time.Date(2024, 1, 15, 0, 0, 0, 0, time.Local)).Value() },
			scan: func(value driver.Value) (time.Time, error) {
				var decoded Date
				err := decoded.Scan(value)
				return time.Time(decoded), err
			},
		},
		{
			name:     "DateTime",
			original: time.Date(2024, 1, 15, 10, 20, 30, 0, time.Local),
			value: func() (driver.Value, error) {
				return DateTime(time.Date(2024, 1, 15, 10, 20, 30, 0, time.Local)).Value()
			},
			scan: func(value driver.Value) (time.Time, error) {
				var decoded DateTime
				err := decoded.Scan(value)
				return time.Time(decoded), err
			},
		},
		{
			name:     "DTime",
			original: time.Date(0, 1, 1, 10, 20, 30, 0, time.Local),
			value:    func() (driver.Value, error) { return DTime(time.Date(0, 1, 1, 10, 20, 30, 0, time.Local)).Value() },
			scan: func(value driver.Value) (time.Time, error) {
				var decoded DTime
				err := decoded.Scan(value)
				return time.Time(decoded), err
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			encoded, err := tt.value()
			if err != nil {
				t.Fatal(err)
			}
			decoded, err := tt.scan(encoded)
			if err != nil {
				t.Fatal(err)
			}
			if !decoded.Equal(tt.original) {
				t.Fatalf("Value -> Scan = %v (%d), want %v (%d)", decoded, decoded.Unix(), tt.original, tt.original.Unix())
			}
		})
	}
}

func TestDateStructJSONValueAndPointerAreSymmetric(t *testing.T) {
	tests := []struct {
		name  string
		value interface{}
		ptr   interface{}
		fresh func() interface{}
	}{
		{
			name:  "DateStruct",
			value: DateStruct{Time: time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC)},
			ptr:   &DateStruct{Time: time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC)},
			fresh: func() interface{} { return &DateStruct{} },
		},
		{
			name:  "DateTimeStruct",
			value: DateTimeStruct{Time: time.Date(2024, 1, 15, 10, 20, 30, 0, time.UTC)},
			ptr:   &DateTimeStruct{Time: time.Date(2024, 1, 15, 10, 20, 30, 0, time.UTC)},
			fresh: func() interface{} { return &DateTimeStruct{} },
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fromValue, err := json.Marshal(tt.value)
			if err != nil {
				t.Fatal(err)
			}
			fromPointer, err := json.Marshal(tt.ptr)
			if err != nil {
				t.Fatal(err)
			}
			if string(fromValue) != string(fromPointer) {
				t.Fatalf("value JSON = %s, pointer JSON = %s", fromValue, fromPointer)
			}
			decoded := tt.fresh()
			if err := json.Unmarshal(fromValue, decoded); err != nil {
				t.Fatalf("value JSON did not round-trip: %v", err)
			}
		})
	}
}
