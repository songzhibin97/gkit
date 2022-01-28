package reflect2value

type errNonsupportType struct {
	valueType string
}

func NewErrNonsupportType(valueType string) error {
	return &errNonsupportType{valueType: valueType}
}

func (e *errNonsupportType) Error() string {
	return e.valueType + ":不是支持类型"
}
