package jsonutil

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

// Marshal 将 Go 值序列化为 JSON 字节数组
func Marshal(v interface{}) ([]byte, error) {
	jsonStr, err := MarshalString(v)
	if err != nil {
		return nil, err
	}
	return []byte(jsonStr), nil
}

// MarshalString 将 Go 值序列化为 JSON 字符串
func MarshalString(v interface{}) (string, error) {
	return marshalValue(v, 0)
}

// Unmarshal 将 JSON 字节数组反序列化为 Go 值
func Unmarshal(data []byte, v interface{}) error {
	return UnmarshalString(string(data), v)
}

// UnmarshalString 将 JSON 字符串反序列化为 Go 值
func UnmarshalString(jsonStr string, v interface{}) error {
	val := reflect.ValueOf(v)
	if val.Kind() != reflect.Ptr || val.IsNil() {
		return fmt.Errorf("unmarshal target must be a non-nil pointer")
	}

	elem := val.Elem()
	return unmarshalValue(jsonStr, "", elem)
}

func marshalValue(v interface{}, depth int) (string, error) {
	if depth > 32 {
		return "", fmt.Errorf("maximum nesting depth exceeded")
	}

	if v == nil {
		return "null", nil
	}

	val := reflect.ValueOf(v)
	typ := val.Type()

	switch typ.Kind() {
	case reflect.String:
		return fmt.Sprintf(`"%s"`, escapeString(val.String())), nil

	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		if typ == reflect.TypeOf(time.Duration(0)) {
			// time.Duration 序列化为纳秒数值（符合项目规范）
			return strconv.FormatInt(val.Int(), 10), nil
		}
		return strconv.FormatInt(val.Int(), 10), nil

	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return strconv.FormatUint(val.Uint(), 10), nil

	case reflect.Float32, reflect.Float64:
		return strconv.FormatFloat(val.Float(), 'f', -1, 64), nil

	case reflect.Bool:
		if val.Bool() {
			return "true", nil
		}
		return "false", nil

	case reflect.Slice, reflect.Array:
		return marshalSlice(val, depth+1)

	case reflect.Map:
		return marshalMap(val, depth+1)

	case reflect.Struct:
		return marshalStruct(val, depth+1)

	case reflect.Ptr:
		if val.IsNil() {
			return "null", nil
		}
		return marshalValue(val.Elem().Interface(), depth)

	case reflect.Interface:
		if val.IsNil() {
			return "null", nil
		}
		return marshalValue(val.Elem().Interface(), depth)

	default:
		return "", fmt.Errorf("unsupported type: %s", typ.Kind())
	}
}

func marshalSlice(val reflect.Value, depth int) (string, error) {
	if val.IsNil() {
		return "null", nil
	}

	result := "[]"
	for i := 0; i < val.Len(); i++ {
		elem := val.Index(i).Interface()
		elemJSON, err := marshalValue(elem, depth)
		if err != nil {
			return "", err
		}

		var err2 error
		result, err2 = sjson.Set(result, strconv.Itoa(i), gjson.Parse(elemJSON).Value())
		if err2 != nil {
			return "", err2
		}
	}

	return result, nil
}

func marshalMap(val reflect.Value, depth int) (string, error) {
	if val.IsNil() {
		return "null", nil
	}

	result := "{}"
	for _, key := range val.MapKeys() {
		keyStr := fmt.Sprintf("%v", key.Interface())
		value := val.MapIndex(key).Interface()

		valueJSON, err := marshalValue(value, depth)
		if err != nil {
			return "", err
		}

		var err2 error
		result, err2 = sjson.Set(result, keyStr, gjson.Parse(valueJSON).Value())
		if err2 != nil {
			return "", err2
		}
	}

	return result, nil
}

func marshalStruct(val reflect.Value, depth int) (string, error) {
	typ := val.Type()
	result := "{}"

	for i := 0; i < val.NumField(); i++ {
		field := val.Field(i)
		fieldType := typ.Field(i)

		// 跳过非导出字段
		if !field.CanInterface() {
			continue
		}

		// 获取 JSON 标签
		tag := fieldType.Tag.Get("json")
		if tag == "-" {
			continue
		}

		fieldName := fieldType.Name
		omitEmpty := false

		if tag != "" {
			parts := strings.Split(tag, ",")
			if parts[0] != "" {
				fieldName = parts[0]
			}
			for _, part := range parts[1:] {
				if part == "omitempty" {
					omitEmpty = true
				}
			}
		}

		// 处理 omitempty
		if omitEmpty && isEmptyValue(field) {
			continue
		}

		// 特殊处理 time.Duration
		if fieldType.Type == reflect.TypeOf(time.Duration(0)) {
			duration := field.Interface().(time.Duration)
			var err error
			result, err = sjson.Set(result, fieldName, int64(duration))
			if err != nil {
				return "", err
			}
			continue
		}

		// 序列化字段值
		fieldJSON, err := marshalValue(field.Interface(), depth)
		if err != nil {
			return "", err
		}

		var err2 error
		result, err2 = sjson.Set(result, fieldName, gjson.Parse(fieldJSON).Value())
		if err2 != nil {
			return "", err2
		}
	}

	return result, nil
}

func unmarshalValue(jsonStr string, path string, val reflect.Value) error {
	var result gjson.Result
	if path == "" {
		result = gjson.Parse(jsonStr)
	} else {
		result = gjson.Get(jsonStr, path)
	}

	if !result.Exists() && path != "" {
		return nil // 字段不存在，跳过
	}

	switch val.Kind() {
	case reflect.String:
		if result.Type == gjson.String {
			val.SetString(result.String())
		} else if result.Type == gjson.Null {
			val.SetString("")
		} else {
			val.SetString(result.Raw)
		}

	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		if val.Type() == reflect.TypeOf(time.Duration(0)) {
			// 处理 time.Duration - 支持纳秒数值格式（符合项目规范）
			if result.Type == gjson.Number {
				val.Set(reflect.ValueOf(time.Duration(result.Int())))
			}
		} else {
			if result.Type == gjson.Number {
				val.SetInt(result.Int())
			}
		}

	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		if result.Type == gjson.Number {
			val.SetUint(result.Uint())
		}

	case reflect.Float32, reflect.Float64:
		if result.Type == gjson.Number {
			val.SetFloat(result.Float())
		}

	case reflect.Bool:
		if result.Type == gjson.True {
			val.SetBool(true)
		} else if result.Type == gjson.False {
			val.SetBool(false)
		}

	case reflect.Slice:
		return unmarshalSlice(result, val)

	case reflect.Map:
		return unmarshalMap(result, val)

	case reflect.Struct:
		return unmarshalStruct(result, val)

	case reflect.Ptr:
		if result.Type == gjson.Null {
			val.Set(reflect.Zero(val.Type()))
			return nil
		}

		// 分配新值
		elem := reflect.New(val.Type().Elem())
		if err := unmarshalValue(result.Raw, "", elem.Elem()); err != nil {
			return err
		}
		val.Set(elem)

	case reflect.Interface:
		return unmarshalInterface(result, val)

	default:
		return fmt.Errorf("unsupported type: %s", val.Kind())
	}

	return nil
}

func unmarshalSlice(result gjson.Result, val reflect.Value) error {
	if result.Type == gjson.Null {
		val.Set(reflect.Zero(val.Type()))
		return nil
	}

	if result.Type != gjson.JSON || !result.IsArray() {
		return fmt.Errorf("expected array, got %s", result.Type)
	}

	array := result.Array()
	slice := reflect.MakeSlice(val.Type(), len(array), len(array))

	for i, elem := range array {
		if err := unmarshalValue(elem.Raw, "", slice.Index(i)); err != nil {
			return err
		}
	}

	val.Set(slice)
	return nil
}

func unmarshalMap(result gjson.Result, val reflect.Value) error {
	if result.Type == gjson.Null {
		val.Set(reflect.Zero(val.Type()))
		return nil
	}

	if result.Type != gjson.JSON || !result.IsObject() {
		return fmt.Errorf("expected object, got %s", result.Type)
	}

	mapVal := reflect.MakeMap(val.Type())

	result.ForEach(func(key, value gjson.Result) bool {
		keyVal := reflect.New(val.Type().Key()).Elem()
		valueVal := reflect.New(val.Type().Elem()).Elem()

		// 设置键
		if err := unmarshalValue(key.Raw, "", keyVal); err != nil {
			return false
		}

		// 设置值
		if err := unmarshalValue(value.Raw, "", valueVal); err != nil {
			return false
		}

		mapVal.SetMapIndex(keyVal, valueVal)
		return true
	})

	val.Set(mapVal)
	return nil
}

func unmarshalStruct(result gjson.Result, val reflect.Value) error {
	if result.Type == gjson.Null {
		return nil
	}

	if result.Type != gjson.JSON || !result.IsObject() {
		return fmt.Errorf("expected object, got %s", result.Type)
	}

	typ := val.Type()

	for i := 0; i < val.NumField(); i++ {
		field := val.Field(i)
		fieldType := typ.Field(i)

		if !field.CanSet() {
			continue
		}

		// 获取 JSON 字段名
		fieldName := fieldType.Name
		tag := fieldType.Tag.Get("json")
		if tag != "" && tag != "-" {
			parts := strings.Split(tag, ",")
			if parts[0] != "" {
				fieldName = parts[0]
			}
		}

		// 获取字段值
		fieldResult := gjson.Get(result.Raw, fieldName)
		if !fieldResult.Exists() {
			continue
		}

		if err := unmarshalValue(fieldResult.Raw, "", field); err != nil {
			return err
		}
	}

	return nil
}

func unmarshalInterface(result gjson.Result, val reflect.Value) error {
	switch result.Type {
	case gjson.String:
		val.Set(reflect.ValueOf(result.String()))
	case gjson.Number:
		if strings.Contains(result.Raw, ".") {
			val.Set(reflect.ValueOf(result.Float()))
		} else {
			val.Set(reflect.ValueOf(result.Int()))
		}
	case gjson.True:
		val.Set(reflect.ValueOf(true))
	case gjson.False:
		val.Set(reflect.ValueOf(false))
	case gjson.Null:
		val.Set(reflect.Zero(val.Type()))
	case gjson.JSON:
		if result.IsArray() {
			var slice []interface{}
			for _, elem := range result.Array() {
				var item interface{}
				if err := unmarshalValue(elem.Raw, "", reflect.ValueOf(&item).Elem()); err != nil {
					return err
				}
				slice = append(slice, item)
			}
			val.Set(reflect.ValueOf(slice))
		} else if result.IsObject() {
			mapVal := make(map[string]interface{})
			result.ForEach(func(key, value gjson.Result) bool {
				var item interface{}
				if err := unmarshalValue(value.Raw, "", reflect.ValueOf(&item).Elem()); err != nil {
					return false
				}
				mapVal[key.String()] = item
				return true
			})
			val.Set(reflect.ValueOf(mapVal))
		}
	}

	return nil
}

// 辅助函数

func isEmptyValue(v reflect.Value) bool {
	switch v.Kind() {
	case reflect.Array, reflect.Map, reflect.Slice, reflect.String:
		return v.Len() == 0
	case reflect.Bool:
		return !v.Bool()
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return v.Int() == 0
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return v.Uint() == 0
	case reflect.Float32, reflect.Float64:
		return v.Float() == 0
	case reflect.Interface, reflect.Ptr:
		return v.IsNil()
	}
	return false
}

func escapeString(s string) string {
	s = strings.ReplaceAll(s, "\\", "\\\\")
	s = strings.ReplaceAll(s, "\"", "\\\"")
	s = strings.ReplaceAll(s, "\n", "\\n")
	s = strings.ReplaceAll(s, "\r", "\\r")
	s = strings.ReplaceAll(s, "\t", "\\t")
	return s
}
