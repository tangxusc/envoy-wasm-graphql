package main

import (
	"testing"
	"time"

	"envoy-wasm-graphql-federation/pkg/cache"
	"envoy-wasm-graphql-federation/pkg/caller"
	"envoy-wasm-graphql-federation/pkg/config"
	"envoy-wasm-graphql-federation/pkg/jsonutil"
	federationtypes "envoy-wasm-graphql-federation/pkg/types"
	"envoy-wasm-graphql-federation/pkg/utils"
)

// TestTinyGoCompatibility 测试TinyGo兼容性修复
func TestTinyGoCompatibility(t *testing.T) {
	logger := utils.NewLogger("test")

	// 测试1: JSON工具包
	t.Run("JSONUtil", func(t *testing.T) {
		testData := map[string]interface{}{
			"name": "test",
			"age":  30,
			"tags": []string{"tag1", "tag2"},
		}

		// 测试序列化
		jsonBytes, err := jsonutil.Marshal(testData)
		if err != nil {
			t.Fatalf("JSON marshal failed: %v", err)
		}

		// 测试反序列化
		var result map[string]interface{}
		err = jsonutil.Unmarshal(jsonBytes, &result)
		if err != nil {
			t.Fatalf("JSON unmarshal failed: %v", err)
		}

		if result["name"] != "test" {
			t.Errorf("Expected name=test, got %v", result["name"])
		}
	})

	// 测试2: URL验证
	t.Run("URLValidation", func(t *testing.T) {
		validURLs := []string{
			"http://example.com",
			"https://api.example.com",
			"http://localhost:8080",
			"https://subdomain.example.com/path",
		}

		invalidURLs := []string{
			"",
			"not-a-url",
			"ftp://example.com",
			"http://",
			"https://",
		}

		for _, url := range validURLs {
			if !utils.IsValidURL(url) {
				t.Errorf("Valid URL %s was marked as invalid", url)
			}
		}

		for _, url := range invalidURLs {
			if utils.IsValidURL(url) {
				t.Errorf("Invalid URL %s was marked as valid", url)
			}
		}
	})

	// 测试3: 查询参数解析
	t.Run("QueryParam", func(t *testing.T) {
		query := "name=test&age=30&tags=tag1,tag2"

		name := utils.GetQueryParam(query, "name")
		if name != "test" {
			t.Errorf("Expected name=test, got %s", name)
		}

		age := utils.GetQueryParam(query, "age")
		if age != "30" {
			t.Errorf("Expected age=30, got %s", age)
		}

		notFound := utils.GetQueryParam(query, "notfound")
		if notFound != "" {
			t.Errorf("Expected empty string for not found param, got %s", notFound)
		}
	})

	// 测试4: 缓存功能
	t.Run("Cache", func(t *testing.T) {
		// 创建缓存配置
		cacheConfig := cache.DefaultCacheConfig()

		c := cache.NewMemoryCache(cacheConfig, logger)

		// 测试查询缓存
		testKey := "test-query"
		testResponse := &federationtypes.GraphQLResponse{
			Data: map[string]interface{}{
				"test": "data",
			},
		}

		err := c.SetQuery(testKey, testResponse, time.Minute)
		if err != nil {
			t.Fatalf("SetQuery failed: %v", err)
		}

		cached, exists := c.GetQuery(testKey)
		if !exists {
			t.Errorf("Cache query should exist")
			return
		}

		if cached == nil || cached.Data == nil {
			t.Errorf("Cached query data should not be nil")
			return
		}

		// 测试统计
		stats := c.Stats()
		if stats.QuerySets == 0 {
			t.Errorf("Query sets should be greater than 0")
		}
	})

	// 测试5: 配置管理器
	t.Run("ConfigManager", func(t *testing.T) {
		manager := config.NewManager(logger)

		// 测试配置数据
		configData := `{
			"services": [
				{
					"name": "test-service",
					"endpoint": "http://example.com/graphql",
					"path": "/graphql",
					"schema": "type Query { test: String }",
					"weight": 1,
					"timeout": 5000000000
				}
			],
			"enableQueryPlan": true,
			"enableCaching": true,
			"maxQueryDepth": 10,
			"queryTimeout": 30000000000
		}`

		config, err := manager.LoadConfig([]byte(configData))
		if err != nil {
			t.Fatalf("Config loading failed: %v", err)
		}

		if len(config.Services) != 1 {
			t.Errorf("Expected 1 service, got %d", len(config.Services))
		}

		if config.Services[0].Name != "test-service" {
			t.Errorf("Expected service name=test-service, got %s", config.Services[0].Name)
		}

		if config.Services[0].Path != "/graphql" {
			t.Errorf("Expected service path=/graphql, got %s", config.Services[0].Path)
		}
	})

	// 测试6: WASM调用器
	t.Run("WASMCaller", func(t *testing.T) {
		callerConfig := &caller.CallerConfig{
			DefaultTimeout:   10 * time.Second,
			MaxRetries:       3,
			HealthCheckCache: 30 * time.Second,
		}

		caller := caller.NewHTTPCaller(callerConfig, logger)

		// 测试服务配置
		service := &federationtypes.ServiceConfig{
			Name:     "test-service",
			Endpoint: "http://example.com/graphql",
			Path:     "/graphql", // 新增的Path字段
			Schema:   "type Query { test: String }",
			Weight:   1,
			Timeout:  5 * time.Second,
		}

		// 注意：这里我们只能测试健康检查，因为实际的Call方法需要WASM环境
		// 测试健康检查（这会使用简化的实现）
		healthy := caller.IsHealthy(nil, service)
		// 在模拟环境中应该返回true
		if !healthy {
			t.Errorf("Service should be healthy in test environment")
		}

		// 测试新的Path字段
		if service.Path != "/graphql" {
			t.Errorf("Expected service path=/graphql, got %s", service.Path)
		}
	})
}
