package parseGo

import (
	"testing"
)

func TestGoParsePB_GeneratePB(t *testing.T) {
	rr, err := ParseGo("/Users/songzhibin/go/src/Songzhibin/GKit/parse/demo/demo.api")
	if err != nil {
		panic(err)
	}
	r := rr.(*GoParsePB)
	for _, note := range r.Note {
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
