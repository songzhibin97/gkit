package deepcopy

import (
	"reflect"

	"github.com/songzhibin97/gkit/tools"
)

// deepCopy recursively copies src into dst. The visited map tracks
// already-copied pointers so self-referential structures
// (`type N struct{ Next *N }; n := &N{}; n.Next = n`) do not recurse
// forever and stack-overflow the process.
func deepCopy(dst, src reflect.Value, visited map[uintptr]reflect.Value) {
	switch src.Kind() {
	case reflect.Interface:
		value := src.Elem()
		if !value.IsValid() {
			return
		}
		newValue := reflect.New(value.Type()).Elem()
		deepCopy(newValue, value, visited)
		dst.Set(newValue)
	case reflect.Ptr:
		if src.IsNil() {
			return
		}
		ptr := src.Pointer()
		if v, ok := visited[ptr]; ok {
			dst.Set(v)
			return
		}
		value := src.Elem()
		if !value.IsValid() {
			return
		}
		newPtr := reflect.New(value.Type())
		visited[ptr] = newPtr
		dst.Set(newPtr)
		deepCopy(dst.Elem(), value, visited)
	case reflect.Map:
		if src.IsNil() {
			return
		}
		ptr := src.Pointer()
		if v, ok := visited[ptr]; ok {
			dst.Set(v)
			return
		}
		newMap := reflect.MakeMap(src.Type())
		visited[ptr] = newMap
		dst.Set(newMap)
		for _, key := range src.MapKeys() {
			value := src.MapIndex(key)
			newValue := reflect.New(value.Type()).Elem()
			deepCopy(newValue, value, visited)
			dst.SetMapIndex(key, newValue)
		}
	case reflect.Slice:
		if src.IsNil() {
			return
		}
		newSlice := reflect.MakeSlice(src.Type(), src.Len(), src.Cap())
		dst.Set(newSlice)
		for i := 0; i < src.Len(); i++ {
			deepCopy(dst.Index(i), src.Index(i), visited)
		}
	case reflect.Struct:
		typeSrc := src.Type()
		for i := 0; i < src.NumField(); i++ {
			value := src.Field(i)
			tag := typeSrc.Field(i).Tag
			if value.CanSet() && tag.Get("gkit") != "-" {
				deepCopy(dst.Field(i), value, visited)
			}
		}
	default:
		dst.Set(src)
	}
}

func DeepCopy(dst, src interface{}) error {
	dstT, srcT := reflect.TypeOf(dst), reflect.TypeOf(src)
	if dstT != srcT {
		return tools.ErrorNoEquals
	}
	if srcT.Kind() != reflect.Ptr {
		return tools.ErrorMustPtr
	}
	dstV, srcV := reflect.ValueOf(dst).Elem(), reflect.ValueOf(src).Elem()
	if !dstV.IsValid() || !srcV.IsValid() {
		return tools.ErrorInvalidValue
	}
	deepCopy(dstV, srcV, map[uintptr]reflect.Value{})
	return nil
}

func Clone(v interface{}) interface{} {
	rv := reflect.ValueOf(v)
	visited := map[uintptr]reflect.Value{}
	if rv.Kind() == reflect.Ptr && !rv.IsNil() {
		// Allocate a new *T, register it in the visited map BEFORE
		// recursing so a self-referential field whose pointer equals the
		// caller's input resolves back to this newly-allocated pointer.
		// Without the pre-registration, deepCopy would allocate a fresh
		// inner pointer and the topological self-loop would be lost.
		dst := reflect.New(rv.Type().Elem())
		visited[rv.Pointer()] = dst
		deepCopy(dst.Elem(), rv.Elem(), visited)
		return dst.Interface()
	}
	dst := reflect.New(rv.Type())
	deepCopy(dst.Elem(), rv, visited)
	return dst.Elem().Interface()
}
