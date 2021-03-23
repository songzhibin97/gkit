package parsePb

import "testing"

func TestParsePb(t *testing.T) {
	r, err := ParsePb("/Users/songzhibin/go/src/Songzhibin/GKit/parse/demo/test.proto")
	if err != nil {
		panic(err)
	}
	t.Log(r)
}
