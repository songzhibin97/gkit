package parse_go

var GoToPBMapping = map[string]string{
	"int":     "int64",
	"float":   "double",
	"int16":   "int32",
	"float16": "double",
	"float64": "double",
	"float32": "float",
	"int32":   "int32",
	"int64":   "int64",
	"uint32":  "uint32",
	"uint64":  "uint64",
	"bool":    "bool",
	"string":  "string",
	"[]byte":  "bytes",
}

// GoTypeToPB go type 转化成 pb type
func GoTypeToPB(s string) string {
	if v, ok := GoToPBMapping[s]; ok {
		return v
	}
	return s
}

// IsMappingKey 判断是否是 pb map的key类型
func IsMappingKey(key string) bool {
	// Map key cannot be float, double, bytes, message, or enum types
	switch key {
	case "int32", "int64", "uint32", "uint64", "sint32", "sint64", "fixed32", "fixed64", "sfixed32", "sfixed64", "string":
		return true
	default:
		return false
	}
}

func addOne(a int) int {
	return a + 1
}
