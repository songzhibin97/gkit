package fileparse

import "testing"

func TestGoParsePB_GeneratePB(t *testing.T) {
	r, err := ParseGo("/Users/songzhibin/go/src/Songzhibin/GKit/parse/demo/demo.api")
	if err != nil {
		panic(err)
	}
	for _, note := range r.notes {
		t.Log(note.Text,note.Pos(),note.End())
	}
	//t.Log(r.GeneratePB())
}

func TestGoParsePB_PileDriving(t *testing.T) {
	r, err := ParseGo("/Users/songzhibin/go/src/Songzhibin/GKit/parse/demo/demo.api")
	if err != nil {
		panic(err)
	}
	t.Log(r.PileDriving("","start","end","testPileDriving"))
}
