package config

import (
	"testing"
	"time"
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

func TestNewManager(t *testing.T) {
	logger := &MockLogger{}
	manager := NewManager(logger)

	if manager == nil {
		t.Fatal("NewManager() returned nil")
	}
}

func TestNewManagerWithOptions(t *testing.T) {
	logger := &MockLogger{}
	options := ManagerOptions{
		ValidationLevel: ValidationLevelStrict,
	}

	manager := NewManagerWithOptions(logger, options)

	if manager == nil {
		t.Fatal("NewManagerWithOptions() returned nil")
	}

	// 验证选项是否正确设置
	// 注意：由于字段是私有的，我们无法直接访问验证级别
	// 但我们可以通过调用方法来间接验证
}

func TestLoadConfig_EmptyData(t *testing.T) {
	logger := &MockLogger{}
	manager := NewManager(logger)

	_, err := manager.LoadConfig([]byte{})
	if err == nil {
		t.Error("Expected error for empty configuration data")
	}
}

func TestLoadConfig_InvalidJSON(t *testing.T) {
	logger := &MockLogger{}
	manager := NewManager(logger)

	invalidJSON := []byte("{ invalid json }")
	_, err := manager.LoadConfig(invalidJSON)
	if err == nil {
		t.Error("Expected error for invalid JSON")
	}
}

func TestLoadConfig_ValidConfig(t *testing.T) {
	logger := &MockLogger{}
	manager := NewManager(logger)

	validConfig := []byte(`{
		"services": [
			{
				"name": "users-service",
				"endpoint": "http://localhost:8080/graphql",
				"schema": "type Query { user(id: ID!): User } type User { id: ID! name: String }"
			}
		],
		"enableQueryPlanning": true,
		"enableCaching": true,
		"maxQueryDepth": 10,
		"queryTimeout": 30000000000,
		"enableIntrospection": true,
		"debugMode": false
	}`)

	_, err := manager.LoadConfig(validConfig)
	if err != nil {
		t.Fatalf("Unexpected error for valid config: %v", err)
	}
}

func TestValidationLevelConstants(t *testing.T) {
	if ValidationLevelBasic != 0 {
		t.Errorf("Expected ValidationLevelBasic to be 0, got %d", ValidationLevelBasic)
	}

	if ValidationLevelStrict != 1 {
		t.Errorf("Expected ValidationLevelStrict to be 1, got %d", ValidationLevelStrict)
	}

	if ValidationLevelExtended != 2 {
		t.Errorf("Expected ValidationLevelExtended to be 2, got %d", ValidationLevelExtended)
	}
}

func TestChangeTypeConstants(t *testing.T) {
	if ChangeTypeAdded != "added" {
		t.Errorf("Expected ChangeTypeAdded to be 'added', got %s", ChangeTypeAdded)
	}

	if ChangeTypeModified != "modified" {
		t.Errorf("Expected ChangeTypeModified to be 'modified', got %s", ChangeTypeModified)
	}

	if ChangeTypeRemoved != "removed" {
		t.Errorf("Expected ChangeTypeRemoved to be 'removed', got %s", ChangeTypeRemoved)
	}
}

func TestErrorSeverityConstants(t *testing.T) {
	if SeverityError != "error" {
		t.Errorf("Expected SeverityError to be 'error', got %s", SeverityError)
	}

	if SeverityWarning != "warning" {
		t.Errorf("Expected SeverityWarning to be 'warning', got %s", SeverityWarning)
	}

	if SeverityInfo != "info" {
		t.Errorf("Expected SeverityInfo to be 'info', got %s", SeverityInfo)
	}
}

func TestConfigChange_Struct(t *testing.T) {
	change := &ConfigChange{
		Type:        ChangeTypeModified,
		Path:        "services[0].endpoint",
		OldValue:    "http://old-endpoint:8080/graphql",
		NewValue:    "http://new-endpoint:8080/graphql",
		Description: "Service endpoint changed",
		Metadata: map[string]interface{}{
			"service": "users-service",
		},
	}

	if change.Type != ChangeTypeModified {
		t.Errorf("Expected Type to be 'modified', got %s", change.Type)
	}

	if change.Path != "services[0].endpoint" {
		t.Errorf("Expected Path to be 'services[0].endpoint', got %s", change.Path)
	}

	if change.Description != "Service endpoint changed" {
		t.Errorf("Expected Description to match, got %s", change.Description)
	}
}

func TestValidationError_Struct(t *testing.T) {
	error := &ValidationError{
		Path:       "services[0].endpoint",
		Message:    "Invalid endpoint URL",
		Severity:   SeverityError,
		Code:       "INVALID_ENDPOINT",
		Suggestion: "Use a valid HTTP/HTTPS URL",
		Metadata: map[string]interface{}{
			"service": "users-service",
		},
	}

	if error.Path != "services[0].endpoint" {
		t.Errorf("Expected Path to be 'services[0].endpoint', got %s", error.Path)
	}

	if error.Message != "Invalid endpoint URL" {
		t.Errorf("Expected Message to match, got %s", error.Message)
	}

	if error.Severity != SeverityError {
		t.Errorf("Expected Severity to be 'error', got %s", error.Severity)
	}
}

func TestConfigMetrics_Struct(t *testing.T) {
	now := time.Now()
	metrics := &ConfigMetrics{
		ReloadCount:       5,
		ValidationCount:   10,
		ValidationErrors:  2,
		LastReloadTime:    now,
		LastValidation:    now.Add(-time.Minute),
		AverageReloadTime: time.Second,
		ConfigVersion:     "1.0.0",
		ServiceCount:      3,
		ServiceHealth: map[string]bool{
			"users-service":  true,
			"orders-service": false,
		},
	}

	if metrics.ReloadCount != 5 {
		t.Errorf("Expected ReloadCount to be 5, got %d", metrics.ReloadCount)
	}

	if metrics.ValidationErrors != 2 {
		t.Errorf("Expected ValidationErrors to be 2, got %d", metrics.ValidationErrors)
	}

	if metrics.ServiceCount != 3 {
		t.Errorf("Expected ServiceCount to be 3, got %d", metrics.ServiceCount)
	}

	if len(metrics.ServiceHealth) != 2 {
		t.Errorf("Expected ServiceHealth to have 2 entries, got %d", len(metrics.ServiceHealth))
	}
}
