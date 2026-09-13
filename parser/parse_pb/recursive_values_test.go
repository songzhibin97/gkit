package parse_pb

import "testing"

func TestRecursiveMessagesGenerateCompilableGo(t *testing.T) {
	tests := []struct{ name, definition, consumer string }{
		{"direct", `message Node { Node next = 1; int32 value = 2; repeated Node children = 3; map<string,Node> lookup = 4; }`, `var _ = Node{Next:&Node{Value:7}, Children:[]Node{}, Lookup:map[string]Node{}}`},
		{"indirect", `message A { B next = 1; Leaf leaf = 2; } message B { C next = 1; } message C { A next = 1; } message Leaf { string name = 1; }`, `var _ = A{Next:&B{Next:&C{Next:&A{}}}, Leaf:Leaf{Name:"kept"}}`},
		{"slice-break", `message A { repeated B children = 1; } message B { A parent = 1; }`, `var _ = B{Parent:A{Children:[]B{}}}`},
		{"map-break", `message A { map<string,B> children = 1; } message B { A parent = 1; }`, `var _ = B{Parent:A{Children:map[string]B{}}}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := validatedProtoFixture(t, "syntax = \"proto3\"; package fixture;\n"+tt.definition)
			parsed, err := ParsePb(path)
			if err != nil {
				t.Fatal(err)
			}
			compileGeneratedGo(t, parsed.Generate(), "package fixture\n"+tt.consumer)
		})
	}
}
