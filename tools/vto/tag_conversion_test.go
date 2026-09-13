package vto

import "testing"

type issue172Number int

func (n issue172Number) Number() int { return int(n) }

type issue172Numberer interface{ Number() int }

func TestTagBindConvertsSourceInterfaceToAny(t *testing.T) {
	source := struct {
		Value issue172Numberer `json:"value"`
	}{Value: issue172Number(7)}
	for _, model := range []BindModel{FieldBind, TagBind} {
		var target struct {
			Value interface{} `json:"value"`
		}
		if err := VoToDoPlus(&target, &source, ModelParameters{Model: model}); err != nil {
			t.Fatal(err)
		}
		if target.Value != issue172Number(7) {
			t.Fatalf("model=%v value=%#v want 7", model, target.Value)
		}
	}
}

func TestTagBindKeepsLegalGoNumericConversions(t *testing.T) {
	type sourceNumber int64
	type targetNumber int64
	source := struct {
		Value sourceNumber `json:"value"`
	}{Value: -7}
	var target struct {
		Value targetNumber `json:"value"`
	}
	if err := VoToDoPlus(&target, &source, ModelParameters{Model: TagBind}); err != nil {
		t.Fatal(err)
	}
	if target.Value != -7 {
		t.Fatalf("named number=%v", target.Value)
	}
	fraction := struct{ Value float64 }{Value: 3.75}
	var integral struct{ Value int }
	if err := VoToDo(&integral, &fraction); err != nil {
		t.Fatal(err)
	}
	if integral.Value != int(fraction.Value) {
		t.Fatalf("numeric conversion=%d", integral.Value)
	}
}
