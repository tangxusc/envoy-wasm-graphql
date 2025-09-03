package planner

import (
	"context"
	"testing"
	"time"

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

func TestNewPlanner(t *testing.T) {
	logger := &MockLogger{}

	planner := NewPlanner(logger)
	if planner == nil {
		t.Fatal("NewPlanner() returned nil")
	}

	// 检查是否正确创建了 Planner 实例
	_, ok := planner.(*Planner)
	if !ok {
		t.Error("NewPlanner() did not return a Planner instance")
	}
}

func TestPlanner_CreateExecutionPlan_NilParameters(t *testing.T) {
	logger := &MockLogger{}
	planner := NewPlanner(logger)
	ctx := context.Background()

	// 测试 nil 查询
	_, err := planner.CreateExecutionPlan(ctx, nil, []types.ServiceConfig{})
	if err == nil {
		t.Error("Expected error for nil query")
	}

	// 测试空服务列表
	query := &types.ParsedQuery{
		Operation: "testOperation",
	}
	_, err = planner.CreateExecutionPlan(ctx, query, []types.ServiceConfig{})
	if err == nil {
		t.Error("Expected error for empty services")
	}
}

func TestPlanner_OptimizePlan_NilPlan(t *testing.T) {
	logger := &MockLogger{}
	planner := NewPlanner(logger)

	// 测试 nil 计划
	_, err := planner.OptimizePlan(nil)
	if err == nil {
		t.Error("Expected error for nil plan")
	}
}

func TestPlanner_ValidatePlan_NilPlan(t *testing.T) {
	logger := &MockLogger{}
	planner := NewPlanner(logger)

	// 测试 nil 计划
	err := planner.ValidatePlan(nil)
	if err == nil {
		t.Error("Expected error for nil plan")
	}
}

func TestPlanner_ValidatePlan_EmptySubQueries(t *testing.T) {
	logger := &MockLogger{}
	planner := NewPlanner(logger)

	// 测试空子查询
	plan := &types.ExecutionPlan{
		SubQueries: []types.SubQuery{},
	}

	err := planner.ValidatePlan(plan)
	if err == nil {
		t.Error("Expected error for empty sub-queries")
	}
}

func TestExecutionPlan_Struct(t *testing.T) {
	now := time.Now()
	plan := &types.ExecutionPlan{
		SubQueries: []types.SubQuery{
			{
				ServiceName:   "users-service",
				Query:         "{ user(id: \"123\") { id name } }",
				Variables:     map[string]interface{}{"id": "123"},
				OperationName: "GetUser",
				Timeout:       time.Second * 30,
			},
		},
		Dependencies: map[string][]string{
			"orders-service": {"users-service"},
		},
		MergeStrategy: "deep",
		Metadata: map[string]interface{}{
			"createdAt": now,
			"version":   "1.0.0",
		},
	}

	if len(plan.SubQueries) != 1 {
		t.Errorf("Expected 1 sub-query, got %d", len(plan.SubQueries))
	}

	if plan.SubQueries[0].ServiceName != "users-service" {
		t.Errorf("Expected service to be 'users-service', got %s", plan.SubQueries[0].ServiceName)
	}

	if len(plan.Dependencies) != 1 {
		t.Errorf("Expected 1 dependency, got %d", len(plan.Dependencies))
	}

	if plan.MergeStrategy != "deep" {
		t.Errorf("Expected merge strategy to be 'deep', got %s", plan.MergeStrategy)
	}

	if plan.Metadata["version"] != "1.0.0" {
		t.Errorf("Expected version to be '1.0.0', got %v", plan.Metadata["version"])
	}
}

func TestSubQuery_Struct(t *testing.T) {
	subQuery := &types.SubQuery{
		ServiceName:   "users-service",
		Query:         "{ user(id: \"123\") { id name } }",
		Variables:     map[string]interface{}{"id": "123"},
		OperationName: "GetUser",
		Timeout:       time.Second * 30,
		Path:          []string{"user", "profile"},
	}

	if subQuery.ServiceName != "users-service" {
		t.Errorf("Expected service to be 'users-service', got %s", subQuery.ServiceName)
	}

	if subQuery.OperationName != "GetUser" {
		t.Errorf("Expected operation name to be 'GetUser', got %s", subQuery.OperationName)
	}

	if subQuery.Timeout != time.Second*30 {
		t.Errorf("Expected timeout to be 30s, got %v", subQuery.Timeout)
	}

	if len(subQuery.Path) != 2 {
		t.Errorf("Expected path length to be 2, got %d", len(subQuery.Path))
	}
}
