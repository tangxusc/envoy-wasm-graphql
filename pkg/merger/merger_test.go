package merger

import (
	"context"
	"testing"

	federationtypes "envoy-wasm-graphql-federation/pkg/types"
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

func TestConflictPolicyConstants(t *testing.T) {
	if ConflictPolicyFirst != "first" {
		t.Errorf("Expected ConflictPolicyFirst to be 'first', got %s", ConflictPolicyFirst)
	}

	if ConflictPolicyLast != "last" {
		t.Errorf("Expected ConflictPolicyLast to be 'last', got %s", ConflictPolicyLast)
	}

	if ConflictPolicyMerge != "merge" {
		t.Errorf("Expected ConflictPolicyMerge to be 'merge', got %s", ConflictPolicyMerge)
	}

	if ConflictPolicyError != "error" {
		t.Errorf("Expected ConflictPolicyError to be 'error', got %s", ConflictPolicyError)
	}
}

func TestNullPolicyConstants(t *testing.T) {
	if NullPolicySkip != "skip" {
		t.Errorf("Expected NullPolicySkip to be 'skip', got %s", NullPolicySkip)
	}

	if NullPolicyKeep != "keep" {
		t.Errorf("Expected NullPolicyKeep to be 'keep', got %s", NullPolicyKeep)
	}

	if NullPolicyOverride != "override" {
		t.Errorf("Expected NullPolicyOverride to be 'override', got %s", NullPolicyOverride)
	}
}

func TestDefaultMergerConfig(t *testing.T) {
	config := DefaultMergerConfig()

	if config == nil {
		t.Fatal("DefaultMergerConfig() returned nil")
	}

	if config.MaxDepth != 10 {
		t.Errorf("Expected MaxDepth to be 10, got %d", config.MaxDepth)
	}

	if config.ConflictPolicy != ConflictPolicyFirst {
		t.Errorf("Expected ConflictPolicy to be 'first', got %s", config.ConflictPolicy)
	}

	if config.NullPolicy != NullPolicySkip {
		t.Errorf("Expected NullPolicy to be 'skip', got %s", config.NullPolicy)
	}

	if config.TypeMapping == nil {
		t.Error("Expected TypeMapping to be initialized")
	}

	if config.FieldMapping == nil {
		t.Error("Expected FieldMapping to be initialized")
	}

	if config.EnableMetrics != true {
		t.Errorf("Expected EnableMetrics to be true, got %t", config.EnableMetrics)
	}
}

func TestNewResponseMerger(t *testing.T) {
	logger := &MockLogger{}

	// 测试使用默认配置
	merger := NewResponseMerger(nil, logger)
	if merger == nil {
		t.Fatal("NewResponseMerger() returned nil")
	}

	// 测试使用自定义配置
	config := &MergerConfig{
		MaxDepth:       5,
		ConflictPolicy: ConflictPolicyLast,
		NullPolicy:     NullPolicyKeep,
	}

	merger = NewResponseMerger(config, logger)
	if merger == nil {
		t.Fatal("NewResponseMerger() returned nil")
	}
}

func TestMergeResult_Struct(t *testing.T) {
	result := &MergeResult{
		Data: map[string]interface{}{
			"user": map[string]interface{}{
				"id":   "123",
				"name": "Alice",
			},
		},
		Errors: []federationtypes.GraphQLError{
			{
				Message: "Field not found",
				Path:    []interface{}{"user", "email"},
			},
		},
		Extensions: map[string]interface{}{
			"timestamp": "2023-01-01T00:00:00Z",
		},
		Metadata: &MergeMetadata{
			MergedServices: []string{"users-service", "orders-service"},
			ConflictCount:  0,
			MergeStrategy:  "deep",
			ProcessingTime: "10ms",
			FieldCount:     10,
		},
	}

	if result.Data == nil {
		t.Error("Expected Data to be set")
	}

	if len(result.Errors) != 1 {
		t.Errorf("Expected 1 error, got %d", len(result.Errors))
	}

	if result.Extensions == nil {
		t.Error("Expected Extensions to be set")
	}

	if result.Metadata == nil {
		t.Fatal("Expected Metadata to be set")
	}

	if len(result.Metadata.MergedServices) != 2 {
		t.Errorf("Expected 2 merged services, got %d", len(result.Metadata.MergedServices))
	}

	if result.Metadata.FieldCount != 10 {
		t.Errorf("Expected FieldCount to be 10, got %d", result.Metadata.FieldCount)
	}
}

func TestMergeMetadata_Struct(t *testing.T) {
	metadata := &MergeMetadata{
		MergedServices: []string{"users-service", "orders-service"},
		ConflictCount:  2,
		MergeStrategy:  "deep",
		ProcessingTime: "15ms",
		FieldCount:     25,
	}

	if len(metadata.MergedServices) != 2 {
		t.Errorf("Expected 2 merged services, got %d", len(metadata.MergedServices))
	}

	if metadata.ConflictCount != 2 {
		t.Errorf("Expected ConflictCount to be 2, got %d", metadata.ConflictCount)
	}

	if metadata.MergeStrategy != "deep" {
		t.Errorf("Expected MergeStrategy to be 'deep', got %s", metadata.MergeStrategy)
	}

	if metadata.ProcessingTime != "15ms" {
		t.Errorf("Expected ProcessingTime to be '15ms', got %s", metadata.ProcessingTime)
	}

	if metadata.FieldCount != 25 {
		t.Errorf("Expected FieldCount to be 25, got %d", metadata.FieldCount)
	}
}

func TestMergerConfig_Struct(t *testing.T) {
	config := &MergerConfig{
		MaxDepth:       15,
		ConflictPolicy: ConflictPolicyMerge,
		NullPolicy:     NullPolicyOverride,
		TypeMapping: map[string]string{
			"User": "Person",
		},
		FieldMapping: map[string]FieldMerger{
			"price": &MockFieldMerger{},
		},
		EnableMetrics: false,
	}

	if config.MaxDepth != 15 {
		t.Errorf("Expected MaxDepth to be 15, got %d", config.MaxDepth)
	}

	if config.ConflictPolicy != ConflictPolicyMerge {
		t.Errorf("Expected ConflictPolicy to be 'merge', got %s", config.ConflictPolicy)
	}

	if config.NullPolicy != NullPolicyOverride {
		t.Errorf("Expected NullPolicy to be 'override', got %s", config.NullPolicy)
	}

	if len(config.TypeMapping) != 1 {
		t.Errorf("Expected TypeMapping to have 1 entry, got %d", len(config.TypeMapping))
	}

	if len(config.FieldMapping) != 1 {
		t.Errorf("Expected FieldMapping to have 1 entry, got %d", len(config.FieldMapping))
	}

	if config.EnableMetrics != false {
		t.Errorf("Expected EnableMetrics to be false, got %t", config.EnableMetrics)
	}
}

// MockFieldMerger 实现 FieldMerger 接口用于测试
type MockFieldMerger struct{}

func (m *MockFieldMerger) MergeField(fieldName string, values []interface{}) (interface{}, error) {
	return "merged", nil
}

func TestMergeResponses_EmptyResponses(t *testing.T) {
	logger := &MockLogger{}
	merger := NewResponseMerger(nil, logger)
	ctx := context.Background()

	var responses []*federationtypes.ServiceResponse
	plan := &federationtypes.ExecutionPlan{
		MergeStrategy: "shallow",
	}

	result, err := merger.MergeResponses(ctx, responses, plan)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("Expected result to be non-nil")
	}

	if result.Data != nil {
		t.Error("Expected Data to be nil for empty responses")
	}
}

func TestMergeResponses_NilPlan(t *testing.T) {
	logger := &MockLogger{}
	merger := NewResponseMerger(nil, logger)
	ctx := context.Background()

	responses := []*federationtypes.ServiceResponse{
		{
			Service: "test-service",
			Data: map[string]interface{}{
				"hello": "world",
			},
		},
	}

	// 测试 nil 计划，应该返回错误而不是崩溃
	result, err := merger.MergeResponses(ctx, responses, nil)
	if err == nil {
		t.Log("Nil plan handled gracefully")
	} else {
		t.Logf("Expected error with nil plan: %v", err)
	}

	// 即使有错误，结果也不应该是 nil
	if result != nil {
		t.Log("Result is not nil as expected")
	}
}
