package bind

import "testing"

type validatorTarget struct {
	Name string `binding:"required"`
}

// TestValidateStruct_TypedNilPointer covers I-ll: previously
// ValidateStruct on a typed nil pointer panicked deep in
// reflect.Value.Interface, violating the documented "should never panic"
// contract.
func TestValidateStruct_TypedNilPointer(t *testing.T) {
	v := &defaultValidator{}
	var p *validatorTarget // typed nil
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("ValidateStruct panicked on typed nil pointer: %v", r)
		}
	}()
	if err := v.ValidateStruct(p); err != nil {
		t.Fatalf("ValidateStruct on typed nil pointer err = %v, want nil", err)
	}
}
