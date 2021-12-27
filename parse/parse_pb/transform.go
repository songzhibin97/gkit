package parse_pb

var PbToGoMapping = map[string]string{
	"double":   "float64",
	"float":    "float32",
	"int32":    "int32",
	"uint32":   "uint32",
	"uint64":   "uint64",
	"sint32":   "int32",
	"sint64":   "int64",
	"fixed32":  "uint32",
	"fixed64":  "uint64",
	"sfixed32": "int32",
	"sfixed64": "int64",
	"bool":     "bool",
	"string":   "string",
	"bytes":    "[]byte",
}

// PbTypeToGo go type 转化成 pb type
func PbTypeToGo(s string) string {
	if v, ok := PbToGoMapping[s]; ok {
		return v
	}
	return s
}

func addOne(a int) int {
	return a + 1
}
