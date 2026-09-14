package task

import (
	"encoding/json"
	jsoniter "github.com/json-iterator/go"
	"go.mongodb.org/mongo-driver/bson"
	"math"
	"reflect"
	"testing"
)

func TestResultJSONPrecision(t *testing.T) {
	tests := []struct {
		body string
		want interface{}
	}{
		{`{"type":"int64","value":9007199254740993}`, int64(9007199254740993)},
		{`{"type":"int64","value":9223372036854775807}`, int64(math.MaxInt64)},
		{`{"type":"uint64","value":18446744073709551615}`, uint64(math.MaxUint64)},
		{`{"type":"[]int64","value":[9007199254740993,-9223372036854775808]}`, []int64{9007199254740993, math.MinInt64}},
		{`{"type":"[]uint64","value":[18446744073709551615]}`, []uint64{math.MaxUint64}},
		{`{"type":"[]uint8","value":"AP8="}`, []uint8{0, 255}},
		{`{"type":"[]int64","value":[]}`, []int64{}},
		{`{"type":"[]int64","value":null}`, nil},
		{`{"type":"string","value":"integer 9007199254740993"}`, "integer 9007199254740993"},
		{`{"type":"custom","value":{"number":9007199254740993}}`, map[string]interface{}{"number": json.Number("9007199254740993")}},
		{`{"value":9007199254740993}`, json.Number("9007199254740993")},
	}
	for _, decode := range []func([]byte, interface{}) error{json.Unmarshal, jsoniter.Unmarshal} {
		for _, tt := range tests {
			var result Result
			if err := decode([]byte(tt.body), &result); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(result.Value, tt.want) {
				t.Errorf("%s: %T(%v), want %T(%v)", tt.body, result.Value, result.Value, tt.want, tt.want)
			}
		}
		for _, body := range []string{`{"type":"int64","value":9223372036854775808}`, `{"type":"[]int64","value":[1,"bad"]}`, `{"type":"uint64","value":-1}`} {
			var result Result
			if err := decode([]byte(body), &result); err == nil {
				t.Errorf("accepted invalid result %s", body)
			}
		}
	}
}

func TestResultJSONToBSONPrecision(t *testing.T) {
	var original Result
	if err := json.Unmarshal([]byte(`{"type":"[]int64","value":[9007199254740993,9223372036854775807]}`), &original); err != nil {
		t.Fatal(err)
	}
	body, err := bson.Marshal(original)
	if err != nil {
		t.Fatal(err)
	}
	var decoded Result
	if err := bson.Unmarshal(body, &decoded); err != nil {
		t.Fatal(err)
	}
	values, err := ReflectTaskResults([]*Result{&decoded})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(values[0].Interface(), []int64{9007199254740993, math.MaxInt64}) {
		t.Fatalf("BSON: %v", values)
	}
}
