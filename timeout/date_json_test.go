package timeout

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"
)

type dateJSONCodec interface {
	json.Marshaler
	json.Unmarshaler
	UnmarshalText(string) error
}

func dateJSONTime(t *testing.T, value dateJSONCodec) time.Time {
	t.Helper()
	switch v := value.(type) {
	case *Date:
		return time.Time(*v)
	case *DateTime:
		return time.Time(*v)
	case *DTime:
		return time.Time(*v)
	case *DateStruct:
		return v.Time
	case *DateTimeStruct:
		return v.Time
	default:
		t.Fatalf("unexpected date type %T", value)
		return time.Time{}
	}
}

func TestDateJSONUsesDecodedStringSemantics(t *testing.T) {
	for _, tc := range []struct {
		name     string
		newValue func() dateJSONCodec
		values   []string
		location *time.Location
	}{
		{"Date", func() dateJSONCodec { return new(Date) }, []string{"0000-01-01", "2024-02-29", "9999-12-31"}, time.Local},
		{"DateTime", func() dateJSONCodec { return new(DateTime) }, []string{"0000-01-01 00:00:00", "2024-02-29 12:34:56", "9999-12-31 23:59:59"}, time.Local},
		{"DTime", func() dateJSONCodec { return new(DTime) }, []string{"00:00:00", "12:34:56", "23:59:59"}, time.Local},
		{"DateStruct", func() dateJSONCodec { return new(DateStruct) }, []string{"0000-01-01", "2024-02-29", "9999-12-31"}, time.UTC},
		{"DateTimeStruct", func() dateJSONCodec { return new(DateTimeStruct) }, []string{"0000-01-01 00:00:00", "2024-02-29 12:34:56", "9999-12-31 23:59:59"}, time.UTC},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for _, value := range tc.values {
				plain, err := json.Marshal(value)
				if err != nil {
					t.Fatal(err)
				}
				var escaped strings.Builder
				escaped.WriteByte('"')
				for _, r := range value {
					fmt.Fprintf(&escaped, `\u%04x`, r)
				}
				escaped.WriteByte('"')
				textValue := tc.newValue()
				if err := textValue.UnmarshalText(value); err != nil {
					t.Fatalf("text control %q: %v", value, err)
				}
				expected := dateJSONTime(t, textValue)
				for _, encoded := range []string{string(plain), escaped.String(), fmt.Sprintf(`"\u%04x%s"`, value[0], value[1:]), " \n" + string(plain) + "\t"} {
					var decodedString string
					if err := json.Unmarshal([]byte(encoded), &decodedString); err != nil || decodedString != value {
						t.Fatalf("JSON string control %q: value=%q err=%v", encoded, decodedString, err)
					}
					for _, direct := range []bool{true, false} {
						got := tc.newValue()
						var err error
						if direct {
							err = got.UnmarshalJSON([]byte(encoded))
						} else {
							err = json.Unmarshal([]byte(encoded), got)
						}
						if err != nil {
							t.Errorf("direct=%t equivalent JSON %s rejected: %v", direct, encoded, err)
							continue
						}
						actual := dateJSONTime(t, got)
						if !actual.Equal(expected) || actual.Location() != tc.location {
							t.Errorf("direct=%t decoded %s = %v (%s), want %v (%s)", direct, encoded, actual, actual.Location(), expected, tc.location)
						}
						marshaled, err := got.MarshalJSON()
						if err != nil || string(marshaled) != string(plain) {
							t.Errorf("canonical JSON = %s, err=%v, want %s", marshaled, err, plain)
						}
					}
				}
				// Null, malformed JSON, wrong JSON types, invalid layouts, and an
				// embedded quote all remain errors without modifying the receiver.
				quotedValue, err := json.Marshal(value + `"`)
				if err != nil {
					t.Fatal(err)
				}
				for _, invalid := range []string{"null", "true", "123", "{}", "[]", `"bad"`, `"2024-02-30"`, `"2024-02-30 12:34:56"`, `"24:00:00"`, value, `"\q"`, string(plain) + "x", string(quotedValue)} {
					for _, direct := range []bool{true, false} {
						got := tc.newValue()
						if err := got.UnmarshalText(value); err != nil {
							t.Fatal(err)
						}
						before := dateJSONTime(t, got)
						var err error
						if direct {
							err = got.UnmarshalJSON([]byte(invalid))
						} else {
							err = json.Unmarshal([]byte(invalid), got)
						}
						if err == nil {
							t.Errorf("direct=%t invalid JSON %s accepted", direct, invalid)
						}
						if after := dateJSONTime(t, got); after != before {
							t.Errorf("direct=%t invalid JSON %s changed receiver: %v -> %v", direct, invalid, before, after)
						}
					}
				}
			}
		})
	}
}
