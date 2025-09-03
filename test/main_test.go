package main

import (
	"testing"

	"envoy-wasm-graphql-federation/pkg/config"
	"envoy-wasm-graphql-federation/pkg/errors"
	"envoy-wasm-graphql-federation/pkg/parser"
	"envoy-wasm-graphql-federation/pkg/planner"
	"envoy-wasm-graphql-federation/pkg/utils"
)

func TestBasicComponents(t *testing.T) {
	logger := utils.NewLogger("test")

	// 测试解析器创建
	parser := parser.NewParser(logger)
	if parser == nil {
		t.Fatal("Failed to create parser")
	}

	// 测试规划器创建
	planner := planner.NewPlanner(logger)
	if planner == nil {
		t.Fatal("Failed to create planner")
	}

	// 测试配置管理器创建
	configManager := config.NewManager(logger)
	if configManager == nil {
		t.Fatal("Failed to create config manager")
	}

	t.Log("All basic components created successfully")
}

func TestGraphQLQueryParsing(t *testing.T) {
	logger := utils.NewLogger("test")
	parser := parser.NewParser(logger)

	// 测试简单查询解析
	query := `{
		users {
			id
			name
		}
	}`

	parsedQuery, err := parser.ParseQuery(query)
	if err != nil {
		t.Fatalf("Failed to parse query: %v", err)
	}

	if parsedQuery == nil {
		t.Fatal("Parsed query is nil")
	}

	t.Logf("Query parsed successfully: complexity=%d, depth=%d",
		parsedQuery.Complexity, parsedQuery.Depth)
}

func TestConfigurationLoading(t *testing.T) {
	logger := utils.NewLogger("test")
	configManager := config.NewManager(logger)

	// 测试配置加载
	configJSON := `{
		"services": [
			{
				"name": "test-service",
				"endpoint": "http://localhost:4000/graphql",
				"timeout": 5000000000,
				"weight": 1,
				"schema": "type Query { test: String }"
			}
		],
		"federation": {
			"enableQueryPlanning": true,
			"enableCaching": false,
			"maxQueryDepth": 10,
			"queryTimeout": 30000000000
		}
	}`

	config, err := configManager.LoadConfig([]byte(configJSON))
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	if len(config.Services) != 1 {
		t.Fatalf("Expected 1 service, got %d", len(config.Services))
	}

	if config.Services[0].Name != "test-service" {
		t.Fatalf("Expected service name 'test-service', got '%s'", config.Services[0].Name)
	}

	t.Log("Configuration loaded successfully")
}

func TestErrorHandling(t *testing.T) {
	// 测试错误创建
	err := errors.NewQueryParsingError("test error")
	if err == nil {
		t.Fatal("Failed to create error")
	}

	// 测试错误转换为 GraphQL 格式
	graphqlError := err.ToGraphQLError()
	if graphqlError["message"] != "test error" {
		t.Fatal("Error message conversion failed")
	}

	t.Log("Error handling works correctly")
}
