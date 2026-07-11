package task

import (
	"errors"
	"testing"
)

func TestWorkflowConstructorsRejectInvalidDefinitionsWithoutMutation(t *testing.T) {
	callback := &Signature{ID: "callback"}

	tests := []struct {
		name string
		run  func(*testing.T) error
	}{
		{name: "empty chain", run: func(*testing.T) error {
			_, err := NewChain("chain")
			return err
		}},
		{name: "nil chain member", run: func(t *testing.T) error {
			first := &Signature{ID: "first"}
			_, err := NewChain("chain", first, nil)
			if first.CallbackOnSuccess != nil {
				t.Fatal("NewChain mutated callbacks before rejecting the definition")
			}
			return err
		}},
		{name: "empty group", run: func(*testing.T) error {
			_, err := NewGroup("group-id", "group")
			return err
		}},
		{name: "nil group member", run: func(t *testing.T) error {
			first := &Signature{ID: "first", GroupID: "original", GroupTaskCount: 7}
			_, err := NewGroup("group-id", "group", first, nil)
			if first.GroupID != "original" || first.GroupTaskCount != 7 {
				t.Fatal("NewGroup mutated metadata before rejecting the definition")
			}
			return err
		}},
		{name: "nil group callback group", run: func(*testing.T) error {
			_, err := NewGroupCallback(nil, "chord", callback)
			return err
		}},
		{name: "empty group callback group", run: func(*testing.T) error {
			_, err := NewGroupCallback(&Group{GroupID: "group-id"}, "chord", callback)
			return err
		}},
		{name: "nil group callback", run: func(t *testing.T) error {
			member := &Signature{ID: "member"}
			_, err := NewGroupCallback(&Group{GroupID: "group-id", Tasks: []*Signature{member}}, "chord", nil)
			if member.CallbackChord != nil {
				t.Fatal("NewGroupCallback mutated member before rejecting the definition")
			}
			return err
		}},
		{name: "nil group callback member", run: func(t *testing.T) error {
			member := &Signature{ID: "member"}
			_, err := NewGroupCallback(&Group{GroupID: "group-id", Tasks: []*Signature{member, nil}}, "chord", callback)
			if member.CallbackChord != nil {
				t.Fatal("NewGroupCallback partially mutated members before rejecting the definition")
			}
			return err
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var err error
			func() {
				defer func() {
					if recovered := recover(); recovered != nil {
						t.Fatalf("constructor panicked: %v", recovered)
					}
				}()
				err = tt.run(t)
			}()
			if !errors.Is(err, ErrInvalidWorkflow) {
				t.Fatalf("error = %v, want ErrInvalidWorkflow", err)
			}
		})
	}
}

func TestValidateGroupCallbackRejectsNilValue(t *testing.T) {
	var err error
	func() {
		defer func() {
			if recovered := recover(); recovered != nil {
				t.Fatalf("ValidateGroupCallback panicked: %v", recovered)
			}
		}()
		err = ValidateGroupCallback(nil)
	}()
	if !errors.Is(err, ErrInvalidWorkflow) {
		t.Fatalf("error = %v, want ErrInvalidWorkflow", err)
	}
}

func TestWorkflowConstructorsPreserveValidBehavior(t *testing.T) {
	first := &Signature{ID: "first"}
	second := &Signature{ID: "second"}
	chain, err := NewChain("chain", first, second)
	if err != nil {
		t.Fatalf("NewChain returned error: %v", err)
	}
	if len(chain.Tasks) != 2 || len(first.CallbackOnSuccess) != 1 || first.CallbackOnSuccess[0] != second {
		t.Fatal("NewChain did not preserve callback wiring")
	}

	group, err := NewGroup("group-id", "group", first, second)
	if err != nil {
		t.Fatalf("NewGroup returned error: %v", err)
	}
	if first.GroupID != "group-id" || second.GroupID != "group-id" || first.GroupTaskCount != 2 || second.GroupTaskCount != 2 {
		t.Fatal("NewGroup did not preserve group metadata")
	}

	callback := &Signature{ID: "callback"}
	chord, err := NewGroupCallback(group, "chord", callback)
	if err != nil {
		t.Fatalf("NewGroupCallback returned error: %v", err)
	}
	if chord.Callback != callback || first.CallbackChord != callback || second.CallbackChord != callback {
		t.Fatal("NewGroupCallback did not preserve callback wiring")
	}
}
