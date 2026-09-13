package parse_pb

import (
	"strings"

	"github.com/emicklei/proto"
)

// Index declarations before fields so forward and nested references use the
// same flattened names as their generated Go declarations.
func indexTypes(elements []proto.Visitee, scope, prefix string, names map[string]string) {
	for _, element := range elements {
		switch v := element.(type) {
		case *proto.Message:
			name := strings.TrimPrefix(scope+"."+v.Name, ".")
			names[name] = prefix + v.Name
			indexTypes(v.Elements, name, prefix+v.Name, names)
		case *proto.Enum:
			names[strings.TrimPrefix(scope+"."+v.Name, ".")] = prefix + v.Name
		}
	}
}

func resolveType(name, scope string, names map[string]string) string {
	if strings.HasPrefix(name, ".") {
		if resolved, ok := names[strings.TrimPrefix(name, ".")]; ok {
			return resolved
		}
		return PbTypeToGo(name)
	}
	for {
		candidate := strings.TrimPrefix(scope+"."+name, ".")
		if resolved, ok := names[candidate]; ok {
			return resolved
		}
		if scope == "" {
			break
		}
		if dot := strings.LastIndex(scope, "."); dot >= 0 {
			scope = scope[:dot]
		} else {
			scope = ""
		}
	}
	return PbTypeToGo(name)
}
