package parse_pb

import "testing"

func TestNestedReferencesGenerateCompilableGo(t *testing.T) {
	path := validatedProtoFixture(t, `syntax = "proto3";
package fixture;
service Reader { rpc Read (A.B.C) returns (.fixture.A.B.C); }
message A {
  message B {
    message C { string value = 1; }
    enum D { D_UNSPECIFIED = 0; D_READY = 1; }
    C child = 1;
    D state = 2;
    repeated C children = 3;
    map<string, C> lookup = 4;
  }
  B inner = 1;
  B.C child = 2;
  B.D state = 3;
}
message Outside { A.B.C child = 1; .fixture.A.B.D state = 2; }
`)
	parsed, err := ParsePb(path)
	if err != nil {
		t.Fatal(err)
	}
	compileGeneratedGo(t, parsed.Generate(), `package fixture
var _ = A{Inner: AB{Child: ABC{Value:"x"}, State:D_READY, Children:[]ABC{{Value:"y"}}, Lookup:map[string]ABC{"key":{Value:"z"}}}, Child:ABC{}, State:D_UNSPECIFIED}
var _ = Outside{Child:ABC{},State:D_READY}
var _ func(ABC) ABC = Read
`)
	model := parsed.(*PbParseGo)
	if len(model.Message["AB"].Files) != 4 {
		t.Fatalf("enum declaration became a field: %#v", model.Message["AB"].Files)
	}
}
