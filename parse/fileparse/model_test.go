package fileparse

import "testing"

func TestGoParsePB_GeneratePB(t *testing.T) {
	r, err := ParseGo("/Users/songzhibin/go/src/Songzhibin/GKit/parse/demo/demo.api")
	if err != nil {
		panic(err)
	}
	t.Log(r.checkFormat())
	t.Log(r)
	//t.Log(r.GeneratePB())
}
