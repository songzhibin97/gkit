package parse_pb

import (
	"strings"

	"github.com/emicklei/proto"
)

// Index declarations before fields so forward and nested references use the
// same flattened names as their generated Go declarations.
func (p *PbParseGo) indexTypes(elements []proto.Visitee, scope, prefix string) {
	for _, element := range elements {
		switch v := element.(type) {
		case *proto.Message:
			name := strings.TrimPrefix(scope+"."+v.Name, ".")
			p.typeNames[name] = prefix + v.Name
			p.indexTypes(v.Elements, name, prefix+v.Name)
		case *proto.Enum:
			p.typeNames[strings.TrimPrefix(scope+"."+v.Name, ".")] = prefix + v.Name
		}
	}
}

func (p *PbParseGo) resolveType(name, scope string) string {
	if strings.HasPrefix(name, ".") {
		if resolved, ok := p.typeNames[strings.TrimPrefix(name, ".")]; ok {
			return resolved
		}
		return PbTypeToGo(name)
	}
	for {
		candidate := strings.TrimPrefix(scope+"."+name, ".")
		if resolved, ok := p.typeNames[candidate]; ok {
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
