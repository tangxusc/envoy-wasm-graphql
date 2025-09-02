package federation

import (
	"context"
	"envoy-wasm-graphql-federation/pkg/jsonutil"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"envoy-wasm-graphql-federation/pkg/caller"
	"envoy-wasm-graphql-federation/pkg/errors"
	"envoy-wasm-graphql-federation/pkg/merger"
	"envoy-wasm-graphql-federation/pkg/parser"
	"envoy-wasm-graphql-federation/pkg/planner"
	"envoy-wasm-graphql-federation/pkg/registry"
	federationtypes "envoy-wasm-graphql-federation/pkg/types"
)

// Engine 实现 GraphQL Federation 引擎
type Engine struct {
	// 核心组件
	parser   federationtypes.GraphQLParser
	planner  federationtypes.QueryPlanner
	caller   federationtypes.ServiceCaller
	merger   federationtypes.ResponseMerger
	registry federationtypes.SchemaRegistry
	logger   federationtypes.Logger

	// Federation 相关组件
	directiveParser   federationtypes.FederationDirectiveParser
	federationPlanner federationtypes.FederationPlanner
	entityResolver    federationtypes.EntityResolver

	// 配置和状态
	federationConfig *federationtypes.FederationConfig
	status           federationtypes.EngineStatus
	startTime        time.Time

	// 统计信息
	queryCount int64
	errorCount int64
	mutex      sync.RWMutex
}

// NewEngine 创建新的联邦引擎
func NewEngine(config *federationtypes.FederationConfig, logger federationtypes.Logger) (*Engine, error) {
	if config == nil {
		return nil, fmt.Errorf("configuration is required")
	}

	if logger == nil {
		return nil, fmt.Errorf("logger is required")
	}

	engine := &Engine{
		federationConfig: config,
		logger:           logger,
		startTime:        time.Now(),
		status: federationtypes.EngineStatus{
			Status:   "initializing",
			Services: make(map[string]federationtypes.ServiceStatus),
		},
	}

	// 初始化组件
	engine.parser = parser.NewParser(logger)
	engine.planner = planner.NewPlanner(logger)

	// 初始化 Federation 组件
	engine.directiveParser = NewDirectiveParser(logger)
	engine.federationPlanner = NewFederatedPlanner(logger)
	engine.entityResolver = NewEntityResolver(logger, nil) // caller 将在后面初始化

	// 初始化其他组件
	engine.caller = caller.NewHTTPCaller(nil, logger)
	engine.merger = merger.NewResponseMerger(nil, logger)
	engine.registry = registry.NewSchemaRegistry(nil, logger)

	// 更新 entityResolver 的 caller
	engine.entityResolver = NewEntityResolver(logger, engine.caller)

	logger.Info("Federation engine created",
		"services", len(config.Services),
		"queryPlanning", config.EnableQueryPlan,
	)

	return engine, nil
}

// Initialize 初始化引擎
func (e *Engine) Initialize(config *federationtypes.FederationConfig) error {
	e.logger.Info("Initializing federation engine")

	e.mutex.Lock()
	defer e.mutex.Unlock()

	// 更新配置
	e.federationConfig = config

	// 初始化配置管理器
	// 配置已经通过构造函数传入，无需其他初始化

	// 注册服务模式到SchemaRegistry
	for _, service := range config.Services {
		if service.Schema != "" {
			if err := e.registry.RegisterSchema(service.Name, service.Schema); err != nil {
				e.logger.Warn("Failed to register schema", "service", service.Name, "error", err)
				// 不阻止初始化，只记录警告
			}
		}
	}

	// 初始化服务状态
	e.initializeServiceStatus()

	// 更新引擎状态
	e.status.Status = "running"
	e.status.Uptime = time.Since(e.startTime)

	e.logger.Info("Federation engine initialized successfully",
		"services", len(config.Services),
		"queryPlanning", config.EnableQueryPlan,
		"caching", config.EnableCaching,
	)
	return nil
}

// ExecuteQuery 执行 GraphQL 查询
func (e *Engine) ExecuteQuery(ctx *federationtypes.ExecutionContext, request *federationtypes.GraphQLRequest) (*federationtypes.GraphQLResponse, error) {
	if request == nil {
		return nil, errors.NewExecutionError("request is nil")
	}

	e.incrementQueryCount()

	e.logger.Info("Executing GraphQL query",
		"requestId", ctx.RequestID,
		"operation", request.OperationName,
	)

	// 解析查询
	parsedQuery, err := e.parser.ParseQuery(request.Query)
	if err != nil {
		e.incrementErrorCount()
		return nil, fmt.Errorf("query parsing failed: %w", err)
	}

	// 验证查询深度和复杂度
	if err := e.validateQueryLimits(parsedQuery); err != nil {
		e.incrementErrorCount()
		return nil, err
	}

	// 创建执行计划
	plan, err := e.createExecutionPlan(context.Background(), parsedQuery)
	if err != nil {
		e.incrementErrorCount()
		return nil, fmt.Errorf("planning failed: %w", err)
	}

	// 执行计划
	response, err := e.executePlan(context.Background(), plan, ctx)
	if err != nil {
		e.incrementErrorCount()
		return nil, fmt.Errorf("execution failed: %w", err)
	}

	duration := time.Since(ctx.StartTime)
	e.logger.Info("Query executed successfully",
		"requestId", ctx.RequestID,
		"duration", duration,
		"subQueries", len(plan.SubQueries),
	)

	return response, nil
}

// createExecutionPlan 创建执行计划
func (e *Engine) createExecutionPlan(ctx context.Context, query *federationtypes.ParsedQuery) (*federationtypes.ExecutionPlan, error) {
	services := e.federationConfig.Services

	// 创建基本计划
	plan, err := e.planner.CreateExecutionPlan(ctx, query, services)
	if err != nil {
		return nil, err
	}

	// 验证计划
	if err := e.planner.ValidatePlan(plan); err != nil {
		return nil, err
	}

	// 优化计划（如果启用）
	if e.federationConfig.EnableQueryPlan {
		optimizedPlan, err := e.planner.OptimizePlan(plan)
		if err != nil {
			e.logger.Warn("Plan optimization failed, using original plan", "error", err)
		} else {
			plan = optimizedPlan
		}
	}

	return plan, nil
}

// executePlan 执行计划
func (e *Engine) executePlan(ctx context.Context, plan *federationtypes.ExecutionPlan, execCtx *federationtypes.ExecutionContext) (*federationtypes.GraphQLResponse, error) {
	// 检查服务调用器和响应合并器是否初始化
	if e.caller == nil {
		return nil, errors.NewExecutionError("service caller not initialized")
	}

	if e.merger == nil {
		return nil, errors.NewExecutionError("response merger not initialized")
	}

	// 执行子查询
	responses, err := e.executeSubQueries(ctx, plan.SubQueries, execCtx)
	if err != nil {
		return nil, err
	}

	// 合并响应
	mergedResponse, err := e.merger.MergeResponses(ctx, responses, plan)
	if err != nil {
		return nil, fmt.Errorf("response merging failed: %w", err)
	}

	return mergedResponse, nil
}

// executeSubQueries 执行子查询（并发执行）
func (e *Engine) executeSubQueries(ctx context.Context, subQueries []federationtypes.SubQuery, execCtx *federationtypes.ExecutionContext) ([]*federationtypes.ServiceResponse, error) {
	if len(subQueries) == 0 {
		return nil, nil
	}

	e.logger.Debug("Executing sub-queries concurrently", "count", len(subQueries))

	responses := make([]*federationtypes.ServiceResponse, len(subQueries))
	errCh := make(chan error, len(subQueries))
	responseCh := make(chan struct {
		index    int
		response *federationtypes.ServiceResponse
	}, len(subQueries))

	// 创建上下文，支持超时和取消
	queryCtx, cancel := context.WithTimeout(ctx, execCtx.Config.QueryTimeout)
	defer cancel()

	// 并发执行子查询
	var wg sync.WaitGroup
	for i, subQuery := range subQueries {
		wg.Add(1)
		go func(index int, sq federationtypes.SubQuery) {
			defer wg.Done()

			startTime := time.Now()
			e.logger.Debug("Executing sub-query", "service", sq.ServiceName, "index", index)

			// 获取服务配置
			var serviceConfig *federationtypes.ServiceConfig
			for _, service := range e.federationConfig.Services {
				if service.Name == sq.ServiceName {
					serviceConfig = &service
					break
				}
			}
			if serviceConfig == nil {
				e.logger.Error("Service not found in configuration", "service", sq.ServiceName)
				errCh <- fmt.Errorf("service not found: %s", sq.ServiceName)
				return
			}

			// 检查服务健康状态
			if !e.caller.IsHealthy(queryCtx, serviceConfig) {
				e.logger.Warn("Service is unhealthy", "service", sq.ServiceName)
				response := &federationtypes.ServiceResponse{
					Service: sq.ServiceName,
					Error:   errors.NewServiceError("service is unhealthy: " + sq.ServiceName),
					Latency: time.Since(startTime),
				}
				responseCh <- struct {
					index    int
					response *federationtypes.ServiceResponse
				}{index, response}
				return
			}

			// 构建服务调用
			call := &federationtypes.ServiceCall{
				Service:   serviceConfig,
				SubQuery:  &sq,
				Context:   execCtx.QueryContext,
				StartTime: startTime,
			}

			// 执行调用
			response, err := e.caller.Call(queryCtx, call)
			if err != nil {
				e.logger.Error("Service call failed", "service", sq.ServiceName, "error", err)
				// 创建错误响应
				response = &federationtypes.ServiceResponse{
					Service: sq.ServiceName,
					Error:   err,
					Latency: time.Since(startTime),
					Metadata: map[string]interface{}{
						"error_type": "service_call_error",
						"query":      sq.Query,
					},
				}
			}

			e.logger.Debug("Sub-query completed",
				"service", sq.ServiceName,
				"latency", response.Latency,
				"hasError", response.Error != nil,
			)

			responseCh <- struct {
				index    int
				response *federationtypes.ServiceResponse
			}{index, response}
		}(i, subQuery)
	}

	// 等待所有goroutine完成
	go func() {
		wg.Wait()
		close(responseCh)
		close(errCh)
	}()

	// 收集结果
	completed := 0
	for completed < len(subQueries) {
		select {
		case result := <-responseCh:
			if result.response != nil {
				responses[result.index] = result.response
				completed++
			}
		case err := <-errCh:
			if err != nil {
				// 即使有错误，也继续等待其他查询完成
				e.logger.Error("Sub-query error", "error", err)
			}
		case <-queryCtx.Done():
			// 超时或取消
			e.logger.Warn("Sub-queries execution timeout or cancelled")
			return responses, queryCtx.Err()
		}
	}

	// 统计执行结果
	successful := 0
	failed := 0
	for _, resp := range responses {
		if resp != nil {
			if resp.Error == nil {
				successful++
			} else {
				failed++
			}
		}
	}

	e.logger.Info("Sub-queries execution completed",
		"total", len(subQueries),
		"successful", successful,
		"failed", failed,
	)

	return responses, nil
}

// validateQueryLimits 验证查询限制
func (e *Engine) validateQueryLimits(query *federationtypes.ParsedQuery) error {
	// 检查查询深度
	if e.federationConfig.MaxQueryDepth > 0 && query.Depth > e.federationConfig.MaxQueryDepth {
		return errors.NewQueryComplexityError(
			fmt.Sprintf("query depth %d exceeds maximum %d", query.Depth, e.federationConfig.MaxQueryDepth),
		)
	}

	// 这里可以添加更多限制检查，如复杂度分析等

	return nil
}

// Shutdown 关闭引擎
func (e *Engine) Shutdown() error {
	e.logger.Info("Shutting down federation engine")

	e.mutex.Lock()
	defer e.mutex.Unlock()

	e.status.Status = "shutdown"

	e.logger.Info("Federation engine shutdown completed")
	return nil
}

// GetStatus 获取引擎状态
func (e *Engine) GetStatus() federationtypes.EngineStatus {
	e.mutex.RLock()
	defer e.mutex.RUnlock()

	status := e.status
	status.Uptime = time.Since(e.startTime)
	status.QueryCount = e.queryCount
	status.ErrorCount = e.errorCount

	return status
}

// 私有辅助方法

// initializeServiceStatus 初始化服务状态
func (e *Engine) initializeServiceStatus() {
	e.status.Services = make(map[string]federationtypes.ServiceStatus)

	for _, service := range e.federationConfig.Services {
		e.status.Services[service.Name] = federationtypes.ServiceStatus{
			Name:         service.Name,
			Healthy:      true, // 假设初始状态为健康
			LastCheck:    time.Now(),
			ResponseTime: 0,
			ErrorRate:    0.0,
		}
	}
}

// serializeConfig 序列化配置
func (e *Engine) serializeConfig(config *federationtypes.FederationConfig) ([]byte, error) {
	return jsonutil.Marshal(config)
}

// incrementQueryCount 增加查询计数
func (e *Engine) incrementQueryCount() {
	atomic.AddInt64(&e.queryCount, 1)
}

// incrementErrorCount 增加错误计数
func (e *Engine) incrementErrorCount() {
	atomic.AddInt64(&e.errorCount, 1)
}

// IsHealthy 检查引擎健康状态
func (e *Engine) IsHealthy() bool {
	e.mutex.RLock()
	defer e.mutex.RUnlock()
	return e.status.Status == "running"
}

// GetMetrics 获取引擎指标
func (e *Engine) GetMetrics() map[string]interface{} {
	e.mutex.RLock()
	defer e.mutex.RUnlock()

	return map[string]interface{}{
		"uptime":        time.Since(e.startTime),
		"query_count":   e.queryCount,
		"error_count":   e.errorCount,
		"error_rate":    float64(e.errorCount) / float64(max(e.queryCount, 1)),
		"service_count": len(e.federationConfig.Services),
		"status":        e.status.Status,
	}
}

// max 返回两个整数中的较大值
func max(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}

// Federation 指令处理逻辑

// ProcessFederationDirectives 处理 Federation 指令
func (e *Engine) ProcessFederationDirectives(schema string) (*federationtypes.FederatedSchema, error) {
	e.logger.Info("Processing Federation directives")

	// 从模式中提取 Federation 实体
	entities, err := e.extractFederationEntities(schema)
	if err != nil {
		return nil, fmt.Errorf("failed to extract Federation entities: %w", err)
	}

	// 验证实体的有效性
	if err := e.validateFederationEntities(entities); err != nil {
		return nil, fmt.Errorf("Federation entities validation failed: %w", err)
	}

	// 构建联邦模式
	federatedSchema := &federationtypes.FederatedSchema{
		Entities: entities,
		Version:  "1.0",
	}

	e.logger.Info("Federation directives processed successfully", "entityCount", len(entities))
	return federatedSchema, nil
}

// extractFederationEntities 提取 Federation 实体
func (e *Engine) extractFederationEntities(schema string) ([]federationtypes.FederatedEntity, error) {
	// 使用解析器提取实体
	if parserImpl, ok := e.parser.(*parser.Parser); ok {
		return parserImpl.ExtractFederationEntities(schema)
	}

	return nil, errors.NewInternalError("parser does not support Federation entity extraction")
}

// validateFederationEntities 验证 Federation 实体
func (e *Engine) validateFederationEntities(entities []federationtypes.FederatedEntity) error {
	for _, entity := range entities {
		// 验证实体指令
		if err := e.directiveParser.ValidateDirectives(&entity.Directives); err != nil {
			return fmt.Errorf("entity %s directive validation failed: %w", entity.TypeName, err)
		}

		// 验证字段指令
		for _, field := range entity.Fields {
			if err := e.directiveParser.ValidateDirectives(&field.Directives); err != nil {
				return fmt.Errorf("field %s.%s directive validation failed: %w", entity.TypeName, field.Name, err)
			}
		}
	}

	return nil
}

// ExecuteFederationQuery 执行 Federation 查询
func (e *Engine) ExecuteFederationQuery(ctx *federationtypes.ExecutionContext, request *federationtypes.GraphQLRequest, entities []federationtypes.FederatedEntity) (*federationtypes.GraphQLResponse, error) {
	e.logger.Info("Executing Federation query", "entityCount", len(entities))

	// 解析查询
	parsedQuery, err := e.parser.ParseQuery(request.Query)
	if err != nil {
		e.incrementErrorCount()
		return nil, fmt.Errorf("query parsing failed: %w", err)
	}

	// 创建 Federation 执行计划
	plan, err := e.createFederationPlan(context.Background(), parsedQuery, entities)
	if err != nil {
		e.incrementErrorCount()
		return nil, fmt.Errorf("Federation planning failed: %w", err)
	}

	// 执行计划
	response, err := e.executeFederationPlan(context.Background(), plan, ctx)
	if err != nil {
		e.incrementErrorCount()
		return nil, fmt.Errorf("Federation execution failed: %w", err)
	}

	e.incrementQueryCount()
	e.logger.Info("Federation query executed successfully")
	return response, nil
}

// createFederationPlan 创建 Federation 执行计划
func (e *Engine) createFederationPlan(ctx context.Context, query *federationtypes.ParsedQuery, entities []federationtypes.FederatedEntity) (*federationtypes.FederationPlan, error) {
	// 使用 Federation 规划器创建计划
	return e.federationPlanner.PlanEntityResolution(entities, query)
}

// executeFederationPlan 执行 Federation 计划
func (e *Engine) executeFederationPlan(ctx context.Context, plan *federationtypes.FederationPlan, execCtx *federationtypes.ExecutionContext) (*federationtypes.GraphQLResponse, error) {
	var responses []*federationtypes.ServiceResponse

	// 按依赖顺序执行实体解析
	for _, serviceName := range plan.DependencyOrder {
		// 找到对应的实体解析
		for _, entityResolution := range plan.Entities {
			if entityResolution.ServiceName == serviceName {
				// 执行实体解析
				response, err := e.executeEntityResolution(ctx, &entityResolution)
				if err != nil {
					return nil, fmt.Errorf("entity resolution failed for %s: %w", entityResolution.TypeName, err)
				}
				responses = append(responses, response)
			}
		}
	}

	// 合并响应
	return e.mergeFederationResponses(responses)
}

// executeEntityResolution 执行实体解析
func (e *Engine) executeEntityResolution(ctx context.Context, resolution *federationtypes.EntityResolution) (*federationtypes.ServiceResponse, error) {
	// 构建服务调用
	serviceCall := &federationtypes.ServiceCall{
		Service: &federationtypes.ServiceConfig{
			Name: resolution.ServiceName,
		},
		SubQuery: &federationtypes.SubQuery{
			ServiceName: resolution.ServiceName,
			Query:       resolution.Query,
		},
		Context: &federationtypes.QueryContext{
			RequestID: "federation-entity-resolution",
		},
	}

	// 调用服务
	return e.caller.Call(ctx, serviceCall)
}

// mergeFederationResponses 合并 Federation 响应
func (e *Engine) mergeFederationResponses(responses []*federationtypes.ServiceResponse) (*federationtypes.GraphQLResponse, error) {
	if len(responses) == 0 {
		return &federationtypes.GraphQLResponse{Data: map[string]interface{}{}}, nil
	}

	// 简化实现：返回第一个响应
	// 在实际实现中，应该根据 Federation 规则合并实体数据
	firstResponse := responses[0]
	return &federationtypes.GraphQLResponse{
		Data:   firstResponse.Data,
		Errors: firstResponse.Errors,
	}, nil
}

// ResolveEntityReferences 解析实体引用
func (e *Engine) ResolveEntityReferences(ctx context.Context, serviceName string, representations []federationtypes.RepresentationRequest) ([]interface{}, error) {
	e.logger.Debug("Resolving entity references", "service", serviceName, "count", len(representations))

	// 使用实体解析器批量解析
	return e.entityResolver.ResolveBatchEntities(ctx, serviceName, representations)
}

// BuildRepresentationQuery 构建实体表示查询
func (e *Engine) BuildRepresentationQuery(entity *federationtypes.FederatedEntity, representations []federationtypes.RepresentationRequest) (string, error) {
	// 使用 Federation 规划器构建查询
	return e.federationPlanner.BuildRepresentationQuery(entity, representations)
}

// GetFederatedEntities 获取当前注册的 Federation 实体
func (e *Engine) GetFederatedEntities() []federationtypes.FederatedEntity {
	// 这里应该从模式注册表中获取
	// 简化实现：返回空列表
	return []federationtypes.FederatedEntity{}
}

// ValidateEntityKey 验证实体键
func (e *Engine) ValidateEntityKey(entity *federationtypes.FederatedEntity, keyFields []string) error {
	// 检查键字段是否在实体中存在
	for _, keyField := range keyFields {
		found := false
		for _, field := range entity.Fields {
			if field.Name == keyField {
				found = true
				break
			}
		}

		if !found {
			return fmt.Errorf("key field %s not found in entity %s", keyField, entity.TypeName)
		}
	}

	return nil
}
