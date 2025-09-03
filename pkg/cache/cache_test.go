package cache

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

func TestDefaultCacheConfig(t *testing.T) {
	config := DefaultCacheConfig()

	if config == nil {
		t.Fatal("DefaultCacheConfig() returned nil")
	}

	// 验证默认值
	if config.Enabled != true {
		t.Errorf("Expected Enabled to be true, got %v", config.Enabled)
	}

	if config.DefaultTTL != 5*time.Minute {
		t.Errorf("Expected DefaultTTL to be 5m, got %v", config.DefaultTTL)
	}

	if config.MaxSize != 1000 {
		t.Errorf("Expected MaxSize to be 1000, got %d", config.MaxSize)
	}

	if config.CleanupInterval != 1*time.Minute {
		t.Errorf("Expected CleanupInterval to be 1m, got %v", config.CleanupInterval)
	}

	// 验证查询缓存配置
	if config.QueryCache.Enabled != true {
		t.Errorf("Expected QueryCache.Enabled to be true, got %v", config.QueryCache.Enabled)
	}

	if config.QueryCache.TTL != 2*time.Minute {
		t.Errorf("Expected QueryCache.TTL to be 2m, got %v", config.QueryCache.TTL)
	}

	if config.QueryCache.MaxSize != 500 {
		t.Errorf("Expected QueryCache.MaxSize to be 500, got %d", config.QueryCache.MaxSize)
	}

	if config.QueryCache.MaxKeySize != 1024 {
		t.Errorf("Expected QueryCache.MaxKeySize to be 1024, got %d", config.QueryCache.MaxKeySize)
	}

	// 验证模式缓存配置
	if config.SchemaCache.Enabled != true {
		t.Errorf("Expected SchemaCache.Enabled to be true, got %v", config.SchemaCache.Enabled)
	}

	if config.SchemaCache.TTL != 10*time.Minute {
		t.Errorf("Expected SchemaCache.TTL to be 10m, got %v", config.SchemaCache.TTL)
	}

	if config.SchemaCache.MaxSize != 100 {
		t.Errorf("Expected SchemaCache.MaxSize to be 100, got %d", config.SchemaCache.MaxSize)
	}

	// 验证计划缓存配置
	if config.PlanCache.Enabled != true {
		t.Errorf("Expected PlanCache.Enabled to be true, got %v", config.PlanCache.Enabled)
	}

	if config.PlanCache.TTL != 5*time.Minute {
		t.Errorf("Expected PlanCache.TTL to be 5m, got %v", config.PlanCache.TTL)
	}

	if config.PlanCache.MaxSize != 200 {
		t.Errorf("Expected PlanCache.MaxSize to be 200, got %d", config.PlanCache.MaxSize)
	}

	if config.EnableMetrics != true {
		t.Errorf("Expected EnableMetrics to be true, got %v", config.EnableMetrics)
	}

	if config.EnableCompression != false {
		t.Errorf("Expected EnableCompression to be false, got %v", config.EnableCompression)
	}
}

func TestNewMemoryCache(t *testing.T) {
	logger := &MockLogger{}

	// 测试使用默认配置
	cache := NewMemoryCache(nil, logger)
	if cache == nil {
		t.Fatal("NewMemoryCache() returned nil")
	}

	// 检查是否正确创建了 MemoryCache 实例
	memCache, ok := cache.(*MemoryCache)
	if !ok {
		t.Error("NewMemoryCache() did not return a MemoryCache instance")
	}

	if memCache.logger != logger {
		t.Error("Logger was not set correctly")
	}

	if memCache.config == nil {
		t.Error("Config was not initialized")
	}

	// 测试使用自定义配置
	customConfig := &CacheConfig{
		Enabled: false,
		MaxSize: 2000,
	}

	cache2 := NewMemoryCache(customConfig, logger)
	memCache2, ok := cache2.(*MemoryCache)
	if !ok {
		t.Fatal("NewMemoryCache() did not return a MemoryCache instance")
	}

	if memCache2.config.Enabled != false {
		t.Error("Custom config Enabled was not used")
	}

	if memCache2.config.MaxSize != 2000 {
		t.Error("Custom config MaxSize was not used")
	}
}

func TestCacheConfig_Structs(t *testing.T) {
	// 测试 CacheConfig 结构
	config := &CacheConfig{
		Enabled:         true,
		DefaultTTL:      time.Minute,
		MaxSize:         100,
		CleanupInterval: time.Second,
		QueryCache: QueryCacheConfig{
			Enabled:    true,
			TTL:        time.Minute,
			MaxSize:    50,
			MaxKeySize: 512,
		},
		SchemaCache: SchemaCacheConfig{
			Enabled: true,
			TTL:     time.Minute * 2,
			MaxSize: 25,
		},
		PlanCache: PlanCacheConfig{
			Enabled: true,
			TTL:     time.Minute * 3,
			MaxSize: 25,
		},
		EnableMetrics:     true,
		EnableCompression: false,
	}

	if !config.Enabled {
		t.Error("Expected Enabled to be true")
	}

	if config.DefaultTTL != time.Minute {
		t.Errorf("Expected DefaultTTL to be 1m, got %v", config.DefaultTTL)
	}

	// 测试 QueryCacheConfig 结构
	if !config.QueryCache.Enabled {
		t.Error("Expected QueryCache.Enabled to be true")
	}

	if config.QueryCache.TTL != time.Minute {
		t.Errorf("Expected QueryCache.TTL to be 1m, got %v", config.QueryCache.TTL)
	}

	// 测试 SchemaCacheConfig 结构
	if !config.SchemaCache.Enabled {
		t.Error("Expected SchemaCache.Enabled to be true")
	}

	if config.SchemaCache.TTL != time.Minute*2 {
		t.Errorf("Expected SchemaCache.TTL to be 2m, got %v", config.SchemaCache.TTL)
	}

	// 测试 PlanCacheConfig 结构
	if !config.PlanCache.Enabled {
		t.Error("Expected PlanCache.Enabled to be true")
	}

	if config.PlanCache.TTL != time.Minute*3 {
		t.Errorf("Expected PlanCache.TTL to be 3m, got %v", config.PlanCache.TTL)
	}
}

func TestCacheStats_Struct(t *testing.T) {
	stats := &CacheStats{
		TotalHits:    100,
		TotalMisses:  50,
		TotalSets:    150,
		TotalEvicts:  10,
		QueryHits:    80,
		QueryMisses:  20,
		QuerySets:    100,
		SchemaHits:   15,
		SchemaMisses: 25,
		SchemaSets:   40,
		PlanHits:     5,
		PlanMisses:   5,
		PlanSets:     10,
		HitRate:      0.67,
		Size:         1000,
		LastCleanup:  time.Now(),
	}

	if stats.TotalHits != 100 {
		t.Errorf("Expected TotalHits to be 100, got %d", stats.TotalHits)
	}

	if stats.HitRate != 0.67 {
		t.Errorf("Expected HitRate to be 0.67, got %f", stats.HitRate)
	}

	if stats.Size != 1000 {
		t.Errorf("Expected Size to be 1000, got %d", stats.Size)
	}
}

func TestCacheEntry_Struct(t *testing.T) {
	now := time.Now()
	entry := &CacheEntry{
		Key:         "test-key",
		Value:       "test-value",
		ExpiresAt:   now.Add(time.Minute),
		CreatedAt:   now,
		AccessedAt:  now,
		AccessCount: 5,
		Size:        100,
	}

	if entry.Key != "test-key" {
		t.Errorf("Expected Key to be 'test-key', got %s", entry.Key)
	}

	if entry.Value != "test-value" {
		t.Errorf("Expected Value to be 'test-value', got %v", entry.Value)
	}

	if entry.AccessCount != 5 {
		t.Errorf("Expected AccessCount to be 5, got %d", entry.AccessCount)
	}

	if entry.Size != 100 {
		t.Errorf("Expected Size to be 100, got %d", entry.Size)
	}
}

func TestMemoryCache_Interfaces(t *testing.T) {
	// 验证 MemoryCache 实现了 Cache 接口
	var _ Cache = &MemoryCache{}

	// 验证所有必需的方法都存在
	logger := &MockLogger{}
	cache := NewMemoryCache(nil, logger)

	// 这些方法应该存在（即使我们不测试它们的功能）
	_ = cache.GetQuery
	_ = cache.SetQuery
	_ = cache.InvalidateQuery
	_ = cache.GetSchema
	_ = cache.SetSchema
	_ = cache.InvalidateSchema
	_ = cache.GetPlan
	_ = cache.SetPlan
	_ = cache.InvalidatePlan
	_ = cache.Clear
	_ = cache.Size
	_ = cache.Stats
}
