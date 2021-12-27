package parse_pb

import "testing"

func TestParsePb(t *testing.T) {
	r, err := ParsePb("/Users/songzhibin/go/src/github.com/songzhibin97/gkit/parse/demo/test.proto")
	if err != nil {
		panic(err)
	}
	t.Log(r.Generate())
}
