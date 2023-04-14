package bind

import (
	"errors"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/songzhibin97/gkit/tools/bind/internal/bytesconv"
	"github.com/songzhibin97/gkit/tools/bind/internal/json"
)

var errUnknownType = errors.New("unknown type")

func mapUri(ptr interface{}, m map[string][]string) error {
	return mapFormByTag(ptr, m, "uri")
}

func mapForm(ptr interface{}, form map[string][]string) error {
	return mapFormByTag(ptr, form, "form")
}

var emptyField = reflect.StructField{}

func mapFormByTag(ptr interface{}, form map[string][]string, tag string) error {
	ptrVal := reflect.ValueOf(ptr)
	var pointed interface{}
	// 如果 ptr 是指针
	if ptrVal.Kind() == reflect.Ptr {
		// 获取指针内下的值 并且记录一下 当前类型的 interface模式存储在 pointed 位置
		ptrVal = ptrVal.Elem()
		pointed = ptrVal.Interface()
	}
	if ptrVal.Kind() == reflect.Map &&
		ptrVal.Type().Key().Kind() == reflect.String {
		// 如果 ptr 是map类型 && 并且 key 是 string 的情况下
		if pointed != nil {
			// 如果上面 pointed 不是nil 有值的情况下 ptr = pointed 赋值
			ptr = pointed
		}
		return setFormMap(ptr, form)
	}

	return mappingByPtr(ptr, formSource(form), tag)
}

// setter tries to set value on a walking by fields of a struct
type setter interface {
	TrySet(value reflect.Value, field reflect.StructField, key string, opt setOptions) (isSetted bool, err error)
}

type formSource map[string][]string

var _ setter = formSource(nil)

// TrySet tries to set a value by request's form source (like map[string][]string)
func (form formSource) TrySet(value reflect.Value, field reflect.StructField, tagValue string, opt setOptions) (isSetted bool, err error) {
	return setByForm(value, field, form, tagValue, opt)
}

func mappingByPtr(ptr interface{}, setter setter, tag string) error {
	// emptyField 空的结构体字段
	_, err := mapping(reflect.ValueOf(ptr), emptyField, setter, tag)
	return err
}

func mapping(value reflect.Value, field reflect.StructField, setter setter, tag string) (bool, error) {
	// 获取 tag, 如果是 "-" 忽略此字段
	if field.Tag.Get(tag) == "-" {
		return false, nil
	}

	vKind := value.Kind()

	// obj kind 如果是指针
	if vKind == reflect.Ptr {
		var isNew bool
		vPtr := value
		if value.IsNil() {
			// 如果是空指针
			// 重新赋值 reflect.New(value.Type().Elem())
			isNew = true
			vPtr = reflect.New(value.Type().Elem())
		}
		isSetted, err := mapping(vPtr.Elem(), field, setter, tag)
		if err != nil {
			return false, err
		}
		// 如果是新创建的 && 递归成功赋值回去
		if isNew && isSetted {
			value.Set(vPtr)
		}
		return isSetted, nil
	}

	// obj kind 不是 struct 或者 不是匿名
	if vKind != reflect.Struct || !field.Anonymous {
		// 尝试更新 value
		ok, err := tryToSetValue(value, field, setter, tag)
		if err != nil {
			return false, err
		}
		if ok {
			return true, nil
		}
	}

	// obj kind 是 struct
	if vKind == reflect.Struct {
		tValue := value.Type()

		// isSetted flag
		var isSetted bool
		for i := 0; i < value.NumField(); i++ {

			sf := tValue.Field(i)
			if sf.PkgPath != "" && !sf.Anonymous {
				// 未导出内容
				continue
			}
			// 递归子字段
			ok, err := mapping(value.Field(i), tValue.Field(i), setter, tag)
			if err != nil {
				return false, err
			}
			isSetted = isSetted || ok
		}
		return isSetted, nil
	}
	return false, nil
}

type setOptions struct {
	isDefaultExists bool
	defaultValue    string
}

// tryToSetValue 尝试设置value
func tryToSetValue(value reflect.Value, field reflect.StructField, setter setter, tag string) (bool, error) {
	var tagValue string
	var setOpt setOptions

	// 获取tag
	tagValue = field.Tag.Get(tag)
	// 处理tag,截取第一个有效的 tagValue, opt 将后面剩余的全部返回
	tagValue, opts := head(tagValue, ",")

	// 如果 tagValue 是空的 就用自己的 字段name
	if tagValue == "" {
		tagValue = field.Name
	}
	// 如果还是没有的话... 那就是 emptyField
	if tagValue == "" {
		return false, nil
	}

	var opt string
	// 如果好友其他 tag
	for len(opts) > 0 {
		// 再去尝试拿一个 挂到opt上
		opt, opts = head(opts, ",")
		// 如果有 default 设置 opt设置为 default模式
		if k, v := head(opt, "="); k == "default" {
			setOpt.isDefaultExists = true
			setOpt.defaultValue = v
		}
	}
	// 尝试设置
	return setter.TrySet(value, field, tagValue, setOpt)
}

// setByForm 设置 from 模式
func setByForm(value reflect.Value, field reflect.StructField, form map[string][]string, tagValue string, opt setOptions) (isSetted bool, err error) {
	// 如果tag不存在from或者是没有默认模式 截断返回
	vs, ok := form[tagValue]
	if !ok && !opt.isDefaultExists {
		return false, nil
	}

	// 判断 value 的类型 处理
	switch value.Kind() {
	case reflect.Slice:
		if !ok {
			vs = []string{opt.defaultValue}
		}
		return true, SetSlice(vs, value, field)
	case reflect.Array:
		if !ok {
			vs = []string{opt.defaultValue}
		}
		if len(vs) != value.Len() {
			return false, fmt.Errorf("%q is not valid value for %s", vs, value.Type().String())
		}
		return true, SetArray(vs, value, field)
	default:
		var val string
		if !ok {
			val = opt.defaultValue
		}

		if len(vs) > 0 {
			val = vs[0]
		}
		return true, SetWithProperType(val, value, field)
	}
}

func SetWithProperType(val string, value reflect.Value, field reflect.StructField) error {
	switch value.Kind() {
	case reflect.Int:
		return setIntField(val, 0, value)
	case reflect.Int8:
		return setIntField(val, 8, value)
	case reflect.Int16:
		return setIntField(val, 16, value)
	case reflect.Int32:
		return setIntField(val, 32, value)
	case reflect.Int64:
		switch value.Interface().(type) {
		case time.Duration:
			return setTimeDuration(val, value)
		}
		return setIntField(val, 64, value)
	case reflect.Uint:
		return setUintField(val, 0, value)
	case reflect.Uint8:
		return setUintField(val, 8, value)
	case reflect.Uint16:
		return setUintField(val, 16, value)
	case reflect.Uint32:
		return setUintField(val, 32, value)
	case reflect.Uint64:
		return setUintField(val, 64, value)
	case reflect.Bool:
		return setBoolField(val, value)
	case reflect.Float32:
		return setFloatField(val, 32, value)
	case reflect.Float64:
		return setFloatField(val, 64, value)
	case reflect.String:
		value.SetString(val)
	case reflect.Struct:
		switch value.Interface().(type) {
		case time.Time:
			return setTimeField(val, field, value)
		}
		return json.Unmarshal(bytesconv.StringToBytes(val), value.Addr().Interface())
	case reflect.Map:
		return json.Unmarshal(bytesconv.StringToBytes(val), value.Addr().Interface())
	case reflect.Slice:
		return json.Unmarshal(bytesconv.StringToBytes(val), value.Addr().Interface())
	case reflect.Array:
		return json.Unmarshal(bytesconv.StringToBytes(val), value.Addr().Interface())
	default:
		return errUnknownType
	}
	return nil
}

func setIntField(val string, bitSize int, field reflect.Value) error {
	if val == "" {
		val = "0"
	}
	intVal, err := strconv.ParseInt(val, 10, bitSize)
	if err == nil {
		field.SetInt(intVal)
	}
	return err
}

func setUintField(val string, bitSize int, field reflect.Value) error {
	if val == "" {
		val = "0"
	}
	uintVal, err := strconv.ParseUint(val, 10, bitSize)
	if err == nil {
		field.SetUint(uintVal)
	}
	return err
}

func setBoolField(val string, field reflect.Value) error {
	if val == "" {
		val = "false"
	}
	boolVal, err := strconv.ParseBool(val)
	if err == nil {
		field.SetBool(boolVal)
	}
	return err
}

func setFloatField(val string, bitSize int, field reflect.Value) error {
	if val == "" {
		val = "0.0"
	}
	floatVal, err := strconv.ParseFloat(val, bitSize)
	if err == nil {
		field.SetFloat(floatVal)
	}
	return err
}

func setTimeField(val string, structField reflect.StructField, value reflect.Value) error {
	timeFormat := structField.Tag.Get("time_format")
	if timeFormat == "" {
		timeFormat = time.RFC3339
	}

	switch tf := strings.ToLower(timeFormat); tf {
	case "unix", "unixnano":
		tv, err := strconv.ParseInt(val, 10, 64)
		if err != nil {
			return err
		}

		d := time.Duration(1)
		if tf == "unixnano" {
			d = time.Second
		}

		t := time.Unix(tv/int64(d), tv%int64(d))
		value.Set(reflect.ValueOf(t))
		return nil
	}

	if val == "" {
		value.Set(reflect.ValueOf(time.Time{}))
		return nil
	}

	l := time.Local
	if isUTC, _ := strconv.ParseBool(structField.Tag.Get("time_utc")); isUTC {
		l = time.UTC
	}

	if locTag := structField.Tag.Get("time_location"); locTag != "" {
		loc, err := time.LoadLocation(locTag)
		if err != nil {
			return err
		}
		l = loc
	}

	t, err := time.ParseInLocation(timeFormat, val, l)
	if err != nil {
		return err
	}

	value.Set(reflect.ValueOf(t))
	return nil
}

func SetArray(vals []string, value reflect.Value, field reflect.StructField) error {
	for i, s := range vals {
		err := SetWithProperType(s, value.Index(i), field)
		if err != nil {
			return err
		}
	}
	return nil
}

func SetSlice(vals []string, value reflect.Value, field reflect.StructField) error {
	slice := reflect.MakeSlice(value.Type(), len(vals), len(vals))
	err := SetArray(vals, slice, field)
	if err != nil {
		return err
	}
	value.Set(slice)
	return nil
}

func setTimeDuration(val string, value reflect.Value) error {
	d, err := time.ParseDuration(val)
	if err != nil {
		return err
	}
	value.Set(reflect.ValueOf(d))
	return nil
}

// eg: head("t1",",") -> t1,""
// eg: head("t1,t2") -> t1,"t2"
// eg: head("t1,t2,t3") -> t1,"t2,t3"
func head(str, sep string) (head string, tail string) {
	idx := strings.Index(str, sep)
	if idx < 0 {
		return str, ""
	}
	return str[:idx], str[idx+len(sep):]
}

func setFormMap(ptr interface{}, form map[string][]string) error {
	// el 获取到 key的类型
	el := reflect.TypeOf(ptr).Elem()

	// 如果是 slice
	if el.Kind() == reflect.Slice {
		// 简单断言一下 和 from 类型是否是一致的
		ptrMap, ok := ptr.(map[string][]string)
		if !ok {
			return errors.New("cannot convert to map slices of strings")
		}
		// 一致赋值
		for k, v := range form {
			ptrMap[k] = v
		}

		return nil
	}
	// 断言 是否是 key:value形式
	ptrMap, ok := ptr.(map[string]string)
	if !ok {
		return errors.New("cannot convert to map of strings")
	}
	for k, v := range form {
		ptrMap[k] = v[len(v)-1] // pick last
	}

	return nil
}
