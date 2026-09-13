package task

import (
	"encoding/json"
	"testing"
)

func TestResultsScanDriverValues(t *testing.T) {
	const body = `[{"Type":"int64","Value":9007199254740993}]`
	for _, src := range []interface{}{body, []byte(body)} {
		var results Results
		if err := results.Scan(src); err != nil {
			t.Fatalf("Scan(%T): %v", src, err)
		}
		if len(results) != 1 || results[0].Value != json.Number("9007199254740993") {
			t.Fatalf("Scan(%T): %#v", src, results)
		}
	}
	for _, src := range []interface{}{42, "invalid", []byte("invalid")} {
		var results Results
		if err := results.Scan(src); err == nil {
			t.Fatalf("Scan(%T) accepted invalid input", src)
		}
	}
}
