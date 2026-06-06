package reflect2value

import "fmt"

type errNonsupportType struct {
	valueType string
}

func NewErrNonsupportType(valueType string) error {
	return &errNonsupportType{valueType: valueType}
}

func (e *errNonsupportType) Error() string {
	return e.valueType + ":不是支持类型"
}

// ErrOverflow is returned when a numeric value cannot fit into the target
// type without truncation. Previously SetInt/SetUint/SetFloat silently
// stored a truncated value (e.g. 200 into int8 → -56).
type ErrOverflow struct {
	TargetType string
	Value      int64
	Float      float64
	IsFloat    bool
}

func NewErrOverflow(targetType string, value int64) error {
	return &ErrOverflow{TargetType: targetType, Value: value}
}

func NewErrOverflowFloat(targetType string, value float64) error {
	return &ErrOverflow{TargetType: targetType, Float: value, IsFloat: true}
}

func (e *ErrOverflow) Error() string {
	if e.IsFloat {
		return fmt.Sprintf("reflect2value: value %g overflows %s", e.Float, e.TargetType)
	}
	return fmt.Sprintf("reflect2value: value %d overflows %s", e.Value, e.TargetType)
}
