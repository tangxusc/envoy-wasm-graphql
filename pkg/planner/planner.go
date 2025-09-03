package planner

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/wundergraph/graphql-go-tools/v2/pkg/ast"

	"envoy-wasm-graphql-federation/pkg/errors"
	federationtypes "envoy-wasm-graphql-federation/pkg/types"
)

// Planner 实现查询规划器
type Planner struct {
	logger            federationtypes.Logger
	federationPlanner federationtypes.FederationPlanner
}

// NewPlanner 创建新的查询规划器
func NewPlanner(logger federationtypes.Logger) federationtypes.QueryPlanner {
	return &Planner{
		logger: logger,
		// 这里不创建 federationPlanner 防止循环依赖
		// federationPlanner: federation.NewFederatedPlanner(logger),
	}
}

// CreateExecutionPlan 创建执行计划
func (p *Planner) CreateExecutionPlan(ctx context.Context, query *federationtypes.ParsedQuery, services []federationtypes.ServiceConfig) (*federationtypes.ExecutionPlan, error) {
	if query == nil {
		return nil, errors.NewPlanningError("query is nil")
	}

	if len(services) == 0 {
		return nil, errors.NewPlanningError("no services available")
	}

	p.logger.Info("Creating execution plan",
		"operation", query.Operation,
		"services", len(services),
		"complexity", query.Complexity,
	)

	// 提取字段路径
	fieldPaths, err := p.extractFieldPaths(query)
	if err != nil {
		return nil, errors.NewPlanningError("failed to extract field paths: " + err.Error())
	}

	// 分析字段和服务映射
	fieldMappings := p.analyzeFieldMappings(fieldPaths, services)

	// 构建依赖关系图
	dependencies := p.buildDependencyGraph(fieldMappings)

	// 生成子查询
	subQueries, err := p.generateSubQueries(query, fieldMappings, services)
	if err != nil {
		return nil, errors.NewPlanningError("failed to generate sub-queries: " + err.Error())
	}

	// 确定合并策略
	mergeStrategy := p.determineMergeStrategy(subQueries)

	plan := &federationtypes.ExecutionPlan{
		SubQueries:    subQueries,
		Dependencies:  dependencies,
		MergeStrategy: mergeStrategy,
		Metadata: map[string]interface{}{
			"totalFields":    len(fieldPaths),
			"totalServices":  len(services),
			"createdAt":      time.Now(),
			"planComplexity": p.calculatePlanComplexity(subQueries),
		},
	}

	p.logger.Info("Execution plan created",
		"subQueries", len(subQueries),
		"dependencies", len(dependencies),
		"mergeStrategy", mergeStrategy,
	)

	return plan, nil
}

// OptimizePlan 优化执行计划
func (p *Planner) OptimizePlan(plan *federationtypes.ExecutionPlan) (*federationtypes.ExecutionPlan, error) {
	if plan == nil {
		return nil, errors.NewPlanningError("plan is nil")
	}

	p.logger.Debug("Optimizing execution plan", "subQueries", len(plan.SubQueries))

	optimizedPlan := &federationtypes.ExecutionPlan{
		SubQueries:    make([]federationtypes.SubQuery, len(plan.SubQueries)),
		Dependencies:  make(map[string][]string),
		MergeStrategy: plan.MergeStrategy,
		Metadata:      make(map[string]interface{}),
	}

	// 复制原始计划
	copy(optimizedPlan.SubQueries, plan.SubQueries)
	for k, v := range plan.Dependencies {
		optimizedPlan.Dependencies[k] = v
	}
	for k, v := range plan.Metadata {
		optimizedPlan.Metadata[k] = v
	}

	// 合并相同服务的查询
	optimizedPlan.SubQueries = p.mergeQueriesForSameService(optimizedPlan.SubQueries)

	// 优化查询顺序
	optimizedPlan.SubQueries = p.optimizeQueryOrder(optimizedPlan.SubQueries, optimizedPlan.Dependencies)

	// 批处理优化
	optimizedPlan.SubQueries = p.optimizeBatching(optimizedPlan.SubQueries)

	// 更新元数据
	optimizedPlan.Metadata["optimized"] = true
	optimizedPlan.Metadata["optimizedAt"] = time.Now()
	optimizedPlan.Metadata["originalSubQueries"] = len(plan.SubQueries)
	optimizedPlan.Metadata["optimizedSubQueries"] = len(optimizedPlan.SubQueries)

	p.logger.Debug("Plan optimization completed",
		"originalQueries", len(plan.SubQueries),
		"optimizedQueries", len(optimizedPlan.SubQueries),
	)

	return optimizedPlan, nil
}

// ValidatePlan 验证执行计划
func (p *Planner) ValidatePlan(plan *federationtypes.ExecutionPlan) error {
	if plan == nil {
		return errors.NewPlanningError("plan is nil")
	}

	// 检查基本有效性
	if len(plan.SubQueries) == 0 {
		return errors.NewPlanningError("plan has no sub-queries")
	}

	// 验证子查询
	for i, subQuery := range plan.SubQueries {
		if err := p.validateSubQuery(&subQuery, i); err != nil {
			return err
		}
	}

	// 验证依赖关系
	if err := p.validateDependencies(plan.Dependencies, plan.SubQueries); err != nil {
		return err
	}

	// 检查循环依赖
	if err := p.checkCircularDependencies(plan.Dependencies); err != nil {
		return err
	}

	p.logger.Debug("Plan validation passed")
	return nil
}

// extractFieldPaths 提取字段路径
func (p *Planner) extractFieldPaths(query *federationtypes.ParsedQuery) ([]federationtypes.FieldPath, error) {
	document, ok := query.AST.(*ast.Document)
	if !ok {
		return nil, fmt.Errorf("invalid AST document")
	}

	var fieldPaths []federationtypes.FieldPath

	// 遍历操作定义
	for i, _ := range document.OperationDefinitions {
		operation := document.OperationDefinitions[i]

		// 检查操作名称匹配
		if query.Operation != "" {
			operationName := document.OperationDefinitionNameString(i)
			if operationName != query.Operation {
				continue
			}
		}

		// 提取选择集字段
		paths := p.extractFieldsFromSelectionSet(document, operation.SelectionSet, []string{})
		fieldPaths = append(fieldPaths, paths...)
	}

	return fieldPaths, nil
}

// extractFieldsFromSelectionSet 从选择集提取字段
func (p *Planner) extractFieldsFromSelectionSet(document *ast.Document, selectionSet int, currentPath []string) []federationtypes.FieldPath {
	visited := make(map[int]bool)
	return p.extractFieldsFromSelectionSetWithVisited(document, selectionSet, currentPath, visited)
}

// extractFieldsFromSelectionSetWithVisited 从选择集提取字段（带访问跟踪）
func (p *Planner) extractFieldsFromSelectionSetWithVisited(document *ast.Document, selectionSet int, currentPath []string, visited map[int]bool) []federationtypes.FieldPath {
	var fieldPaths []federationtypes.FieldPath

	if selectionSet == -1 {
		return fieldPaths
	}

	// 检查是否已经访问过这个选择集，防止循环引用
	if visited[selectionSet] {
		return fieldPaths
	}

	// 标记为已访问
	visited[selectionSet] = true
	defer func() {
		delete(visited, selectionSet)
	}()

	selections := document.SelectionSets[selectionSet].SelectionRefs
	for _, selectionRef := range selections {
		selection := document.Selections[selectionRef]

		switch selection.Kind {
		case ast.SelectionKindField:
			field := document.Fields[selection.Ref]
			fieldName := document.FieldNameString(selection.Ref)

			newPath := append(currentPath, fieldName)
			fieldType := p.getFieldType(document, field)

			fieldPath := federationtypes.FieldPath{
				Path: newPath,
				Type: fieldType,
			}
			fieldPaths = append(fieldPaths, fieldPath)

			// 递归处理子字段
			if field.SelectionSet != -1 {
				subPaths := p.extractFieldsFromSelectionSetWithVisited(document, field.SelectionSet, newPath, visited)
				fieldPaths = append(fieldPaths, subPaths...)
			}

		case ast.SelectionKindInlineFragment:
			inlineFragment := document.InlineFragments[selection.Ref]
			if inlineFragment.SelectionSet != -1 {
				subPaths := p.extractFieldsFromSelectionSetWithVisited(document, inlineFragment.SelectionSet, currentPath, visited)
				fieldPaths = append(fieldPaths, subPaths...)
			}
		}
	}

	return fieldPaths
}

// analyzeFieldMappings 分析字段和服务映射
func (p *Planner) analyzeFieldMappings(fieldPaths []federationtypes.FieldPath, services []federationtypes.ServiceConfig) map[string][]string {
	fieldMappings := make(map[string][]string)

	for _, fieldPath := range fieldPaths {
		pathKey := strings.Join(fieldPath.Path, ".")

		// 简化映射逻辑：根据字段名称推断服务
		// 在实际实现中，这里应该基于联邦模式进行映射
		for _, service := range services {
			if p.fieldBelongsToService(fieldPath, service) {
				fieldMappings[pathKey] = append(fieldMappings[pathKey], service.Name)
			}
		}

		// 如果没有找到服务，分配给第一个服务（回退策略）
		if len(fieldMappings[pathKey]) == 0 && len(services) > 0 {
			fieldMappings[pathKey] = []string{services[0].Name}
		}
	}

	return fieldMappings
}

// fieldBelongsToService 判断字段是否属于服务（基于模式分析）
func (p *Planner) fieldBelongsToService(fieldPath federationtypes.FieldPath, service federationtypes.ServiceConfig) bool {
	if len(fieldPath.Path) == 0 {
		return false
	}

	rootField := fieldPath.Path[0]
	serviceName := strings.ToLower(service.Name)
	fieldName := strings.ToLower(rootField)

	p.logger.Debug("Checking field ownership", "field", rootField, "service", service.Name)

	// 1. 基于服务名称的简单匹配
	if strings.Contains(fieldName, serviceName) || strings.Contains(serviceName, fieldName) {
		return true
	}

	// 2. 基于预定义的字段映射
	fieldMappings := map[string][]string{
		"user":    {"users", "user", "profile", "account", "authentication"},
		"product": {"products", "product", "catalog", "inventory", "item"},
		"order":   {"orders", "order", "purchase", "transaction", "payment"},
		"review":  {"reviews", "review", "rating", "comment", "feedback"},
		"auth":    {"login", "logout", "register", "authenticate", "token"},
	}

	for serviceType, fields := range fieldMappings {
		if strings.Contains(serviceName, serviceType) {
			for _, mappedField := range fields {
				if strings.Contains(fieldName, mappedField) {
					return true
				}
			}
		}
	}

	// 3. 基于模式分析（如果有可用的模式信息）
	if service.Schema != "" {
		return p.fieldExistsInSchema(rootField, service.Schema)
	}

	// 4. 基于端点URL的推断
	if service.Endpoint != "" {
		return p.inferFromEndpoint(fieldName, service.Endpoint)
	}

	// 5. 默认情况：如果没有明确的映射，返回false
	return false
}

// fieldExistsInSchema 检查字段是否在模式中存在
func (p *Planner) fieldExistsInSchema(fieldName, schema string) bool {
	// 由于GraphQL AST API兼容性问题，这里简化处理
	// 使用简单的字符串匹配
	return strings.Contains(schema, fieldName)
}

// checkFieldInObjectType 检查对象类型中的字段
func (p *Planner) checkFieldInObjectType(document *ast.Document, typeRef int, fieldName string) bool {
	// 简化处理，返图false避免AST API兼容性问题
	return false
}

// checkFieldInInterfaceType 检查接口类型中的字段
func (p *Planner) checkFieldInInterfaceType(document *ast.Document, typeRef int, fieldName string) bool {
	// 简化处理，返图false避免AST API兼容性问题
	return false
}

// inferFromEndpoint 从端点URL推断字段归属
func (p *Planner) inferFromEndpoint(fieldName, endpoint string) bool {
	// 从端点URL中提取服务类型
	lowerEndpoint := strings.ToLower(endpoint)
	lowerField := strings.ToLower(fieldName)

	// 检查URL路径中是否包含相关关键词
	if strings.Contains(lowerEndpoint, lowerField) {
		return true
	}

	// 检查常见的服务模式
	urlPatterns := map[string][]string{
		"user":    {"/user", "/users", "/auth", "/account"},
		"product": {"/product", "/products", "/catalog", "/inventory"},
		"order":   {"/order", "/orders", "/purchase", "/payment"},
		"review":  {"/review", "/reviews", "/rating", "/comment"},
	}

	for serviceType, patterns := range urlPatterns {
		if strings.Contains(lowerField, serviceType) {
			for _, pattern := range patterns {
				if strings.Contains(lowerEndpoint, pattern) {
					return true
				}
			}
		}
	}

	return false
}

// mapFieldToService 将字段映射到服务
func (p *Planner) mapFieldToService(fieldName, serviceName string) bool {
	// 一些常见的映射规则
	switch serviceName {
	case "users", "user":
		return strings.HasPrefix(fieldName, "user") || fieldName == "me"
	case "products", "product":
		return strings.HasPrefix(fieldName, "product")
	case "orders", "order":
		return strings.HasPrefix(fieldName, "order")
	default:
		// 默认映射：字段名包含服务名
		return strings.Contains(fieldName, serviceName) || strings.Contains(serviceName, fieldName)
	}
}

// buildDependencyGraph 构建依赖关系图
func (p *Planner) buildDependencyGraph(fieldMappings map[string][]string) map[string][]string {
	dependencies := make(map[string][]string)

	// 基于联邦规范分析字段依赖
	for fieldPath, services := range fieldMappings {
		for _, service := range services {
			// 根据字段路径分析依赖
			serviceDeps := p.analyzeServiceDependencies(service, fieldPath, fieldMappings)
			if len(serviceDeps) > 0 {
				dependencies[service] = append(dependencies[service], serviceDeps...)
			}
		}
	}

	// 去重和清理依赖
	for service, deps := range dependencies {
		dependencies[service] = p.uniqueAndFilterDependencies(deps, service)
	}

	return dependencies
}

// findServiceDependencies 查找服务依赖
func (p *Planner) findServiceDependencies(service string, fieldMappings map[string][]string) []string {
	var deps []string
	serviceLower := strings.ToLower(service)

	// 基于联邦指令和业务逻辑推断依赖
	dependencyRules := map[string][]string{
		// 订单相关服务
		"order":    {"user", "users", "product", "products", "payment", "payments"},
		"orders":   {"user", "users", "product", "products", "payment", "payments"},
		"checkout": {"user", "users", "product", "products", "payment", "payments"},

		// 评价相关服务
		"review":  {"user", "users", "product", "products"},
		"reviews": {"user", "users", "product", "products"},
		"rating":  {"user", "users", "product", "products"},

		// 购物车相关服务
		"cart":     {"user", "users", "product", "products"},
		"wishlist": {"user", "users", "product", "products"},

		// 支付相关服务
		"payment":  {"user", "users", "order", "orders"},
		"payments": {"user", "users", "order", "orders"},
		"billing":  {"user", "users"},

		// 物流相关服务
		"shipping":    {"user", "users", "order", "orders"},
		"delivery":    {"user", "users", "order", "orders"},
		"fulfillment": {"order", "orders", "product", "products"},

		// 通知相关服务
		"notification": {"user", "users"},
		"email":        {"user", "users"},
		"sms":          {"user", "users"},

		// 分析相关服务
		"analytics": {"user", "users", "product", "products", "order", "orders"},
		"reporting": {"user", "users", "product", "products", "order", "orders"},

		// 库存相关服务
		"inventory": {"product", "products"},
		"catalog":   {"product", "products"},
	}

	// 查找匹配的依赖规则
	for servicePattern, dependencies := range dependencyRules {
		if strings.Contains(serviceLower, servicePattern) {
			// 验证依赖服务是否存在于字段映射中
			for _, dep := range dependencies {
				if p.serviceExistsInMappings(dep, fieldMappings) && dep != service {
					deps = append(deps, dep)
				}
			}
			break
		}
	}

	// 基于字段名称推断依赖
	additionalDeps := p.inferDependenciesFromFieldNames(service, fieldMappings)
	deps = append(deps, additionalDeps...)

	return p.uniqueAndFilterDependencies(deps, service)
}

// serviceExistsInMappings 检查服务是否存在于字段映射中
func (p *Planner) serviceExistsInMappings(serviceName string, fieldMappings map[string][]string) bool {
	for _, services := range fieldMappings {
		for _, service := range services {
			if strings.EqualFold(service, serviceName) {
				return true
			}
		}
	}
	return false
}

// inferDependenciesFromFieldNames 从字段名称推断依赖
func (p *Planner) inferDependenciesFromFieldNames(service string, fieldMappings map[string][]string) []string {
	var deps []string

	// 遍历字段映射，查找可能的依赖
	for fieldPath, services := range fieldMappings {
		// 检查当前服务是否处理这个字段
		serviceHandlesField := false
		for _, s := range services {
			if s == service {
				serviceHandlesField = true
				break
			}
		}

		if serviceHandlesField {
			// 分析字段路径，推断可能的依赖
			fieldParts := strings.Split(fieldPath, ".")
			for _, part := range fieldParts {
				// 检查是否有其他服务处理相关字段
				relatedDep := p.findRelatedServiceForField(part, fieldMappings, service)
				if relatedDep != "" {
					deps = append(deps, relatedDep)
				}
			}
		}
	}

	return deps
}

// findRelatedServiceForField 为字段查找相关服务
func (p *Planner) findRelatedServiceForField(fieldName string, fieldMappings map[string][]string, excludeService string) string {
	// 字段名到服务的映射规则
	fieldToServiceMap := map[string][]string{
		"user":     {"user", "users", "auth", "account"},
		"account":  {"user", "users", "auth", "account"},
		"profile":  {"user", "users", "auth", "account"},
		"product":  {"product", "products", "catalog", "inventory"},
		"item":     {"product", "products", "catalog", "inventory"},
		"catalog":  {"product", "products", "catalog"},
		"order":    {"order", "orders", "checkout"},
		"purchase": {"order", "orders", "checkout"},
		"payment":  {"payment", "payments", "billing"},
		"billing":  {"payment", "payments", "billing"},
		"review":   {"review", "reviews", "rating"},
		"rating":   {"review", "reviews", "rating"},
		"comment":  {"review", "reviews", "rating"},
	}

	fieldLower := strings.ToLower(fieldName)
	for pattern, possibleServices := range fieldToServiceMap {
		if strings.Contains(fieldLower, pattern) {
			// 查找在字段映射中存在的服务
			for _, possibleService := range possibleServices {
				if possibleService != excludeService && p.serviceExistsInMappings(possibleService, fieldMappings) {
					return possibleService
				}
			}
		}
	}

	return ""
}

// generateSubQueries 生成子查询
func (p *Planner) generateSubQueries(query *federationtypes.ParsedQuery, fieldMappings map[string][]string, services []federationtypes.ServiceConfig) ([]federationtypes.SubQuery, error) {
	serviceQueries := make(map[string][]string)

	// 按服务分组字段
	for fieldPath, fieldServices := range fieldMappings {
		for _, serviceName := range fieldServices {
			serviceQueries[serviceName] = append(serviceQueries[serviceName], fieldPath)
		}
	}

	var subQueries []federationtypes.SubQuery

	// 为每个服务生成子查询
	for serviceName, fields := range serviceQueries {
		service := p.findServiceByName(serviceName, services)
		if service == nil {
			continue
		}

		// 设置超时值，优先使用服务配置，否则使用默认值
		timeout := service.Timeout
		if timeout <= 0 {
			timeout = 30 * time.Second // 默认超时时间
		}

		subQuery := federationtypes.SubQuery{
			ServiceName: serviceName,
			Query:       p.buildSubQuery(fields, query),
			Variables:   query.Variables,
			Path:        []string{serviceName},
			Timeout:     timeout,
			RetryCount:  3, // 默认重试次数
		}

		subQueries = append(subQueries, subQuery)
	}

	return subQueries, nil
}

// buildSubQuery 构建子查询（基于AST）
func (p *Planner) buildSubQuery(fields []string, originalQuery *federationtypes.ParsedQuery) string {
	if len(fields) == 0 {
		return ""
	}

	p.logger.Debug("Building sub-query from AST", "fields", len(fields))

	// 如果有原始AST，尝试基于AST重构子查询
	if originalQuery.AST != nil {
		return p.buildSubQueryFromAST(fields, originalQuery)
	}

	// 否则使用简化的字符串构建
	return p.buildSubQuerySimple(fields)
}

// buildSubQueryFromAST 基于AST构建子查询
func (p *Planner) buildSubQueryFromAST(fields []string, originalQuery *federationtypes.ParsedQuery) string {
	_, ok := originalQuery.AST.(*ast.Document)
	if !ok {
		p.logger.Warn("AST type assertion failed, falling back to simple query building")
		return p.buildSubQuerySimple(fields)
	}

	// 由于GraphQL AST API兼容性问题，直接使用简化构建
	return p.buildSubQuerySimple(fields)
}

// filterSelectionSet 过滤选择集，只保留指定字段
func (p *Planner) filterSelectionSet(document *ast.Document, selectionSetRef int, targetFields []string) string {
	if selectionSetRef == -1 {
		return ""
	}

	var filteredFields []string
	targetFieldsMap := make(map[string]bool)
	for _, field := range targetFields {
		// 提取根字段名
		parts := strings.Split(field, ".")
		if len(parts) > 0 {
			targetFieldsMap[parts[0]] = true
		}
	}

	// 遍历选择集
	selectionSet := document.SelectionSets[selectionSetRef]
	for _, selectionRef := range selectionSet.SelectionRefs {
		selection := document.Selections[selectionRef]

		if selection.Kind == ast.SelectionKindField {
			fieldName := document.FieldNameString(selection.Ref)

			// 检查是否是目标字段
			if targetFieldsMap[fieldName] {
				// 构建字段的完整选择
				fieldSelection := p.buildFieldSelection(document, selection.Ref, targetFields, fieldName)
				if fieldSelection != "" {
					filteredFields = append(filteredFields, fieldSelection)
				}
			}
		} else if selection.Kind == ast.SelectionKindFragmentSpread {
			// 处理片段展开
			fragmentName := document.FragmentSpreadNameString(selection.Ref)
			p.logger.Debug("Processing fragment spread", "fragment", fragmentName)
			// 简化处理，忽略片段
		}
	}

	return strings.Join(filteredFields, " ")
}

// buildFieldSelection 构建字段选择
func (p *Planner) buildFieldSelection(document *ast.Document, fieldRef int, targetFields []string, currentFieldName string) string {
	fieldName := document.FieldNameString(fieldRef)

	// 构建字段的基本部分
	fieldStr := fieldName

	// 简化处理，不处理参数和子字段
	return fieldStr
}

// getSubFieldsForField 获取指定字段的子字段
func (p *Planner) getSubFieldsForField(targetFields []string, parentField string) []string {
	var subFields []string
	prefix := parentField + "."

	for _, field := range targetFields {
		if strings.HasPrefix(field, prefix) {
			// 移除前缀，得到子字段路径
			subField := strings.TrimPrefix(field, prefix)
			subFields = append(subFields, subField)
		}
	}

	return subFields
}

// getAllSubFields 获取所有子字段
func (p *Planner) getAllSubFields(document *ast.Document, selectionSetRef int) string {
	if selectionSetRef == -1 {
		return ""
	}

	var fields []string
	selectionSet := document.SelectionSets[selectionSetRef]

	for _, selectionRef := range selectionSet.SelectionRefs {
		selection := document.Selections[selectionRef]

		if selection.Kind == ast.SelectionKindField {
			fieldName := document.FieldNameString(selection.Ref)
			fields = append(fields, fieldName)
		}
	}

	return strings.Join(fields, " ")
}

// buildSubQuerySimple 简化的子查询构建
func (p *Planner) buildSubQuerySimple(fields []string) string {
	// 提取根字段
	rootFields := make(map[string]bool)
	for _, field := range fields {
		parts := strings.Split(field, ".")
		if len(parts) > 0 {
			rootFields[parts[0]] = true
		}
	}

	// 构建简化查询
	var rootFieldsList []string
	for field := range rootFields {
		rootFieldsList = append(rootFieldsList, field)
	}

	if len(rootFieldsList) == 0 {
		return ""
	}

	query := fmt.Sprintf("query { %s }", strings.Join(rootFieldsList, " "))
	return query
}

// determineMergeStrategy 确定合并策略
func (p *Planner) determineMergeStrategy(subQueries []federationtypes.SubQuery) federationtypes.MergeStrategy {
	// 简化的策略选择
	if len(subQueries) <= 1 {
		return federationtypes.MergeStrategyShallow
	}

	// 检查是否有复杂的嵌套结构
	hasNestedFields := false
	for _, subQuery := range subQueries {
		if strings.Contains(subQuery.Query, "{") && strings.Count(subQuery.Query, "{") > 1 {
			hasNestedFields = true
			break
		}
	}

	if hasNestedFields {
		return federationtypes.MergeStrategyDeep
	}

	return federationtypes.MergeStrategyShallow
}

// 优化相关方法

// mergeQueriesForSameService 合并相同服务的查询
func (p *Planner) mergeQueriesForSameService(subQueries []federationtypes.SubQuery) []federationtypes.SubQuery {
	serviceGroups := make(map[string][]federationtypes.SubQuery)

	// 按服务分组
	for _, subQuery := range subQueries {
		serviceGroups[subQuery.ServiceName] = append(serviceGroups[subQuery.ServiceName], subQuery)
	}

	var optimized []federationtypes.SubQuery

	// 合并每个服务的查询
	for _, queries := range serviceGroups {
		if len(queries) == 1 {
			optimized = append(optimized, queries[0])
		} else {
			merged := p.mergeQueries(queries)
			optimized = append(optimized, merged)
		}
	}

	return optimized
}

// mergeQueries 合并查询
func (p *Planner) mergeQueries(queries []federationtypes.SubQuery) federationtypes.SubQuery {
	if len(queries) == 0 {
		return federationtypes.SubQuery{}
	}

	if len(queries) == 1 {
		return queries[0]
	}

	// 使用第一个查询作为基础
	merged := queries[0]

	// 合并变量
	allVariables := make(map[string]interface{})
	for _, query := range queries {
		for k, v := range query.Variables {
			allVariables[k] = v
		}
	}
	merged.Variables = allVariables

	// 合并查询字符串
	merged.Query = p.mergeQueryStrings(queries)

	// 合并路径
	merged.Path = p.mergeQueryPaths(queries)

	// 设置最大超时时间
	merged.Timeout = p.getMaxTimeout(queries)

	return merged
}

// mergeQueryStrings 合并查询字符串
func (p *Planner) mergeQueryStrings(queries []federationtypes.SubQuery) string {
	if len(queries) == 0 {
		return ""
	}

	// 提取所有查询中的字段
	fields := make(map[string]bool)
	var queryType string

	for _, query := range queries {
		// 简化解析：提取大括号内的内容
		queryContent := p.extractQueryContent(query.Query)
		if queryContent != "" {
			queryFields := p.parseQueryFields(queryContent)
			for _, field := range queryFields {
				fields[field] = true
			}
		}

		// 确定查询类型
		if queryType == "" {
			queryType = p.extractQueryType(query.Query)
		}
	}

	// 构建合并后的查询
	if len(fields) == 0 {
		return queries[0].Query
	}

	var fieldList []string
	for field := range fields {
		fieldList = append(fieldList, field)
	}

	if queryType == "" {
		queryType = "query"
	}

	return fmt.Sprintf("%s { %s }", queryType, strings.Join(fieldList, " "))
}

// extractQueryContent 提取查询内容
func (p *Planner) extractQueryContent(query string) string {
	start := strings.Index(query, "{")
	end := strings.LastIndex(query, "}")

	if start == -1 || end == -1 || start >= end {
		return ""
	}

	return strings.TrimSpace(query[start+1 : end])
}

// parseQueryFields 解析查询字段
func (p *Planner) parseQueryFields(content string) []string {
	var fields []string

	// 简化解析：按空格和换行分割
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" && !strings.HasPrefix(line, "#") {
			// 提取字段名（去除参数和子字段）
			fieldName := p.extractFieldName(line)
			if fieldName != "" {
				fields = append(fields, fieldName)
			}
		}
	}

	return fields
}

// extractFieldName 提取字段名
func (p *Planner) extractFieldName(line string) string {
	// 移除参数
	if idx := strings.Index(line, "("); idx != -1 {
		line = line[:idx]
	}

	// 移除子字段
	if idx := strings.Index(line, "{"); idx != -1 {
		line = line[:idx]
	}

	return strings.TrimSpace(line)
}

// extractQueryType 提取查询类型
func (p *Planner) extractQueryType(query string) string {
	query = strings.TrimSpace(query)

	if strings.HasPrefix(query, "query") {
		return "query"
	} else if strings.HasPrefix(query, "mutation") {
		return "mutation"
	} else if strings.HasPrefix(query, "subscription") {
		return "subscription"
	}

	return "query" // 默认为query
}

// mergeQueryPaths 合并查询路径
func (p *Planner) mergeQueryPaths(queries []federationtypes.SubQuery) []string {
	pathsMap := make(map[string]bool)

	for _, query := range queries {
		for _, path := range query.Path {
			pathsMap[path] = true
		}
	}

	var paths []string
	for path := range pathsMap {
		paths = append(paths, path)
	}

	return paths
}

// getMaxTimeout 获取最大超时时间
func (p *Planner) getMaxTimeout(queries []federationtypes.SubQuery) time.Duration {
	var maxTimeout time.Duration

	for _, query := range queries {
		if query.Timeout > maxTimeout {
			maxTimeout = query.Timeout
		}
	}

	return maxTimeout
}

// optimizeQueryOrder 优化查询顺序
func (p *Planner) optimizeQueryOrder(subQueries []federationtypes.SubQuery, dependencies map[string][]string) []federationtypes.SubQuery {
	// 基于依赖关系进行拓扑排序
	ordered := make([]federationtypes.SubQuery, 0, len(subQueries))
	visited := make(map[string]bool)
	visiting := make(map[string]bool)

	var visit func(serviceName string) bool
	visit = func(serviceName string) bool {
		if visiting[serviceName] {
			// 检测到循环依赖
			p.logger.Warn("Circular dependency detected", "service", serviceName)
			return false
		}

		if visited[serviceName] {
			return true
		}

		visiting[serviceName] = true

		// 先处理依赖
		for _, dep := range dependencies[serviceName] {
			if !visit(dep) {
				return false
			}
		}

		visiting[serviceName] = false
		visited[serviceName] = true

		// 添加到结果
		for _, subQuery := range subQueries {
			if subQuery.ServiceName == serviceName {
				ordered = append(ordered, subQuery)
				break
			}
		}

		return true
	}

	// 访问所有服务
	for _, subQuery := range subQueries {
		visit(subQuery.ServiceName)
	}

	return ordered
}

// optimizeBatching 批处理优化
func (p *Planner) optimizeBatching(subQueries []federationtypes.SubQuery) []federationtypes.SubQuery {
	if len(subQueries) <= 1 {
		return subQueries
	}

	// 按服务名分组
	serviceGroups := make(map[string][]federationtypes.SubQuery)
	for _, subQuery := range subQueries {
		serviceGroups[subQuery.ServiceName] = append(serviceGroups[subQuery.ServiceName], subQuery)
	}

	var optimized []federationtypes.SubQuery

	// 对每个服务组进行批处理优化
	for serviceName, queries := range serviceGroups {
		if len(queries) == 1 {
			// 单个查询，直接添加
			optimized = append(optimized, queries[0])
		} else {
			// 多个查询，尝试批处理合并
			batchedQueries := p.batchQueriesForService(serviceName, queries)
			optimized = append(optimized, batchedQueries...)
		}
	}

	p.logger.Debug("Batching optimization completed",
		"original", len(subQueries),
		"optimized", len(optimized))

	return optimized
}

// batchQueriesForService 为特定服务批处理查询
func (p *Planner) batchQueriesForService(serviceName string, queries []federationtypes.SubQuery) []federationtypes.SubQuery {
	// 分析查询相似性
	groups := p.groupSimilarQueries(queries)

	var result []federationtypes.SubQuery

	for _, group := range groups {
		if len(group) == 1 {
			result = append(result, group[0])
		} else if p.canBatchQueries(group) {
			// 合并可批处理的查询
			batched := p.createBatchedQuery(serviceName, group)
			result = append(result, batched)
		} else {
			// 不能批处理，保持原样
			result = append(result, group...)
		}
	}

	return result
}

// groupSimilarQueries 分组相似的查询
func (p *Planner) groupSimilarQueries(queries []federationtypes.SubQuery) [][]federationtypes.SubQuery {
	var groups [][]federationtypes.SubQuery

	for _, query := range queries {
		placed := false

		// 尝试放入现有组
		for i, group := range groups {
			if len(group) > 0 && p.areQueriesSimilar(query, group[0]) {
				groups[i] = append(groups[i], query)
				placed = true
				break
			}
		}

		// 如果没有找到相似的组，创建新组
		if !placed {
			groups = append(groups, []federationtypes.SubQuery{query})
		}
	}

	return groups
}

// areQueriesSimilar 检查两个查询是否相似
func (p *Planner) areQueriesSimilar(q1, q2 federationtypes.SubQuery) bool {
	// 检查服务名
	if q1.ServiceName != q2.ServiceName {
		return false
	}

	// 检查查询类型
	type1 := p.extractQueryType(q1.Query)
	type2 := p.extractQueryType(q2.Query)
	if type1 != type2 {
		return false
	}

	// 检查查询复杂度
	complexity1 := p.calculateQueryComplexity(q1.Query)
	complexity2 := p.calculateQueryComplexity(q2.Query)

	// 相似的复杂度阈值
	maxComplexity := 10
	if complexity1 > maxComplexity || complexity2 > maxComplexity {
		return false
	}

	// 检查变量相似性
	return p.areVariablesSimilar(q1.Variables, q2.Variables)
}

// calculateQueryComplexity 计算查询复杂度
func (p *Planner) calculateQueryComplexity(query string) int {
	complexity := 0
	complexity += strings.Count(query, "{") * 2 // 字段嵌套
	complexity += strings.Count(query, "(")     // 参数
	complexity += strings.Count(query, "[]")    // 数组
	return complexity
}

// areVariablesSimilar 检查变量是否相似
func (p *Planner) areVariablesSimilar(vars1, vars2 map[string]interface{}) bool {
	// 如果变量数量相差太大，认为不相似
	len1, len2 := len(vars1), len(vars2)
	if len1 == 0 && len2 == 0 {
		return true
	}

	maxLen := len1
	if len2 > maxLen {
		maxLen = len2
	}

	minLen := len1
	if len2 < minLen {
		minLen = len2
	}

	// 如果变量数量相差超过50%，认为不相似
	if maxLen > 0 && float64(minLen)/float64(maxLen) < 0.5 {
		return false
	}

	return true
}

// canBatchQueries 检查是否可以批处理查询
func (p *Planner) canBatchQueries(queries []federationtypes.SubQuery) bool {
	if len(queries) <= 1 {
		return false
	}

	// 检查是否都是相同的服务
	firstService := queries[0].ServiceName
	for _, query := range queries[1:] {
		if query.ServiceName != firstService {
			return false
		}
	}

	// 检查是否都是查询类型（不是mutation）
	for _, query := range queries {
		queryType := p.extractQueryType(query.Query)
		if queryType != "query" {
			return false
		}
	}

	// 检查批处理大小限制
	maxBatchSize := 5
	if len(queries) > maxBatchSize {
		return false
	}

	return true
}

// createBatchedQuery 创建批处理查询
func (p *Planner) createBatchedQuery(serviceName string, queries []federationtypes.SubQuery) federationtypes.SubQuery {
	// 合并所有查询的字段
	allFields := make(map[string]bool)
	allVariables := make(map[string]interface{})
	allPaths := make(map[string]bool)

	var maxTimeout time.Duration
	maxRetryCount := 0

	for _, query := range queries {
		// 提取字段
		queryContent := p.extractQueryContent(query.Query)
		fields := p.parseQueryFields(queryContent)
		for _, field := range fields {
			allFields[field] = true
		}

		// 合并变量
		for k, v := range query.Variables {
			allVariables[k] = v
		}

		// 合并路径
		for _, path := range query.Path {
			allPaths[path] = true
		}

		// 获取最大超时和重试次数
		if query.Timeout > maxTimeout {
			maxTimeout = query.Timeout
		}
		if query.RetryCount > maxRetryCount {
			maxRetryCount = query.RetryCount
		}
	}

	// 构建批处理查询
	var fieldList []string
	for field := range allFields {
		fieldList = append(fieldList, field)
	}

	var pathList []string
	for path := range allPaths {
		pathList = append(pathList, path)
	}

	batchedQuery := fmt.Sprintf("query { %s }", strings.Join(fieldList, " "))

	return federationtypes.SubQuery{
		ServiceName: serviceName,
		Query:       batchedQuery,
		Variables:   allVariables,
		Path:        pathList,
		Timeout:     maxTimeout,
		RetryCount:  maxRetryCount,
	}
}

// 验证相关方法

// validateSubQuery 验证子查询
func (p *Planner) validateSubQuery(subQuery *federationtypes.SubQuery, index int) error {
	if subQuery.ServiceName == "" {
		return errors.NewPlanningError(fmt.Sprintf("sub-query %d has empty service name", index))
	}

	if subQuery.Query == "" {
		return errors.NewPlanningError(fmt.Sprintf("sub-query %d has empty query", index))
	}

	if subQuery.Timeout <= 0 {
		return errors.NewPlanningError(fmt.Sprintf("sub-query %d has invalid timeout", index))
	}

	return nil
}

// validateDependencies 验证依赖关系
func (p *Planner) validateDependencies(dependencies map[string][]string, subQueries []federationtypes.SubQuery) error {
	// 收集所有服务名称
	serviceNames := make(map[string]bool)
	for _, subQuery := range subQueries {
		serviceNames[subQuery.ServiceName] = true
	}

	// 检查依赖的服务是否存在
	for service, deps := range dependencies {
		for _, dep := range deps {
			if !serviceNames[dep] {
				return errors.NewPlanningError(fmt.Sprintf("service %s depends on non-existent service %s", service, dep))
			}
		}
	}

	return nil
}

// checkCircularDependencies 检查循环依赖
func (p *Planner) checkCircularDependencies(dependencies map[string][]string) error {
	visited := make(map[string]bool)
	visiting := make(map[string]bool)

	var visit func(service string) error
	visit = func(service string) error {
		if visiting[service] {
			return errors.NewPlanningError(fmt.Sprintf("circular dependency detected involving service %s", service))
		}

		if visited[service] {
			return nil
		}

		visiting[service] = true

		for _, dep := range dependencies[service] {
			if err := visit(dep); err != nil {
				return err
			}
		}

		visiting[service] = false
		visited[service] = true

		return nil
	}

	for service := range dependencies {
		if err := visit(service); err != nil {
			return err
		}
	}

	return nil
}

// 辅助方法

// getFieldType 获取字段类型
func (p *Planner) getFieldType(document *ast.Document, field ast.Field) string {
	// 由于GraphQL AST API兼容性问题，返回默认类型
	return "String"
}

// extractTypeFromAST 从 AST 中提取类型
func (p *Planner) extractTypeFromAST(document *ast.Document, typeRef int) string {
	if typeRef == -1 || typeRef >= len(document.Types) {
		return "String"
	}

	typeNode := document.Types[typeRef]

	switch typeNode.TypeKind {
	case ast.TypeKindNamed:
		return document.TypeNameString(typeRef)

	case ast.TypeKindNonNull:
		innerType := p.extractTypeFromAST(document, typeNode.OfType)
		return innerType + "!"

	case ast.TypeKindList:
		innerType := p.extractTypeFromAST(document, typeNode.OfType)
		return "[" + innerType + "]"

	default:
		return "String"
	}
}

// findServiceByName 根据名称查找服务
func (p *Planner) findServiceByName(name string, services []federationtypes.ServiceConfig) *federationtypes.ServiceConfig {
	for _, service := range services {
		if service.Name == name {
			return &service
		}
	}
	return nil
}

// calculatePlanComplexity 计算计划复杂度
func (p *Planner) calculatePlanComplexity(subQueries []federationtypes.SubQuery) int {
	complexity := 0
	for _, subQuery := range subQueries {
		// 简化的复杂度计算
		complexity += strings.Count(subQuery.Query, "{") + len(subQuery.Variables)
	}
	return complexity
}

// analyzeServiceDependencies 分析单个服务的依赖
func (p *Planner) analyzeServiceDependencies(service, fieldPath string, fieldMappings map[string][]string) []string {
	var dependencies []string

	// 分析字段路径，找出可能的依赖
	pathParts := strings.Split(fieldPath, ".")

	// 检查联邦关键字来推断依赖
	for _, part := range pathParts {
		if dep := p.inferDependencyFromField(service, part, fieldMappings); dep != "" && dep != service {
			dependencies = append(dependencies, dep)
		}
	}

	// 基于常见的业务逻辑推断依赖
	businessDeps := p.getBusinessLogicDependencies(service)
	dependencies = append(dependencies, businessDeps...)

	return dependencies
}

// inferDependencyFromField 从字段名推断依赖
func (p *Planner) inferDependencyFromField(service, fieldName string, fieldMappings map[string][]string) string {
	// 检查是否有其他服务也处理相同的字段
	for path, services := range fieldMappings {
		if strings.Contains(path, fieldName) {
			for _, s := range services {
				if s != service {
					// 检查是否是基础服务（通常是用户或产品服务）
					if p.isBaseService(s) {
						return s
					}
				}
			}
		}
	}

	return ""
}

// isBaseService 检查是否为基础服务
func (p *Planner) isBaseService(service string) bool {
	baseServices := []string{"users", "user", "accounts", "auth", "products", "product", "catalog"}
	serviceLower := strings.ToLower(service)

	for _, baseService := range baseServices {
		if strings.Contains(serviceLower, baseService) {
			return true
		}
	}

	return false
}

// getBusinessLogicDependencies 获取业务逻辑依赖
func (p *Planner) getBusinessLogicDependencies(service string) []string {
	var dependencies []string
	serviceLower := strings.ToLower(service)

	// 定义常见的业务依赖关系
	dependencyMap := map[string][]string{
		"order":        {"user", "product", "payment"},
		"orders":       {"users", "products", "payments"},
		"review":       {"user", "product"},
		"reviews":      {"users", "products"},
		"cart":         {"user", "product"},
		"shopping":     {"user", "product"},
		"checkout":     {"user", "product", "payment"},
		"payment":      {"user", "order"},
		"payments":     {"users", "orders"},
		"shipping":     {"user", "order"},
		"notification": {"user"},
		"analytics":    {"user", "product", "order"},
	}

	for serviceType, deps := range dependencyMap {
		if strings.Contains(serviceLower, serviceType) {
			dependencies = append(dependencies, deps...)
			break
		}
	}

	return dependencies
}

// uniqueAndFilterDependencies 去重和过滤依赖
func (p *Planner) uniqueAndFilterDependencies(deps []string, service string) []string {
	unique := make(map[string]bool)
	var result []string

	for _, dep := range deps {
		// 过滤掉自身依赖
		if dep == service {
			continue
		}

		// 去重
		if !unique[dep] {
			unique[dep] = true
			result = append(result, dep)
		}
	}

	return result
}

// Federation 指令相关的查询规划功能

// CreateFederationExecutionPlan 基于 Federation 指令创建执行计划
func (p *Planner) CreateFederationExecutionPlan(ctx context.Context, query *federationtypes.ParsedQuery, entities []federationtypes.FederatedEntity) (*federationtypes.ExecutionPlan, error) {
	if query == nil {
		return nil, errors.NewPlanningError("query is nil")
	}

	if len(entities) == 0 {
		return nil, errors.NewPlanningError("no federated entities provided")
	}

	p.logger.Info("Creating Federation execution plan",
		"operation", query.Operation,
		"entities", len(entities),
	)

	// 分析查询需要的实体
	requiredEntities, err := p.analyzeRequiredEntities(query, entities)
	if err != nil {
		return nil, fmt.Errorf("failed to analyze required entities: %w", err)
	}

	// 构建实体解析策略
	entityResolutions, err := p.buildEntityResolutions(requiredEntities)
	if err != nil {
		return nil, fmt.Errorf("failed to build entity resolutions: %w", err)
	}

	// 转换为标准执行计划
	subQueries := p.convertEntityResolutionsToSubQueries(entityResolutions)

	// 分析依赖关系
	dependencies := p.buildEntityDependencies(requiredEntities)

	plan := &federationtypes.ExecutionPlan{
		SubQueries:    subQueries,
		Dependencies:  dependencies,
		MergeStrategy: federationtypes.MergeStrategyDeep,
		Metadata: map[string]interface{}{
			"federationPlan": true,
			"entityCount":    len(requiredEntities),
			"createdAt":      time.Now(),
		},
	}

	return plan, nil
}

// analyzeRequiredEntities 分析查询需要的实体
func (p *Planner) analyzeRequiredEntities(query *federationtypes.ParsedQuery, entities []federationtypes.FederatedEntity) ([]federationtypes.FederatedEntity, error) {
	// 简化实现：返回所有实体
	// 在实际实现中，应该根据查询的字段来确定需要哪些实体
	return entities, nil
}

// buildEntityResolutions 构建实体解析策略
func (p *Planner) buildEntityResolutions(entities []federationtypes.FederatedEntity) ([]federationtypes.EntityResolution, error) {
	var resolutions []federationtypes.EntityResolution

	for _, entity := range entities {
		resolution := federationtypes.EntityResolution{
			TypeName:    entity.TypeName,
			ServiceName: entity.ServiceName,
			KeyFields:   p.extractEntityKeyFields(entity),
			Query:       p.buildEntityQuery(entity),
		}
		resolutions = append(resolutions, resolution)
	}

	return resolutions, nil
}

// extractEntityKeyFields 提取实体键字段
func (p *Planner) extractEntityKeyFields(entity federationtypes.FederatedEntity) []string {
	var keyFields []string

	for _, key := range entity.Directives.Keys {
		fields := strings.Fields(key.Fields)
		keyFields = append(keyFields, fields...)
	}

	// 去重
	seen := make(map[string]bool)
	var unique []string
	for _, field := range keyFields {
		if !seen[field] {
			seen[field] = true
			unique = append(unique, field)
		}
	}

	return unique
}

// buildEntityQuery 构建实体查询
func (p *Planner) buildEntityQuery(entity federationtypes.FederatedEntity) string {
	var fields []string

	for _, field := range entity.Fields {
		// 跳过外部字段（除非是键字段）
		if field.Directives.External != nil && !p.isEntityKeyField(entity, field.Name) {
			continue
		}
		fields = append(fields, field.Name)
	}

	return fmt.Sprintf("{ %s }", strings.Join(fields, " "))
}

// isEntityKeyField 检查字段是否是实体的键字段
func (p *Planner) isEntityKeyField(entity federationtypes.FederatedEntity, fieldName string) bool {
	for _, key := range entity.Directives.Keys {
		fields := strings.Fields(key.Fields)
		for _, field := range fields {
			if field == fieldName {
				return true
			}
		}
	}
	return false
}

// convertEntityResolutionsToSubQueries 将实体解析转换为子查询
func (p *Planner) convertEntityResolutionsToSubQueries(resolutions []federationtypes.EntityResolution) []federationtypes.SubQuery {
	var subQueries []federationtypes.SubQuery

	for _, resolution := range resolutions {
		subQuery := federationtypes.SubQuery{
			ServiceName: resolution.ServiceName,
			Query:       resolution.Query,
			Path:        []string{resolution.TypeName},
			Timeout:     30000000000, // 30秒（纳秒）
		}
		subQueries = append(subQueries, subQuery)
	}

	return subQueries
}

// buildEntityDependencies 构建实体依赖关系
func (p *Planner) buildEntityDependencies(entities []federationtypes.FederatedEntity) map[string][]string {
	dependencies := make(map[string][]string)

	for _, entity := range entities {
		serviceName := entity.ServiceName
		var deps []string

		// 分析字段依赖
		for _, field := range entity.Fields {
			if field.Directives.Requires != nil {
				// 找到提供必需字段的服务
				requiredFields := strings.Fields(field.Directives.Requires.Fields)
				for _, requiredField := range requiredFields {
					provider := p.findFieldProviderService(entity.TypeName, requiredField, entities)
					if provider != "" && provider != serviceName {
						deps = append(deps, provider)
					}
				}
			}
		}

		// 去重
		uniqueDeps := p.uniqueAndFilterDependencies(deps, serviceName)
		if len(uniqueDeps) > 0 {
			dependencies[serviceName] = uniqueDeps
		}
	}

	return dependencies
}

// findFieldProviderService 查找提供指定字段的服务
func (p *Planner) findFieldProviderService(typeName, fieldName string, entities []federationtypes.FederatedEntity) string {
	for _, entity := range entities {
		if entity.TypeName == typeName {
			for _, field := range entity.Fields {
				if field.Name == fieldName && field.Directives.External == nil {
					return entity.ServiceName
				}
			}
		}
	}
	return ""
}
