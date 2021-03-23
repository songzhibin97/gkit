package parseGo

import (
	"testing"
)

func TestGoParsePB_GeneratePB(t *testing.T) {
	r, err := ParseGo("/Users/songzhibin/go/src/Songzhibin/GKit/parse/demo/demo.api")
	if err != nil {
		panic(err)
	}
	for _, note := range r.Notes() {
		t.Log(note.Text, note.Pos(), note.End())
	}
	t.Log(r.Generate())
}

func TestGoParsePB_PileDriving(t *testing.T) {
	rr, err := ParseGo("/Users/songzhibin/go/src/Songzhibin/GKit/parse/demo/demo.api")
	if err != nil {
		panic(err)
	}
	r := rr.(*GoParsePB)
	t.Log(r.PileDriving("", "start", "end", "testPileDriving"))
}
