package parse_go

import (
	"testing"
)

func TestGoParsePB_GeneratePB(t *testing.T) {
	rr, err := ParseGo(
		"/Users/songzhibin/go/src/Songzhibin/gkit/parse/demo/demo.api", AddParseFunc(parseDoc), AddParseStruct(parseTag))
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
	rr, err := ParseGo("/Users/songzhibin/go/src/Songzhibin/gkit/parse/demo/demo.api")
	if err != nil {
		panic(err)
	}
	r := rr.(*GoParsePB)
	t.Log(r.PileDismantle("var _ = 1"))
}

func Test_checkRepeat(t *testing.T) {
	test := `type Demo struct {
    MapField        map[string]int
    SliceField      []int
	StringField     string
	Uint32Field     uint32 
	// 注释1
    // 注释1.1

    // 注释1.2
	InterfaceField  interface{}
	InterField      Inter
	EmptyField
}`
	t.Log(checkRepeat("// 注释1", test))
}
