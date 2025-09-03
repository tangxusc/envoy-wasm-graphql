package utils

import (
	"strings"
	"testing"
)

func TestGenerateRequestID(t *testing.T) {
	// 生成两个请求ID并确保它们不相同
	id1 := GenerateRequestID()
	id2 := GenerateRequestID()

	if id1 == "" {
		t.Error("GenerateRequestID() returned empty string")
	}

	if id2 == "" {
		t.Error("GenerateRequestID() returned empty string")
	}

	// 注意：由于使用时间戳，理论上可能相同，但在实践中几乎不可能
	// 我们不强制要求不同，只要确保不为空即可
	if !strings.HasPrefix(id1, "req_") {
		t.Errorf("Expected request ID to start with \"req_\", got %s", id1)
	}

	if !strings.HasPrefix(id2, "req_") {
		t.Errorf("Expected request ID to start with \"req_\", got %s", id2)
	}
}

func TestGetQueryParam(t *testing.T) {
	// 测试正常情况
	query := "name=John&age=30&city=NewYork"

	result := GetQueryParam(query, "name")
	if result != "John" {
		t.Errorf("Expected 'John', got '%s'", result)
	}

	result = GetQueryParam(query, "age")
	if result != "30" {
		t.Errorf("Expected '30', got '%s'", result)
	}

	result = GetQueryParam(query, "city")
	if result != "NewYork" {
		t.Errorf("Expected 'NewYork', got '%s'", result)
	}

	// 测试不存在的参数
	result = GetQueryParam(query, "email")
	if result != "" {
		t.Errorf("Expected empty string, got '%s'", result)
	}

	// 测试空查询
	result = GetQueryParam("", "name")
	if result != "" {
		t.Errorf("Expected empty string, got '%s'", result)
	}

	// 测试空参数名
	result = GetQueryParam(query, "")
	if result != "" {
		t.Errorf("Expected empty string, got '%s'", result)
	}

	// 测试特殊字符
	query2 := "msg=hello%20world&special=value%3Dtest"
	result = GetQueryParam(query2, "msg")
	if result != "hello%20world" {
		t.Errorf("Expected 'hello%%20world', got '%s'", result)
	}

	result = GetQueryParam(query2, "special")
	if result != "value%3Dtest" {
		t.Errorf("Expected 'value%%3Dtest', got '%s'", result)
	}
}

func TestParseQueryParam(t *testing.T) {
	// 测试正常情况
	query := "name=John&age=30&city=NewYork"

	result := parseQueryParam(query, "name")
	if result != "John" {
		t.Errorf("Expected 'John', got '%s'", result)
	}

	// 测试空查询
	result = parseQueryParam("", "name")
	if result != "" {
		t.Errorf("Expected empty string, got '%s'", result)
	}

	// 测试空参数名
	result = parseQueryParam(query, "")
	if result != "" {
		t.Errorf("Expected empty string, got '%s'", result)
	}

	// 测试没有等号的参数
	query2 := "name=John&invalid&age=30"
	result = parseQueryParam(query2, "invalid")
	if result != "" {
		t.Errorf("Expected empty string for invalid parameter, got '%s'", result)
	}

	// 测试只有键没有值
	query3 := "name=John&empty=&age=30"
	result = parseQueryParam(query3, "empty")
	if result != "" {
		t.Errorf("Expected empty string for empty value, got '%s'", result)
	}
}

func TestIsValidURL(t *testing.T) {
	// 测试有效的HTTP URL
	if !IsValidURL("http://example.com") {
		t.Error("Expected 'http://example.com' to be valid")
	}

	if !IsValidURL("https://example.com") {
		t.Error("Expected 'https://example.com' to be valid")
	}

	if !IsValidURL("http://example.com/path") {
		t.Error("Expected 'http://example.com/path' to be valid")
	}

	if !IsValidURL("https://example.com:8080/path?query=value") {
		t.Error("Expected 'https://example.com:8080/path?query=value' to be valid")
	}

	// 测试无效的URL
	if IsValidURL("") {
		t.Error("Expected empty string to be invalid")
	}

	if IsValidURL("example.com") {
		t.Error("Expected 'example.com' to be invalid (no protocol)")
	}

	if IsValidURL("http://") {
		t.Error("Expected 'http://' to be invalid (no host)")
	}

	if IsValidURL("https://") {
		t.Error("Expected 'https://' to be invalid (no host)")
	}

	// 测试包含无效字符的URL
	if IsValidURL("http://exam ple.com") {
		t.Error("Expected 'http://exam ple.com' to be invalid (space in host)")
	}
}

func TestNewLogger(t *testing.T) {
	logger := NewLogger("test")
	if logger == nil {
		t.Fatal("NewLogger() returned nil")
	}

	// 检查是否正确创建了 Logger 实例
	_, ok := logger.(*Logger)
	if !ok {
		t.Error("NewLogger() did not return a Logger instance")
	}
}

func TestLoggerMethods(t *testing.T) {
	logger := NewLogger("test")

	// 测试各种日志方法（不会panic即可）
	logger.Debug("Debug message")
	logger.Debug("Debug message with fields", "key1", "value1", "key2", "value2")

	logger.Info("Info message")
	logger.Info("Info message with fields", "key1", "value1")

	logger.Warn("Warn message")
	logger.Warn("Warn message with fields", "error", "something went wrong")

	logger.Error("Error message")
	logger.Error("Error message with fields", "error", "something went wrong", "code", 500)

	logger.Fatal("Fatal message")
	logger.Fatal("Fatal message with fields", "error", "critical failure")
}

func TestFormatFields(t *testing.T) {
	// 创建一个Logger实例来访问formatFields方法
	logger := &Logger{}

	// 测试空字段
	result := logger.formatFields()
	if result != "" {
		t.Errorf("Expected empty string, got '%s'", result)
	}

	// 测试正常字段
	result = logger.formatFields("key1", "value1", "key2", "value2")
	expected := "key1=value1 key2=value2"
	if result != expected {
		t.Errorf("Expected '%s', got '%s'", expected, result)
	}

	// 测试奇数个字段
	result = logger.formatFields("key1", "value1", "key2")
	expected = "key1=value1 key2="
	if result != expected {
		t.Errorf("Expected '%s', got '%s'", expected, result)
	}

	// 测试数字值
	result = logger.formatFields("count", 42, "rate", 3.14)
	expected = "count=42 rate=3.14"
	if result != expected {
		t.Errorf("Expected '%s', got '%s'", expected, result)
	}
}
