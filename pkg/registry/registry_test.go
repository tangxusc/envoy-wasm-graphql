package registry

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

func TestDefaultRegistryConfig(t *testing.T) {
	config := DefaultRegistryConfig()

	if config == nil {
		t.Fatal("DefaultRegistryConfig() returned nil")
	}

	// 验证默认值
	if config.AutoRefresh != true {
		t.Errorf("Expected AutoRefresh to be true, got %v", config.AutoRefresh)
	}

	if config.RefreshInterval != 5*time.Minute {
		t.Errorf("Expected RefreshInterval to be 5m, got %v", config.RefreshInterval)
	}

	if config.ValidationLevel != ValidationLevelBasic {
		t.Errorf("Expected ValidationLevel to be basic, got %v", config.ValidationLevel)
	}

	if config.CacheEnabled != true {
		t.Errorf("Expected CacheEnabled to be true, got %v", config.CacheEnabled)
	}

	if config.CacheTTL != 10*time.Minute {
		t.Errorf("Expected CacheTTL to be 10m, got %v", config.CacheTTL)
	}

	if config.MaxSchemaSize != 1024*1024 {
		t.Errorf("Expected MaxSchemaSize to be 1MB, got %d", config.MaxSchemaSize)
	}

	if config.EnableIntrospect != true {
		t.Errorf("Expected EnableIntrospect to be true, got %v", config.EnableIntrospect)
	}

	if config.FederationConfig == nil {
		t.Fatal("FederationConfig should not be nil")
	}

	expectedDirectives := []string{"key", "external", "requires", "provides"}
	for i, directive := range expectedDirectives {
		if config.FederationConfig.RequiredDirectives[i] != directive {
			t.Errorf("Expected RequiredDirectives[%d] to be %s, got %s", i, directive, config.FederationConfig.RequiredDirectives[i])
		}
	}

	if config.FederationConfig.TypeExtensions != true {
		t.Errorf("Expected TypeExtensions to be true, got %v", config.FederationConfig.TypeExtensions)
	}
}

func TestNewSchemaRegistry(t *testing.T) {
	logger := &MockLogger{}

	// 测试使用默认配置
	registry := NewSchemaRegistry(nil, logger)
	if registry == nil {
		t.Fatal("NewSchemaRegistry() returned nil")
	}

	// 检查是否正确创建了 SchemaRegistry 实例
	_, ok := registry.(*SchemaRegistry)
	if !ok {
		t.Error("NewSchemaRegistry() did not return a SchemaRegistry instance")
	}

	// 测试使用自定义配置
	customConfig := &RegistryConfig{
		AutoRefresh:   false,
		MaxSchemaSize: 512 * 1024, // 512KB
	}

	registry2 := NewSchemaRegistry(customConfig, logger)
	schemaRegistry, ok := registry2.(*SchemaRegistry)
	if !ok {
		t.Fatal("NewSchemaRegistry() did not return a SchemaRegistry instance")
	}

	if schemaRegistry.config.AutoRefresh != false {
		t.Error("Custom config AutoRefresh was not used")
	}

	if schemaRegistry.config.MaxSchemaSize != 512*1024 {
		t.Error("Custom config MaxSchemaSize was not used")
	}
}

func TestSchemaRegistry_RegisterSchema_InvalidParameters(t *testing.T) {
	logger := &MockLogger{}
	registry := NewSchemaRegistry(nil, logger)

	// 测试空服务名
	err := registry.RegisterSchema("", "type Query { hello: String }")
	if err == nil {
		t.Error("Expected error for empty service name")
	}

	// 测试空模式
	err = registry.RegisterSchema("test-service", "")
	if err == nil {
		t.Error("Expected error for empty schema")
	}

	// 测试只有空格的模式
	err = registry.RegisterSchema("test-service", "   ")
	if err == nil {
		t.Error("Expected error for whitespace-only schema")
	}
}

func TestValidationLevelConstants(t *testing.T) {
	if ValidationLevelNone != "none" {
		t.Errorf("Expected ValidationLevelNone to be 'none', got %s", ValidationLevelNone)
	}

	if ValidationLevelBasic != "basic" {
		t.Errorf("Expected ValidationLevelBasic to be 'basic', got %s", ValidationLevelBasic)
	}

	if ValidationLevelStrict != "strict" {
		t.Errorf("Expected ValidationLevelStrict to be 'strict', got %s", ValidationLevelStrict)
	}

	// 测试别名
	if ValidationStrict != "strict" {
		t.Errorf("Expected ValidationStrict to be 'strict', got %s", ValidationStrict)
	}
}

func TestSchemaInfo_Struct(t *testing.T) {
	info := &SchemaInfo{
		ServiceName:   "test-service",
		SDL:           "type Query { hello: String }",
		Version:       "v1.0.0",
		Types:         make(map[string]*TypeInfo),
		Queries:       make(map[string]*FieldInfo),
		Mutations:     make(map[string]*FieldInfo),
		Subscriptions: make(map[string]*FieldInfo),
		Directives:    make(map[string]*DirectiveInfo),
		Metadata:      make(map[string]interface{}),
	}

	if info.ServiceName != "test-service" {
		t.Errorf("Expected ServiceName to be 'test-service', got %s", info.ServiceName)
	}

	if info.SDL != "type Query { hello: String }" {
		t.Errorf("Expected SDL to match, got %s", info.SDL)
	}
}

func TestTypeInfo_Struct(t *testing.T) {
	info := &TypeInfo{
		Name:        "User",
		Kind:        "OBJECT",
		Fields:      make(map[string]*FieldInfo),
		Interfaces:  []string{"Node"},
		UnionTypes:  []string{},
		EnumValues:  []string{},
		Description: "A user type",
		Directives:  make(map[string]*DirectiveInfo),
	}

	if info.Name != "User" {
		t.Errorf("Expected Name to be 'User', got %s", info.Name)
	}

	if info.Kind != "OBJECT" {
		t.Errorf("Expected Kind to be 'OBJECT', got %s", info.Kind)
	}

	if len(info.Interfaces) != 1 || info.Interfaces[0] != "Node" {
		t.Errorf("Expected Interfaces to contain 'Node', got %v", info.Interfaces)
	}
}

func TestFieldInfo_Struct(t *testing.T) {
	info := &FieldInfo{
		Name:        "id",
		Type:        "ID!",
		Arguments:   make(map[string]*ArgumentInfo),
		Description: "The unique identifier",
		Directives:  make(map[string]*DirectiveInfo),
		IsResolver:  true,
	}

	if info.Name != "id" {
		t.Errorf("Expected Name to be 'id', got %s", info.Name)
	}

	if info.Type != "ID!" {
		t.Errorf("Expected Type to be 'ID!', got %s", info.Type)
	}

	if !info.IsResolver {
		t.Error("Expected IsResolver to be true")
	}
}

func TestArgumentInfo_Struct(t *testing.T) {
	info := &ArgumentInfo{
		Name:         "first",
		Type:         "Int",
		DefaultValue: 10,
		Description:  "Number of items to return",
	}

	if info.Name != "first" {
		t.Errorf("Expected Name to be 'first', got %s", info.Name)
	}

	if info.Type != "Int" {
		t.Errorf("Expected Type to be 'Int', got %s", info.Type)
	}

	if info.DefaultValue != 10 {
		t.Errorf("Expected DefaultValue to be 10, got %v", info.DefaultValue)
	}
}

func TestDirectiveInfo_Struct(t *testing.T) {
	info := &DirectiveInfo{
		Name:        "deprecated",
		Description: "Marks an element as deprecated",
		Arguments:   map[string]interface{}{"reason": "Use newer field instead"},
		Locations:   []string{"FIELD_DEFINITION"},
		Repeatable:  false,
	}

	if info.Name != "deprecated" {
		t.Errorf("Expected Name to be 'deprecated', got %s", info.Name)
	}

	if info.Description != "Marks an element as deprecated" {
		t.Errorf("Expected Description to match, got %s", info.Description)
	}

	if info.Repeatable != false {
		t.Error("Expected Repeatable to be false")
	}
}

func TestRegistryMetrics_Struct(t *testing.T) {
	metrics := &RegistryMetrics{
		SchemaCount:       5,
		LastRefreshTime:   time.Now(),
		RefreshCount:      10,
		ValidationErrors:  2,
		FederationBuilds:  3,
		AverageSchemaSize: 1024,
		RefreshDuration:   time.Second,
	}

	if metrics.SchemaCount != 5 {
		t.Errorf("Expected SchemaCount to be 5, got %d", metrics.SchemaCount)
	}

	if metrics.RefreshCount != 10 {
		t.Errorf("Expected RefreshCount to be 10, got %d", metrics.RefreshCount)
	}

	if metrics.ValidationErrors != 2 {
		t.Errorf("Expected ValidationErrors to be 2, got %d", metrics.ValidationErrors)
	}
}
