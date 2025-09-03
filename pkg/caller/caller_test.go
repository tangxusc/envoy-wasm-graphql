package caller

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

func TestDefaultCallerConfig(t *testing.T) {
	config := DefaultCallerConfig()

	if config == nil {
		t.Fatal("DefaultCallerConfig() returned nil")
	}

	// 验证默认值
	if config.DefaultTimeout != 10*time.Second {
		t.Errorf("Expected DefaultTimeout to be 10s, got %v", config.DefaultTimeout)
	}

	if config.MaxRetries != 3 {
		t.Errorf("Expected MaxRetries to be 3, got %d", config.MaxRetries)
	}

	if config.HealthCheckCache != 30*time.Second {
		t.Errorf("Expected HealthCheckCache to be 30s, got %v", config.HealthCheckCache)
	}

	if config.ConnectTimeout != 5*time.Second {
		t.Errorf("Expected ConnectTimeout to be 5s, got %v", config.ConnectTimeout)
	}

	if config.ReadTimeout != 10*time.Second {
		t.Errorf("Expected ReadTimeout to be 10s, got %v", config.ReadTimeout)
	}

	if config.WriteTimeout != 10*time.Second {
		t.Errorf("Expected WriteTimeout to be 10s, got %v", config.WriteTimeout)
	}

	if config.MaxIdleConns != 100 {
		t.Errorf("Expected MaxIdleConns to be 100, got %d", config.MaxIdleConns)
	}

	if config.MaxConnsPerHost != 10 {
		t.Errorf("Expected MaxConnsPerHost to be 10, got %d", config.MaxConnsPerHost)
	}

	if config.IdleConnTimeout != 90*time.Second {
		t.Errorf("Expected IdleConnTimeout to be 90s, got %v", config.IdleConnTimeout)
	}
}

func TestNewHTTPCaller(t *testing.T) {
	logger := &MockLogger{}

	// 测试使用默认配置
	caller := NewHTTPCaller(nil, logger)
	if caller == nil {
		t.Fatal("NewHTTPCaller() returned nil")
	}

	wasmCaller, ok := caller.(*WASMCaller)
	if !ok {
		t.Fatal("NewHTTPCaller() did not return a WASMCaller")
	}

	if wasmCaller.logger != logger {
		t.Error("Logger was not set correctly")
	}

	if wasmCaller.metrics == nil {
		t.Error("Metrics was not initialized")
	}

	if wasmCaller.config == nil {
		t.Error("Config was not set")
	}

	// 测试使用自定义配置
	customConfig := &CallerConfig{
		DefaultTimeout: 5 * time.Second,
		MaxRetries:     1,
	}

	caller2 := NewHTTPCaller(customConfig, logger)
	wasmCaller2, ok := caller2.(*WASMCaller)
	if !ok {
		t.Fatal("NewHTTPCaller() did not return a WASMCaller")
	}

	if wasmCaller2.config.DefaultTimeout != 5*time.Second {
		t.Error("Custom config was not used")
	}
}

func TestWASMCaller_Call_WithNilParameters(t *testing.T) {
	logger := &MockLogger{}
	caller := NewHTTPCaller(nil, logger).(*WASMCaller)
	ctx := context.Background()

	// 测试 nil call
	_, err := caller.Call(ctx, nil)
	if err == nil {
		t.Error("Expected error for nil call")
	}

	// 测试 nil service
	call := &types.ServiceCall{}
	_, err = caller.Call(ctx, call)
	if err == nil {
		t.Error("Expected error for nil service")
	}
}

func TestWASMCaller_CallBatch_EmptySlice(t *testing.T) {
	logger := &MockLogger{}
	caller := NewHTTPCaller(nil, logger).(*WASMCaller)
	ctx := context.Background()

	// 测试空切片
	responses, err := caller.CallBatch(ctx, []*types.ServiceCall{})
	if err != nil {
		t.Errorf("Unexpected error for empty calls: %v", err)
	}

	if responses != nil {
		t.Error("Expected nil responses for empty calls")
	}
}

func TestWASMCaller_extractClusterName(t *testing.T) {
	logger := &MockLogger{}
	caller := NewHTTPCaller(nil, logger).(*WASMCaller)

	testCases := []struct {
		endpoint string
		expected string
	}{
		{"http://localhost:8080", "localhost"},
		{"https://api.example.com", "api.example.com"},
		{"http://service:3000/graphql", "service"},
		{"localhost:8080", "localhost"},
		{"api.service.local", "api.service.local"},
	}

	for _, tc := range testCases {
		result := caller.extractClusterName(tc.endpoint)
		if result != tc.expected {
			t.Errorf("extractClusterName(%s) = %s, expected %s", tc.endpoint, result, tc.expected)
		}
	}
}

func TestWASMCaller_updateMetrics(t *testing.T) {
	logger := &MockLogger{}
	caller := NewHTTPCaller(nil, logger).(*WASMCaller)

	initialFailed := caller.metrics.FailedCalls

	caller.recordFailure()

	if caller.metrics.FailedCalls != initialFailed+1 {
		t.Error("recordFailure() did not increment FailedCalls")
	}
}

func TestWASMCaller_updateLatency(t *testing.T) {
	logger := &MockLogger{}
	caller := NewHTTPCaller(nil, logger).(*WASMCaller)

	latency := 100 * time.Millisecond
	caller.updateLatency(latency)

	if caller.metrics.AvgLatency == 0 {
		t.Error("updateLatency() did not update AvgLatency")
	}
}

func TestWASMCaller_GetMetrics(t *testing.T) {
	logger := &MockLogger{}
	caller := NewHTTPCaller(nil, logger).(*WASMCaller)

	// Simulate some metrics
	caller.recordFailure()
	caller.updateLatency(100 * time.Millisecond)

	metrics := caller.GetMetrics()
	if metrics == nil {
		t.Fatal("GetMetrics() returned nil")
	}

	if metrics.SuccessfulCalls != 0 {
		t.Errorf("Expected SuccessfulCalls to be 0, got %d", metrics.SuccessfulCalls)
	}

	if metrics.FailedCalls != 1 {
		t.Errorf("Expected FailedCalls to be 1, got %d", metrics.FailedCalls)
	}
}

func TestHealthStatus(t *testing.T) {
	status := &HealthStatus{
		Healthy:    true,
		LastCheck:  time.Now(),
		Latency:    50 * time.Millisecond,
		CheckCount: 10,
		FailCount:  1,
	}

	if !status.Healthy {
		t.Error("Expected status to be healthy")
	}

	if status.Latency != 50*time.Millisecond {
		t.Errorf("Expected latency to be 50ms, got %v", status.Latency)
	}

	if status.CheckCount != 10 {
		t.Errorf("Expected CheckCount to be 10, got %d", status.CheckCount)
	}

	if status.FailCount != 1 {
		t.Errorf("Expected FailCount to be 1, got %d", status.FailCount)
	}
}

func TestWASMCaller_ClearHealthCache(t *testing.T) {
	logger := &MockLogger{}
	caller := NewHTTPCaller(nil, logger).(*WASMCaller)

	// 添加一些健康状态到缓存
	caller.healthCache.Store("service1", &HealthStatus{Healthy: true})
	caller.healthCache.Store("service2", &HealthStatus{Healthy: false})

	// 清除缓存
	caller.ClearHealthCache()

	// 验证缓存已清空
	var count int
	caller.healthCache.Range(func(key, value interface{}) bool {
		count++
		return true
	})

	if count != 0 {
		t.Errorf("Expected health cache to be empty, but found %d entries", count)
	}
}
