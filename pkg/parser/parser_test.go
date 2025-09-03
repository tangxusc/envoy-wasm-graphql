package parser

import (
	"testing"

	"envoy-wasm-graphql-federation/pkg/types"
)

// MockLogger 实现 Logger 接口用于测试
type MockLogger struct {
	logs []LogEntry
}

type LogEntry struct {
	Level   string
	Message string
	Fields  []interface{}
}

func (m *MockLogger) Debug(msg string, fields ...interface{}) {
	m.logs = append(m.logs, LogEntry{Level: "DEBUG", Message: msg, Fields: fields})
}

func (m *MockLogger) Info(msg string, fields ...interface{}) {
	m.logs = append(m.logs, LogEntry{Level: "INFO", Message: msg, Fields: fields})
}

func (m *MockLogger) Warn(msg string, fields ...interface{}) {
	m.logs = append(m.logs, LogEntry{Level: "WARN", Message: msg, Fields: fields})
}

func (m *MockLogger) Error(msg string, fields ...interface{}) {
	m.logs = append(m.logs, LogEntry{Level: "ERROR", Message: msg, Fields: fields})
}

func (m *MockLogger) Fatal(msg string, fields ...interface{}) {
	m.logs = append(m.logs, LogEntry{Level: "FATAL", Message: msg, Fields: fields})
}

func TestNewParser(t *testing.T) {
	logger := &MockLogger{}
	parser := NewParser(logger)

	if parser == nil {
		t.Fatal("NewParser() returned nil")
	}

	// 检查是否正确创建了 Parser 实例
	_, ok := parser.(*Parser)
	if !ok {
		t.Error("NewParser() did not return a Parser instance")
	}
}

func TestParseQuery_EmptyQuery(t *testing.T) {
	logger := &MockLogger{}
	parser := NewParser(logger)

	// 测试空查询
	_, err := parser.ParseQuery("")
	if err == nil {
		t.Error("Expected error for empty query")
	}

	// 测试只有空格的查询
	_, err = parser.ParseQuery("   ")
	if err == nil {
		t.Error("Expected error for whitespace-only query")
	}
}

func TestParseQuery_InvalidQuery(t *testing.T) {
	logger := &MockLogger{}
	parser := NewParser(logger)

	// 测试无效的 GraphQL 查询
	invalidQuery := "invalid graphql query {"
	_, err := parser.ParseQuery(invalidQuery)
	if err == nil {
		t.Error("Expected error for invalid query")
	}
}

func TestParseQuery_ValidQuery(t *testing.T) {
	logger := &MockLogger{}
	parser := NewParser(logger)

	// 测试有效的 GraphQL 查询
	validQuery := `
	query GetUser {
		user(id: "123") {
			id
			name
			email
		}
	}`

	parsedQuery, err := parser.ParseQuery(validQuery)
	if err != nil {
		t.Fatalf("Unexpected error for valid query: %v", err)
	}

	if parsedQuery == nil {
		t.Fatal("ParsedQuery should not be nil")
	}

	if parsedQuery.AST == nil {
		t.Error("AST should not be nil")
	}

	if parsedQuery.Variables == nil {
		t.Error("Variables should not be nil")
	}

	if parsedQuery.Fragments == nil {
		t.Error("Fragments should not be nil")
	}
}

func TestValidateQuery_NilParameters(t *testing.T) {
	logger := &MockLogger{}
	parser := NewParser(logger)

	// 测试 nil 查询
	err := parser.ValidateQuery(nil, &types.Schema{})
	if err == nil {
		t.Error("Expected error for nil query")
	}

	// 测试 nil 模式
	err = parser.ValidateQuery(&types.ParsedQuery{}, nil)
	if err == nil {
		t.Error("Expected error for nil schema")
	}
}

func TestExtractFields_NilQuery(t *testing.T) {
	logger := &MockLogger{}
	parser := NewParser(logger)

	// 测试 nil 查询
	_, err := parser.ExtractFields(nil)
	if err == nil {
		t.Error("Expected error for nil query")
	}
}

func TestParser_truncateQuery(t *testing.T) {
	logger := &MockLogger{}
	p := &Parser{logger: logger}

	// 测试短查询
	shortQuery := "query { user { id } }"
	truncated := p.truncateQuery(shortQuery)
	if truncated != shortQuery {
		t.Errorf("Expected %s, got %s", shortQuery, truncated)
	}

	// 测试长查询
	longQuery := "query { " + string(make([]byte, 300)) + " }"
	truncated = p.truncateQuery(longQuery)
	if len(truncated) > 203 { // 200 + 3 个点
		t.Errorf("Expected truncated query to be <= 203 chars, got %d", len(truncated))
	}

	// 测试包含换行符的查询
	multilineQuery := "query {\n  user {\n    id\n  }\n}"
	truncated = p.truncateQuery(multilineQuery)
	if len(truncated) == 0 {
		t.Error("Truncated query should not be empty")
	}
}
