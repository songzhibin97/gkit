package stm

import "testing"

type issue84Struct struct {
	Name string `json:"name"`
}

func TestStructToMapTypedNilPointerReturnsNil(t *testing.T) {
	var src *issue84Struct
	if got := StructToMap(src, "json"); got != nil {
		t.Fatalf("StructToMap = %#v, want nil", got)
	}
	if got := StructToMapExtraExport(src, "json"); got != nil {
		t.Fatalf("StructToMapExtraExport = %#v, want nil", got)
	}
}

func TestStructToMapNonNilControl(t *testing.T) {
	got := StructToMap(&issue84Struct{Name: "gkit"}, "json")
	if got == nil || got["name"] != "gkit" {
		t.Fatalf("StructToMap = %#v, want map[name:gkit]", got)
	}
}
