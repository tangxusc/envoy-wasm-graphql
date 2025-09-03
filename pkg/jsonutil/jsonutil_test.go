package jsonutil

import (
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestMarshal_Nil(t *testing.T) {
	data, err := Marshal(nil)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	expected := "null"
	if string(data) != expected {
		t.Errorf("Expected %s, got %s", expected, string(data))
	}
}

func TestMarshal_String(t *testing.T) {
	value := "hello world"
	data, err := Marshal(value)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	expected := `"hello world"`
	if string(data) != expected {
		t.Errorf("Expected %s, got %s", expected, string(data))
	}
}

func TestMarshal_StringWithEscape(t *testing.T) {
	value := "hello \"world\""
	data, err := Marshal(value)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	expected := `"hello \"world\""`
	if string(data) != expected {
		t.Errorf("Expected %s, got %s", expected, string(data))
	}
}

func TestMarshal_Int(t *testing.T) {
	value := 42
	data, err := Marshal(value)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	expected := "42"
	if string(data) != expected {
		t.Errorf("Expected %s, got %s", expected, string(data))
	}
}

func TestMarshal_Int64(t *testing.T) {
	value := int64(9223372036854775807)
	data, err := Marshal(value)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	expected := "9223372036854775807"
	if string(data) != expected {
		t.Errorf("Expected %s, got %s", expected, string(data))
	}
}

func TestMarshal_Uint(t *testing.T) {
	value := uint(42)
	data, err := Marshal(value)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	expected := "42"
	if string(data) != expected {
		t.Errorf("Expected %s, got %s", expected, string(data))
	}
}

func TestMarshal_Float64(t *testing.T) {
	value := 3.14159
	data, err := Marshal(value)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	expected := "3.14159"
	if string(data) != expected {
		t.Errorf("Expected %s, got %s", expected, string(data))
	}
}

func TestMarshal_Bool(t *testing.T) {
	// 测试 true
	value := true
	data, err := Marshal(value)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	expected := "true"
	if string(data) != expected {
		t.Errorf("Expected %s, got %s", expected, string(data))
	}

	// 测试 false
	value = false
	data, err = Marshal(value)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	expected = "false"
	if string(data) != expected {
		t.Errorf("Expected %s, got %s", expected, string(data))
	}
}

func TestMarshal_Slice(t *testing.T) {
	value := []int{1, 2, 3}
	data, err := Marshal(value)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	expected := "[1,2,3]"
	if string(data) != expected {
		t.Errorf("Expected %s, got %s", expected, string(data))
	}
}

func TestMarshal_EmptySlice(t *testing.T) {
	value := []int{}
	data, err := Marshal(value)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	expected := "[]"
	if string(data) != expected {
		t.Errorf("Expected %s, got %s", expected, string(data))
	}
}

func TestMarshal_NilSlice(t *testing.T) {
	var value []int
	data, err := Marshal(value)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	expected := "null"
	if string(data) != expected {
		t.Errorf("Expected %s, got %s", expected, string(data))
	}
}

func TestMarshal_Map(t *testing.T) {
	value := map[string]int{
		"a": 1,
		"b": 2,
	}
	data, err := Marshal(value)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// 由于 map 的迭代顺序不确定，我们需要检查是否包含正确的键值对
	result := string(data)
	if !strings.Contains(result, `"a":1`) || !strings.Contains(result, `"b":2`) {
		t.Errorf("Expected map to contain correct key-value pairs, got %s", result)
	}
}

func TestMarshal_Struct(t *testing.T) {
	type TestStruct struct {
		Name string `json:"name"`
		Age  int    `json:"age"`
	}

	value := TestStruct{
		Name: "Alice",
		Age:  30,
	}

	data, err := Marshal(value)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// 检查是否包含正确的字段
	result := string(data)
	if !strings.Contains(result, `"name":"Alice"`) || !strings.Contains(result, `"age":30`) {
		t.Errorf("Expected struct to contain correct fields, got %s", result)
	}
}

func TestMarshal_StructWithOmitEmpty(t *testing.T) {
	type TestStruct struct {
		Name     string `json:"name"`
		Age      int    `json:"age,omitempty"`
		Nickname string `json:"nickname,omitempty"`
	}

	value := TestStruct{
		Name:     "Alice",
		Age:      30,
		Nickname: "", // 空字符串应该被忽略
	}

	data, err := Marshal(value)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	result := string(data)
	if !strings.Contains(result, `"name":"Alice"`) || !strings.Contains(result, `"age":30`) {
		t.Errorf("Expected struct to contain name and age fields, got %s", result)
	}

	if strings.Contains(result, `"nickname"`) {
		t.Errorf("Expected nickname field to be omitted, got %s", result)
	}
}

func TestMarshal_Duration(t *testing.T) {
	value := 5 * time.Second
	data, err := Marshal(value)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// time.Duration 应该序列化为纳秒数值
	expected := "5000000000" // 5秒 = 5,000,000,000纳秒
	if string(data) != expected {
		t.Errorf("Expected %s, got %s", expected, string(data))
	}
}

func TestMarshal_Pointer(t *testing.T) {
	value := 42
	ptr := &value
	data, err := Marshal(ptr)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	expected := "42"
	if string(data) != expected {
		t.Errorf("Expected %s, got %s", expected, string(data))
	}

	// 测试 nil 指针
	var nilPtr *int
	data, err = Marshal(nilPtr)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	expected = "null"
	if string(data) != expected {
		t.Errorf("Expected %s, got %s", expected, string(data))
	}
}

func TestUnmarshal_NilPointer(t *testing.T) {
	var target *int
	err := Unmarshal([]byte("42"), target)
	if err == nil {
		t.Error("Expected error for nil pointer")
	}
}

func TestUnmarshal_String(t *testing.T) {
	var target string
	err := Unmarshal([]byte(`"hello world"`), &target)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	expected := "hello world"
	if target != expected {
		t.Errorf("Expected %s, got %s", expected, target)
	}
}

func TestUnmarshal_Int(t *testing.T) {
	var target int
	err := Unmarshal([]byte("42"), &target)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	expected := 42
	if target != expected {
		t.Errorf("Expected %d, got %d", expected, target)
	}
}

func TestUnmarshal_Bool(t *testing.T) {
	var target bool
	err := Unmarshal([]byte("true"), &target)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	expected := true
	if target != expected {
		t.Errorf("Expected %t, got %t", expected, target)
	}
}

func TestUnmarshal_Slice(t *testing.T) {
	var target []int
	err := Unmarshal([]byte("[1,2,3]"), &target)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	expected := []int{1, 2, 3}
	if !reflect.DeepEqual(target, expected) {
		t.Errorf("Expected %v, got %v", expected, target)
	}
}

func TestUnmarshal_Map(t *testing.T) {
	var target map[string]int
	err := Unmarshal([]byte(`{"a":1,"b":2}`), &target)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	expected := map[string]int{
		"a": 1,
		"b": 2,
	}

	if !reflect.DeepEqual(target, expected) {
		t.Errorf("Expected %v, got %v", expected, target)
	}
}

func TestUnmarshal_Struct(t *testing.T) {
	type TestStruct struct {
		Name string `json:"name"`
		Age  int    `json:"age"`
	}

	var target TestStruct
	err := Unmarshal([]byte(`{"name":"Alice","age":30}`), &target)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	expected := TestStruct{
		Name: "Alice",
		Age:  30,
	}

	if target != expected {
		t.Errorf("Expected %v, got %v", expected, target)
	}
}

func TestMarshalString(t *testing.T) {
	value := "hello world"
	result, err := MarshalString(value)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	expected := `"hello world"`
	if result != expected {
		t.Errorf("Expected %s, got %s", expected, result)
	}
}

func TestUnmarshalString(t *testing.T) {
	var target string
	err := UnmarshalString(`"hello world"`, &target)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	expected := "hello world"
	if target != expected {
		t.Errorf("Expected %s, got %s", expected, target)
	}
}

func TestIsEmptyValue(t *testing.T) {
	// 测试各种类型的空值
	// 这个函数在 jsonutil.go 中是私有的，但我们可以通过其他方式测试它

	// 测试 int 零值
	var intValue int
	if !isEmptyValue(reflect.ValueOf(intValue)) {
		t.Error("Expected int zero value to be empty")
	}

	// 测试非零 int
	nonZeroInt := 42
	if isEmptyValue(reflect.ValueOf(nonZeroInt)) {
		t.Error("Expected non-zero int to not be empty")
	}

	// 测试 string 零值
	var stringValue string
	if !isEmptyValue(reflect.ValueOf(stringValue)) {
		t.Error("Expected string zero value to be empty")
	}

	// 测试非空 string
	nonEmptyString := "hello"
	if isEmptyValue(reflect.ValueOf(nonEmptyString)) {
		t.Error("Expected non-empty string to not be empty")
	}

	// 测试 bool 零值
	var boolValue bool
	if !isEmptyValue(reflect.ValueOf(boolValue)) {
		t.Error("Expected bool false to be empty")
	}

	// 测试 true bool
	trueValue := true
	if isEmptyValue(reflect.ValueOf(trueValue)) {
		t.Error("Expected bool true to not be empty")
	}

	// 测试 slice 零值
	var sliceValue []int
	if !isEmptyValue(reflect.ValueOf(sliceValue)) {
		t.Error("Expected nil slice to be empty")
	}

	// 测试空 slice
	emptySlice := []int{}
	if !isEmptyValue(reflect.ValueOf(emptySlice)) {
		t.Error("Expected empty slice to be empty")
	}

	// 测试非空 slice
	nonEmptySlice := []int{1}
	if isEmptyValue(reflect.ValueOf(nonEmptySlice)) {
		t.Error("Expected non-empty slice to not be empty")
	}
}
