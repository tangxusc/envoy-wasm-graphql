package main

import (
	"context"
	"envoy-wasm-graphql-federation/pkg/jsonutil"
	"testing"
	"time"

	"envoy-wasm-graphql-federation/pkg/cache"
	"envoy-wasm-graphql-federation/pkg/caller"
	"envoy-wasm-graphql-federation/pkg/config"
	"envoy-wasm-graphql-federation/pkg/errors"
	"envoy-wasm-graphql-federation/pkg/federation"
	"envoy-wasm-graphql-federation/pkg/merger"
	"envoy-wasm-graphql-federation/pkg/parser"
	"envoy-wasm-graphql-federation/pkg/planner"
	"envoy-wasm-graphql-federation/pkg/registry"
	federationtypes "envoy-wasm-graphql-federation/pkg/types"
	"envoy-wasm-graphql-federation/pkg/utils"
)

// TestFederationEngineIntegration 测试联邦引擎的集成
func TestFederationEngineIntegration(t *testing.T) {
	t.Run("BasicEngineOperation", testBasicEngineOperation)
	t.Run("ConfigurationManagement", testConfigurationManagement)
	t.Run("ErrorHandling", testErrorHandling)
	t.Run("CacheIntegration", testCacheIntegration)
	t.Run("ComponentIntegration", testComponentIntegration)
}

// testBasicEngineOperation 测试基本引擎操作
func testBasicEngineOperation(t *testing.T) {
	logger := utils.NewLogger("test")

	// 创建基本配置
	config := &federationtypes.FederationConfig{
		Services: []federationtypes.ServiceConfig{
			{
				Name:     "user-service",
				Endpoint: "http://localhost:8001/graphql",
				Schema:   "type User { id: ID! name: String! }",
				Weight:   1,
				Timeout:  5 * time.Second,
			},
			{
				Name:     "product-service",
				Endpoint: "http://localhost:8002/graphql",
				Schema:   "type Product { id: ID! name: String! price: Float! }",
				Weight:   1,
				Timeout:  5 * time.Second,
			},
		},
		EnableQueryPlan:  true,
		EnableCaching:    true,
		MaxQueryDepth:    10,
		QueryTimeout:     30 * time.Second,
		EnableIntrospect: true,
		DebugMode:        true,
	}

	// 创建联邦引擎
	engine, err := federation.NewEngine(config, logger)
	if err != nil {
		t.Fatalf("Failed to create federation engine: %v", err)
	}

	// 初始化引擎
	err = engine.Initialize(config)
	if err != nil {
		t.Fatalf("Failed to initialize engine: %v", err)
	}

	// 验证引擎状态
	status := engine.GetStatus()
	if status.Status != "running" {
		t.Errorf("Expected engine status to be 'running', got '%s'", status.Status)
	}

	if len(status.Services) != 2 {
		t.Errorf("Expected 2 services, got %d", len(status.Services))
	}

	// 关闭引擎
	err = engine.Shutdown()
	if err != nil {
		t.Errorf("Failed to shutdown engine: %v", err)
	}

	// 验证关闭后状态
	statusAfterShutdown := engine.GetStatus()
	if statusAfterShutdown.Status == "running" {
		t.Error("Engine should not be running after shutdown")
	}
}

// testConfigurationManagement 测试配置管理
func testConfigurationManagement(t *testing.T) {
	logger := utils.NewLogger("test")

	// 创建配置管理器
	configManager := config.NewManager(logger)

	// 测试基本配置加载
	configData := []byte(`{
		"services": [
			{
				"name": "test-service",
				"endpoint": "http://localhost:8000/graphql",
				"schema": "type Query { hello: String }",
				"weight": 1
			}
		],
		"enableQueryPlanning": true,
		"enableCaching": false,
		"maxQueryDepth": 10,
		"enableIntrospection": true,
		"debugMode": true
	}`)

	loadedConfig, err := configManager.LoadConfig(configData)
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	if len(loadedConfig.Services) != 1 {
		t.Errorf("Expected 1 service, got %d", len(loadedConfig.Services))
	}

	if loadedConfig.Services[0].Name != "test-service" {
		t.Errorf("Expected service name 'test-service', got '%s'", loadedConfig.Services[0].Name)
	}

	// 测试配置验证
	err = configManager.ValidateConfig(loadedConfig)
	if err != nil {
		t.Errorf("Config validation failed: %v", err)
	}

	// 测试无效配置
	invalidConfigData := []byte(`{
		"services": [],
		"maxQueryDepth": -1
	}`)

	_, err = configManager.LoadConfig(invalidConfigData)
	if err == nil {
		t.Error("Expected validation error for invalid config")
	}
}

// testErrorHandling 测试错误处理
func testErrorHandling(t *testing.T) {
	// 测试错误创建
	parseError := errors.NewQueryParsingError("Invalid query syntax")
	if parseError == nil {
		t.Fatal("Expected error to be created")
	}

	// 测试错误转换为GraphQL格式
	graphqlErr := parseError.ToGraphQLError()
	if graphqlErr["message"] == nil {
		t.Error("Expected message in GraphQL error")
	}

	if extensions, ok := graphqlErr["extensions"].(map[string]interface{}); ok {
		if extensions["code"] == nil {
			t.Error("Expected code in error extensions")
		}
	} else {
		t.Error("Expected extensions in GraphQL error")
	}

	// 测试错误收集器
	collector := errors.NewErrorCollector()

	collector.Add(errors.NewQueryParsingError("Error 1"))
	collector.Add(errors.NewQueryValidationError("Error 2"))
	collector.AddError(errors.NewServiceError("Error 3"))

	if !collector.HasErrors() {
		t.Error("Expected collector to have errors")
	}

	if collector.Count() != 3 {
		t.Errorf("Expected 3 errors, got %d", collector.Count())
	}

	graphqlErrors := collector.ToGraphQLErrors()
	if len(graphqlErrors) == 0 {
		t.Error("Expected GraphQL errors to be generated")
	}
}

// testCacheIntegration 测试缓存集成
func testCacheIntegration(t *testing.T) {
	logger := utils.NewLogger("test")

	// 创建缓存
	cacheConfig := cache.DefaultCacheConfig()
	cacheInstance := cache.NewMemoryCache(cacheConfig, logger)

	// 测试查询缓存
	response := &federationtypes.GraphQLResponse{
		Data: map[string]interface{}{
			"user": map[string]interface{}{
				"id":   "1",
				"name": "John Doe",
			},
		},
	}

	keyGen := cache.NewCacheKeyGenerator()
	cacheKey := keyGen.GenerateQueryKey("query { user { id name } }", nil, "")

	// 存储到缓存
	err := cacheInstance.SetQuery(cacheKey, response, 5*time.Minute)
	if err != nil {
		t.Errorf("Failed to set query cache: %v", err)
	}

	// 从缓存获取
	cachedResponse, found := cacheInstance.GetQuery(cacheKey)
	if !found {
		t.Error("Expected to find cached query response")
	}

	if cachedResponse == nil {
		t.Error("Expected cached response to not be nil")
	}

	// 验证缓存统计
	stats := cacheInstance.Stats()
	if stats.QueryHits != 1 {
		t.Errorf("Expected 1 query hit, got %d", stats.QueryHits)
	}

	if stats.QuerySets != 1 {
		t.Errorf("Expected 1 query set, got %d", stats.QuerySets)
	}

	// 测试模式缓存
	schema := &federationtypes.Schema{
		SDL: "type User { id: ID! name: String! }",
		Types: map[string]*federationtypes.TypeDefinition{
			"User": {
				Name: "User",
				Kind: "OBJECT",
				Fields: map[string]*federationtypes.FieldDefinition{
					"id":   {Name: "id", Type: "ID!"},
					"name": {Name: "name", Type: "String!"},
				},
			},
		},
		Version: "v1.0.0",
	}

	err = cacheInstance.SetSchema("user-service", schema, 10*time.Minute)
	if err != nil {
		t.Errorf("Failed to set schema cache: %v", err)
	}

	cachedSchema, found := cacheInstance.GetSchema("user-service")
	if !found {
		t.Error("Expected to find cached schema")
	}

	if cachedSchema == nil || cachedSchema.SDL != schema.SDL {
		t.Error("Cached schema does not match original")
	}

	// 清理缓存
	err = cacheInstance.Clear()
	if err != nil {
		t.Errorf("Failed to clear cache: %v", err)
	}

	// 验证缓存已清空
	if cacheInstance.Size() != 0 {
		t.Errorf("Expected cache size to be 0 after clear, got %d", cacheInstance.Size())
	}
}

// testComponentIntegration 测试组件集成
func testComponentIntegration(t *testing.T) {
	logger := utils.NewLogger("test")

	// 测试Parser组件
	parserInstance := parser.NewParser(logger)
	query := "query { user(id: \"1\") { id name email } }"

	parsedQuery, err := parserInstance.ParseQuery(query)
	if err != nil {
		t.Errorf("Failed to parse query: %v", err)
	}

	if parsedQuery == nil {
		t.Fatal("Expected parsed query to not be nil")
	}

	if parsedQuery.Complexity <= 0 {
		t.Error("Expected query complexity to be greater than 0")
	}

	// 测试Planner组件
	plannerInstance := planner.NewPlanner(logger)

	services := []federationtypes.ServiceConfig{
		{
			Name:     "user-service",
			Endpoint: "http://localhost:8001/graphql",
			Schema:   "type User { id: ID! name: String! email: String! }",
		},
	}

	ctx := context.Background()
	plan, err := plannerInstance.CreateExecutionPlan(ctx, parsedQuery, services)
	if err != nil {
		t.Errorf("Failed to create execution plan: %v", err)
	}

	if plan == nil {
		t.Fatal("Expected execution plan to not be nil")
	}

	if len(plan.SubQueries) == 0 {
		t.Error("Expected execution plan to have sub-queries")
	}

	// 验证计划
	err = plannerInstance.ValidatePlan(plan)
	if err != nil {
		t.Errorf("Plan validation failed: %v", err)
	}

	// 测试优化
	optimizedPlan, err := plannerInstance.OptimizePlan(plan)
	if err != nil {
		t.Errorf("Plan optimization failed: %v", err)
	}

	if optimizedPlan == nil {
		t.Error("Expected optimized plan to not be nil")
	}

	// 测试Merger组件
	mergerInstance := merger.NewResponseMerger(nil, logger)

	responses := []*federationtypes.ServiceResponse{
		{
			Service: "user-service",
			Data: map[string]interface{}{
				"user": map[string]interface{}{
					"id":    "1",
					"name":  "John Doe",
					"email": "john@example.com",
				},
			},
			Latency: 100 * time.Millisecond,
		},
	}

	mergedResponse, err := mergerInstance.MergeResponses(ctx, responses, plan)
	if err != nil {
		t.Errorf("Failed to merge responses: %v", err)
	}

	if mergedResponse == nil {
		t.Fatal("Expected merged response to not be nil")
	}

	if mergedResponse.Data == nil {
		t.Error("Expected merged response to have data")
	}

	// 测试ServiceCaller组件 (模拟测试)
	callerConfig := caller.DefaultCallerConfig()
	callerInstance := caller.NewHTTPCaller(callerConfig, logger)

	// 由于这是集成测试，我们只测试健康检查逻辑
	healthyService := &federationtypes.ServiceConfig{
		Name:     "test-service",
		Endpoint: "http://example.com/graphql",
		HealthCheck: &federationtypes.HealthCheck{
			Enabled: false, // 禁用实际健康检查以避免网络调用
		},
	}

	// 这应该返回true因为健康检查被禁用，会执行简单检查
	// 在实际环境中会进行真实的网络调用
	isHealthy := callerInstance.IsHealthy(ctx, healthyService)
	// 由于没有实际服务运行，这可能返回false，这是正常的
	_ = isHealthy

	// 测试Registry组件
	registryInstance := registry.NewSchemaRegistry(nil, logger)

	testSchema := "type User { id: ID! name: String! }"
	err = registryInstance.RegisterSchema("test-service", testSchema)
	if err != nil {
		t.Errorf("Failed to register schema: %v", err)
	}

	retrievedSchema, err := registryInstance.GetSchema("test-service")
	if err != nil {
		t.Errorf("Failed to get schema: %v", err)
	}

	if retrievedSchema == nil {
		t.Error("Expected retrieved schema to not be nil")
	}

	if retrievedSchema.Schema != testSchema {
		t.Error("Retrieved schema does not match registered schema")
	}

	// 验证模式
	err = registryInstance.ValidateSchema(testSchema)
	if err != nil {
		t.Errorf("Schema validation failed: %v", err)
	}

	// 测试联邦模式构建
	federatedSchema, err := registryInstance.GetFederatedSchema()
	if err != nil {
		t.Errorf("Failed to get federated schema: %v", err)
	}

	if federatedSchema == nil {
		t.Error("Expected federated schema to not be nil")
	}
}

// TestUtilityFunctions 测试工具函数
func TestUtilityFunctions(t *testing.T) {
	// 测试日志记录器
	logger := utils.NewLogger("test")
	logger.Info("Test info message")
	logger.Debug("Test debug message")
	logger.Warn("Test warning message")
	logger.Error("Test error message")

	// 测试请求ID生成
	requestID := utils.GenerateRequestID()
	if requestID == "" {
		t.Error("Expected request ID to not be empty")
	}

	// 测试字符串工具
	sanitized := utils.SanitizeString("test\nstring\twith\rcontrol\x00chars")
	if sanitized == "test\nstring\twith\rcontrol\x00chars" {
		t.Error("Expected string to be sanitized")
	}

	truncated := utils.TruncateString("this is a very long string that should be truncated", 20)
	if len(truncated) > 20 {
		t.Error("Expected string to be truncated")
	}

	// 测试头部合并
	base := map[string]string{"Content-Type": "application/json"}
	override := map[string]string{"Authorization": "Bearer token"}
	merged := utils.MergeHeaders(base, override)

	if len(merged) != 2 {
		t.Errorf("Expected 2 headers, got %d", len(merged))
	}

	if merged["Content-Type"] != "application/json" {
		t.Error("Expected Content-Type header to be preserved")
	}

	if merged["Authorization"] != "Bearer token" {
		t.Error("Expected Authorization header to be added")
	}

	// 测试字符串数组操作
	slice := []string{"a", "b", "c", "b", "d"}
	unique := utils.UniqueStrings(slice)
	if len(unique) != 4 {
		t.Errorf("Expected 4 unique strings, got %d", len(unique))
	}

	contains := utils.ContainsString(slice, "b")
	if !contains {
		t.Error("Expected slice to contain 'b'")
	}

	removed := utils.RemoveString(slice, "b")
	if utils.ContainsString(removed, "b") {
		t.Error("Expected 'b' to be removed from slice")
	}

	// 测试GraphQL名称验证
	if !utils.IsValidGraphQLName("validName") {
		t.Error("Expected 'validName' to be a valid GraphQL name")
	}

	if utils.IsValidGraphQLName("123invalid") {
		t.Error("Expected '123invalid' to be an invalid GraphQL name")
	}

	if utils.IsValidGraphQLName("") {
		t.Error("Expected empty string to be an invalid GraphQL name")
	}

	// 测试持续时间解析
	duration, err := utils.ParseDuration("5s")
	if err != nil {
		t.Errorf("Failed to parse duration: %v", err)
	}

	if duration != 5*time.Second {
		t.Errorf("Expected 5 seconds, got %v", duration)
	}

	formatted := utils.FormatDuration(duration)
	if formatted == "" {
		t.Error("Expected formatted duration to not be empty")
	}

	// 测试数学工具
	if utils.Min(5, 3) != 3 {
		t.Error("Expected Min(5, 3) to return 3")
	}

	if utils.Max(5, 3) != 5 {
		t.Error("Expected Max(5, 3) to return 5")
	}

	clamped := utils.ClampInt(10, 0, 5)
	if clamped != 5 {
		t.Errorf("Expected ClampInt(10, 0, 5) to return 5, got %d", clamped)
	}

	// 测试哈希函数
	hash := utils.HashString("test string")
	if hash == 0 {
		t.Error("Expected hash to not be zero")
	}

	// 相同字符串应该产生相同的哈希
	hash2 := utils.HashString("test string")
	if hash != hash2 {
		t.Error("Expected identical strings to produce identical hashes")
	}
}

// TestConfigurationValidation 测试配置验证
func TestConfigurationValidation(t *testing.T) {
	// 测试默认验证器
	validator := config.NewDefaultValidator()

	// 测试有效配置
	validConfig := &federationtypes.FederationConfig{
		Services: []federationtypes.ServiceConfig{
			{
				Name:     "valid-service",
				Endpoint: "http://localhost:8000/graphql",
				Weight:   1,
				Timeout:  5 * time.Second,
			},
		},
		QueryTimeout:  30 * time.Second,
		MaxQueryDepth: 10,
	}

	errors := validator.Validate(validConfig)
	if len(errors) > 0 {
		t.Errorf("Expected no validation errors for valid config, got %d", len(errors))
	}

	// 测试无效配置
	invalidConfig := &federationtypes.FederationConfig{
		Services:      []federationtypes.ServiceConfig{}, // 空服务列表
		QueryTimeout:  -1 * time.Second,                  // 无效超时
		MaxQueryDepth: -1,                                // 无效深度
	}

	errors = validator.Validate(invalidConfig)
	if len(errors) == 0 {
		t.Error("Expected validation errors for invalid config")
	}

	// 验证错误类型
	hasServiceError := false
	hasTimeoutError := false
	hasDepthError := false

	for _, err := range errors {
		switch err.Code {
		case "NO_SERVICES":
			hasServiceError = true
		case "INVALID_TIMEOUT":
			hasTimeoutError = true
		case "INVALID_DEPTH":
			hasDepthError = true
		}
	}

	if !hasServiceError {
		t.Error("Expected NO_SERVICES validation error")
	}

	if !hasTimeoutError {
		t.Error("Expected INVALID_TIMEOUT validation error")
	}

	if !hasDepthError {
		t.Error("Expected INVALID_DEPTH validation error")
	}

	// 测试变更检测器
	detector := config.NewDefaultChangeDetector()

	oldConfig := &federationtypes.FederationConfig{
		EnableQueryPlan: false,
		EnableCaching:   true,
		Services: []federationtypes.ServiceConfig{
			{Name: "service1", Endpoint: "http://old.example.com"},
		},
	}

	newConfig := &federationtypes.FederationConfig{
		EnableQueryPlan: true, // 变更
		EnableCaching:   true, // 无变更
		Services: []federationtypes.ServiceConfig{
			{Name: "service1", Endpoint: "http://new.example.com"},  // 变更
			{Name: "service2", Endpoint: "http://new2.example.com"}, // 新增
		},
	}

	changes := detector.DetectChanges(oldConfig, newConfig)
	if len(changes) == 0 {
		t.Error("Expected configuration changes to be detected")
	}

	// 验证特定变更
	hasQueryPlanChange := false
	hasEndpointChange := false
	hasServiceAdd := false

	for _, change := range changes {
		switch {
		case change.Path == "enableQueryPlan":
			hasQueryPlanChange = true
		case change.Path == "services.service1.endpoint":
			hasEndpointChange = true
		case change.Path == "services.service2" && change.Type == config.ChangeTypeAdded:
			hasServiceAdd = true
		}
	}

	if !hasQueryPlanChange {
		t.Error("Expected query plan change to be detected")
	}

	if !hasEndpointChange {
		t.Error("Expected endpoint change to be detected")
	}

	if !hasServiceAdd {
		t.Error("Expected service addition to be detected")
	}
}

// TestTypesSerialization 测试类型序列化
func TestTypesSerialization(t *testing.T) {
	// 测试GraphQL响应序列化
	response := &federationtypes.GraphQLResponse{
		Data: map[string]interface{}{
			"user": map[string]interface{}{
				"id":   "1",
				"name": "John Doe",
			},
		},
		Errors: []federationtypes.GraphQLError{
			{
				Message: "Test error",
				Extensions: map[string]interface{}{
					"code": "TEST_ERROR",
				},
			},
		},
		Extensions: map[string]interface{}{
			"requestId": "req_123",
		},
	}

	// 序列化为JSON
	data, err := jsonutil.Marshal(response)
	if err != nil {
		t.Errorf("Failed to marshal GraphQL response: %v", err)
	}

	// 反序列化
	var deserialized federationtypes.GraphQLResponse
	err = jsonutil.Unmarshal(data, &deserialized)
	if err != nil {
		t.Errorf("Failed to unmarshal GraphQL response: %v", err)
	}

	// 验证数据完整性
	if deserialized.Data == nil {
		t.Error("Expected data to be preserved after serialization")
	}

	if len(deserialized.Errors) != 1 {
		t.Errorf("Expected 1 error, got %d", len(deserialized.Errors))
	}

	if deserialized.Extensions["requestId"] != "req_123" {
		t.Error("Expected requestId to be preserved in extensions")
	}

	// 测试执行计划序列化
	plan := &federationtypes.ExecutionPlan{
		SubQueries: []federationtypes.SubQuery{
			{
				ServiceName: "user-service",
				Query:       "query { user { id name } }",
				Variables:   map[string]interface{}{"id": "1"},
			},
		},
		Dependencies: map[string][]string{
			"user-service": {},
		},
		MergeStrategy: federationtypes.MergeStrategyShallow,
		Metadata: map[string]interface{}{
			"complexity": 5,
		},
	}

	planData, err := jsonutil.Marshal(plan)
	if err != nil {
		t.Errorf("Failed to marshal execution plan: %v", err)
	}

	var deserializedPlan federationtypes.ExecutionPlan
	err = jsonutil.Unmarshal(planData, &deserializedPlan)
	if err != nil {
		t.Errorf("Failed to unmarshal execution plan: %v", err)
	}

	if len(deserializedPlan.SubQueries) != 1 {
		t.Errorf("Expected 1 sub-query, got %d", len(deserializedPlan.SubQueries))
	}

	if deserializedPlan.SubQueries[0].ServiceName != "user-service" {
		t.Error("Expected service name to be preserved")
	}

	if deserializedPlan.MergeStrategy != federationtypes.MergeStrategyShallow {
		t.Error("Expected merge strategy to be preserved")
	}
}
