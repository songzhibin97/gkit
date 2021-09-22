package vto

import (
	"errors"
	"reflect"
	"strconv"
)

// VoToDo 试图对象与domino对象转换,只能转相同字段且类型相同的
// dst: 目标
// src: 源位置
// 支持简单的 default模式 在基础类型增加default可以指定默认值
func VoToDo(dst interface{}, src interface{}) error {
	dstT, srcT := reflect.TypeOf(dst), reflect.TypeOf(src)
	if dstT.Kind() != reflect.Ptr || srcT.Kind() != reflect.Ptr {
		return errors.New("dst 或 src 必须是指针类型")
	}
	dstT, srcT = dstT.Elem(), srcT.Elem()
	dstV, srcV := reflect.ValueOf(dst).Elem(), reflect.ValueOf(src).Elem()
	for i := 0; i < dstT.NumField(); i++ {
		field := dstT.Field(i)
		defaultTag := field.Tag.Get("default")
		if !field.IsExported() {
			continue
		}
		if _, ok := srcT.FieldByName(field.Name); ok {
			d := dstV.Field(i)
			s := srcV.FieldByName(field.Name)
			for s.Kind() == reflect.Ptr && d.Kind() != s.Kind() {
				s = s.Elem()
			}
			if d.Kind() == s.Kind() {
				if s.IsZero() && len(defaultTag) > 0 && d.Kind() == reflect.Ptr {
					v, err := ptrDefaultPtr(d, defaultTag)
					if err != nil {
						return err
					}
					d.Set(v)
				} else if s.IsZero() && len(defaultTag) > 0 {
					err := bindDefault(d, defaultTag)
					if err != nil {
						return err
					}
				} else {
					d.Set(s)
				}

			}
		}
	}
	return nil
}
func bindInt(value reflect.Value, df string) (v int64, err error) {
	switch value.Kind() {
	case reflect.Ptr:
		value = value.Elem()
	case reflect.Int8:
		v, err = strconv.ParseInt(df, 10, 8)
	case reflect.Int16:
		v, err = strconv.ParseInt(df, 10, 16)
	case reflect.Int32:
		v, err = strconv.ParseInt(df, 10, 32)
	case reflect.Int64:
		v, err = strconv.ParseInt(df, 10, 64)
	case reflect.Int:
		v, err = strconv.ParseInt(df, 10, 64)
	}
	return
}

func bindIntValue(value reflect.Value, df string) (reflect.Value, error) {
	switch value.Type().String() {
	case "*int8":
		vv, err := strconv.ParseInt(df, 10, 8)
		if err != nil {
			return reflect.Value{}, err
		}
		v := int8(vv)
		return reflect.ValueOf(&v), nil
	case "*int16":
		vv, err := strconv.ParseInt(df, 10, 16)
		if err != nil {
			return reflect.Value{}, err
		}
		v := int16(vv)
		return reflect.ValueOf(&v), nil

	case "*int32":
		vv, err := strconv.ParseInt(df, 10, 32)
		if err != nil {
			return reflect.Value{}, err
		}
		v := int32(vv)

		return reflect.ValueOf(&v), nil

	case "*int64":
		vv, err := strconv.ParseInt(df, 10, 64)
		if err != nil {
			return reflect.Value{}, err
		}

		return reflect.ValueOf(&vv), nil

	case "*int":
		vv, err := strconv.ParseInt(df, 10, 64)
		if err != nil {
			return reflect.Value{}, err
		}
		v := int(vv)
		return reflect.ValueOf(&v), nil
	}
	return reflect.Value{}, nil
}

func bindFloat(value reflect.Value, df string) (v float64, err error) {
	switch value.Kind() {
	case reflect.Float32:
		v, err = strconv.ParseFloat(df, 32)
	case reflect.Float64:
		v, err = strconv.ParseFloat(df, 64)
	}
	return
}

func bindFloatValue(value reflect.Value, df string) (reflect.Value, error) {
	switch value.Type().String() {
	case "*float32":
		v, err := strconv.ParseFloat(df, 32)
		if err != nil {
			return reflect.Value{}, err
		}
		vv := float32(v)
		return reflect.ValueOf(&vv), nil
	case "*float64":
		v, err := strconv.ParseFloat(df, 64)
		if err != nil {
			return reflect.Value{}, err
		}
		return reflect.ValueOf(&v), nil
	}
	return reflect.Value{}, nil
}

func bindDefault(value reflect.Value, df string) error {
	switch value.Kind() {
	case reflect.String:
		value.SetString(df)
	case reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64, reflect.Int:
		v, err := bindInt(value, df)
		if err != nil {
			return err
		}
		value.SetInt(v)
	case reflect.Bool:
		switch df {
		case "n", "no", "0", "false":
			value.SetBool(false)
		case "y", "yes", "1", "true":
			value.SetBool(true)
		default:
			return errors.New("bool matching failed")
		}
	case reflect.Float32, reflect.Float64:
		v, err := bindFloat(value, df)
		if err != nil {
			return err
		}
		value.SetFloat(v)
	}
	return nil
}

// return *reflect.Value
func ptrDefaultPtr(value reflect.Value, df string) (reflect.Value, error) {

	var (
		rf  reflect.Value
		err error
	)
	switch value.Type().String() {
	case "*string":
		rf = reflect.ValueOf(&df)

	case "*int8", "*int16", "*int32", "*int64", "*int":
		rf, err = bindIntValue(value, df)
		if err != nil {
			return reflect.Value{}, err
		}
	case "*bool":
		var b bool
		switch df {
		case "n", "no", "0", "false":

		case "y", "yes", "1", "true":
			b = true
		default:
			return reflect.Value{}, errors.New("bool matching failed")
		}
		rf = reflect.ValueOf(&b)

	case "*float32", "*float64":

		rf, err = bindFloatValue(value, df)
		if err != nil {
			return reflect.Value{}, err
		}
	}
	return rf, nil
}

