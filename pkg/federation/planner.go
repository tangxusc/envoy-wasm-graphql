package federation

import (
	"fmt"
	"sort"
	"strings"

	"envoy-wasm-graphql-federation/pkg/errors"
	federationtypes "envoy-wasm-graphql-federation/pkg/types"
)

// FederatedPlanner 实现 Federation 查询规划器
type FederatedPlanner struct {
	logger          federationtypes.Logger
	directiveParser federationtypes.FederationDirectiveParser
}

// NewFederatedPlanner 创建新的联邦规划器
func NewFederatedPlanner(logger federationtypes.Logger) federationtypes.FederationPlanner {
	return &FederatedPlanner{
		logger:          logger,
		directiveParser: NewDirectiveParser(logger),
	}
}

// PlanEntityResolution 规划实体解析
func (p *FederatedPlanner) PlanEntityResolution(entities []federationtypes.FederatedEntity, query *federationtypes.ParsedQuery) (*federationtypes.FederationPlan, error) {
	if len(entities) == 0 {
		return nil, errors.NewPlanningError("no entities provided")
	}

	if query == nil {
		return nil, errors.NewPlanningError("query cannot be nil")
	}

	p.logger.Debug("Planning entity resolution", "entityCount", len(entities))

	plan := &federationtypes.FederationPlan{
		Entities:         []federationtypes.EntityResolution{},
		Representations:  []federationtypes.RepresentationRequest{},
		RequiredServices: []string{},
		DependencyOrder:  []string{},
	}

	// 分析查询中需要的实体
	requiredEntities, err := p.analyzeQueryEntities(query, entities)
	if err != nil {
		return nil, fmt.Errorf("failed to analyze query entities: %w", err)
	}

	// 为每个实体创建解析策略
	for _, entity := range requiredEntities {
		resolution, err := p.createEntityResolution(&entity)
		if err != nil {
			return nil, fmt.Errorf("failed to create resolution for entity %s: %w", entity.TypeName, err)
		}
		plan.Entities = append(plan.Entities, *resolution)
	}

	// 分析依赖关系
	dependencyOrder, err := p.AnalyzeDependencies(requiredEntities)
	if err != nil {
		return nil, fmt.Errorf("failed to analyze dependencies: %w", err)
	}
	plan.DependencyOrder = dependencyOrder

	// 收集所需服务
	plan.RequiredServices = p.collectRequiredServices(requiredEntities)

	p.logger.Debug("Entity resolution plan created",
		"entities", len(plan.Entities),
		"services", len(plan.RequiredServices),
	)

	return plan, nil
}

// BuildRepresentationQuery 构建实体表示查询
func (p *FederatedPlanner) BuildRepresentationQuery(entity *federationtypes.FederatedEntity, representations []federationtypes.RepresentationRequest) (string, error) {
	if entity == nil {
		return "", errors.NewPlanningError("entity cannot be nil")
	}

	if len(representations) == 0 {
		return "", errors.NewPlanningError("no representations provided")
	}

	p.logger.Debug("Building representation query", "entity", entity.TypeName, "representations", len(representations))

	// 构建 _entities 查询
	var queryBuilder strings.Builder
	queryBuilder.WriteString("query($representations: [_Any!]!) {\n")
	queryBuilder.WriteString("  _entities(representations: $representations) {\n")
	queryBuilder.WriteString("    ... on ")
	queryBuilder.WriteString(entity.TypeName)
	queryBuilder.WriteString(" {\n")

	// 添加实体字段
	for _, field := range entity.Fields {
		// 跳过外部字段（除非需要用于键）
		if field.Directives.External != nil && !p.isKeyField(entity, field.Name) {
			continue
		}

		queryBuilder.WriteString("      ")
		queryBuilder.WriteString(field.Name)
		queryBuilder.WriteString("\n")
	}

	queryBuilder.WriteString("    }\n")
	queryBuilder.WriteString("  }\n")
	queryBuilder.WriteString("}")

	return queryBuilder.String(), nil
}

// AnalyzeDependencies 分析实体依赖关系
func (p *FederatedPlanner) AnalyzeDependencies(entities []federationtypes.FederatedEntity) ([]string, error) {
	if len(entities) == 0 {
		return []string{}, nil
	}

	p.logger.Debug("Analyzing entity dependencies", "entityCount", len(entities))

	// 构建依赖图
	dependencyGraph := make(map[string][]string)
	serviceSet := make(map[string]bool)

	for _, entity := range entities {
		serviceName := entity.ServiceName
		serviceSet[serviceName] = true

		if _, exists := dependencyGraph[serviceName]; !exists {
			dependencyGraph[serviceName] = []string{}
		}

		// 分析字段依赖
		dependencies := p.analyzeFieldDependencies(entity, entities)
		// 注意：这里需要反过来构建图
		// 如果 A 依赖 B，那么在图中应该是 B -> A
		for _, dep := range dependencies {
			if dep != serviceName { // 避免自依赖
				// 确保依赖节点存在在图中
				if _, exists := dependencyGraph[dep]; !exists {
					dependencyGraph[dep] = []string{}
					serviceSet[dep] = true
				}
				// dep 指向 serviceName（因为 serviceName 依赖 dep）
				dependencyGraph[dep] = append(dependencyGraph[dep], serviceName)
			}
		}
	}

	// 拓扑排序
	order, err := p.topologicalSort(dependencyGraph)
	if err != nil {
		return nil, fmt.Errorf("failed to sort dependencies: %w", err)
	}

	p.logger.Debug("Dependency analysis completed", "order", order)
	return order, nil
}

// OptimizeFederationPlan 优化联邦执行计划
func (p *FederatedPlanner) OptimizeFederationPlan(plan *federationtypes.FederationPlan) (*federationtypes.FederationPlan, error) {
	if plan == nil {
		return nil, errors.NewPlanningError("plan cannot be nil")
	}

	p.logger.Debug("Optimizing federation plan", "entities", len(plan.Entities))

	optimizedPlan := &federationtypes.FederationPlan{
		Entities:         make([]federationtypes.EntityResolution, len(plan.Entities)),
		Representations:  make([]federationtypes.RepresentationRequest, len(plan.Representations)),
		RequiredServices: make([]string, len(plan.RequiredServices)),
		DependencyOrder:  make([]string, len(plan.DependencyOrder)),
	}

	// 复制原计划
	copy(optimizedPlan.Entities, plan.Entities)
	copy(optimizedPlan.Representations, plan.Representations)
	copy(optimizedPlan.RequiredServices, plan.RequiredServices)
	copy(optimizedPlan.DependencyOrder, plan.DependencyOrder)

	// 优化1: 合并相同服务的实体解析
	optimizedPlan.Entities = p.mergeEntityResolutions(optimizedPlan.Entities)

	// 优化2: 重新排序以最小化网络调用
	optimizedPlan.DependencyOrder = p.optimizeDependencyOrder(optimizedPlan.DependencyOrder, optimizedPlan.Entities)

	p.logger.Debug("Federation plan optimized",
		"originalEntities", len(plan.Entities),
		"optimizedEntities", len(optimizedPlan.Entities),
	)

	return optimizedPlan, nil
}

// 私有辅助方法

// analyzeQueryEntities 分析查询中需要的实体
func (p *FederatedPlanner) analyzeQueryEntities(query *federationtypes.ParsedQuery, allEntities []federationtypes.FederatedEntity) ([]federationtypes.FederatedEntity, error) {
	// 这里应该根据查询的 AST 分析需要哪些实体
	// 为了简化，假设所有实体都是需要的
	// 在实际实现中，需要遍历查询AST并匹配实体类型

	p.logger.Debug("Analyzing query entities", "totalEntities", len(allEntities))

	// 简化实现：返回所有实体
	// TODO: 实现真正的查询分析逻辑
	return allEntities, nil
}

// createEntityResolution 创建实体解析策略
func (p *FederatedPlanner) createEntityResolution(entity *federationtypes.FederatedEntity) (*federationtypes.EntityResolution, error) {
	if entity == nil {
		return nil, errors.NewPlanningError("entity cannot be nil")
	}

	// 提取键字段
	keyFields := p.extractKeyFields(entity)
	if len(keyFields) == 0 {
		return nil, fmt.Errorf("entity %s has no key fields", entity.TypeName)
	}

	// 构建基本查询
	query := p.buildBasicEntityQuery(entity)

	resolution := &federationtypes.EntityResolution{
		TypeName:    entity.TypeName,
		ServiceName: entity.ServiceName,
		KeyFields:   keyFields,
		Query:       query,
	}

	return resolution, nil
}

// extractKeyFields 提取实体的键字段
func (p *FederatedPlanner) extractKeyFields(entity *federationtypes.FederatedEntity) []string {
	var keyFields []string

	for _, key := range entity.Directives.Keys {
		// 解析字段选择集
		fields := strings.Fields(key.Fields)
		keyFields = append(keyFields, fields...)
	}

	// 去重
	seen := make(map[string]bool)
	var uniqueFields []string
	for _, field := range keyFields {
		if !seen[field] {
			seen[field] = true
			uniqueFields = append(uniqueFields, field)
		}
	}

	return uniqueFields
}

// buildBasicEntityQuery 构建基本实体查询
func (p *FederatedPlanner) buildBasicEntityQuery(entity *federationtypes.FederatedEntity) string {
	var queryBuilder strings.Builder

	queryBuilder.WriteString("{\n")
	for _, field := range entity.Fields {
		// 跳过外部字段（除非是键字段）
		if field.Directives.External != nil && !p.isKeyField(entity, field.Name) {
			continue
		}

		queryBuilder.WriteString("  ")
		queryBuilder.WriteString(field.Name)
		queryBuilder.WriteString("\n")
	}
	queryBuilder.WriteString("}")

	return queryBuilder.String()
}

// isKeyField 检查字段是否是键字段
func (p *FederatedPlanner) isKeyField(entity *federationtypes.FederatedEntity, fieldName string) bool {
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

// analyzeFieldDependencies 分析字段依赖关系
func (p *FederatedPlanner) analyzeFieldDependencies(entity federationtypes.FederatedEntity, allEntities []federationtypes.FederatedEntity) []string {
	var dependencies []string

	for _, field := range entity.Fields {
		// 检查 @requires 指令
		if field.Directives.Requires != nil {
			// 查找提供必需字段的服务
			requiredFields := strings.Fields(field.Directives.Requires.Fields)

			for _, requiredField := range requiredFields {
				provider := p.findFieldProvider(entity.TypeName, requiredField, allEntities)

				if provider != "" && provider != entity.ServiceName {
					dependencies = append(dependencies, provider)
				}
			}
		}
	}

	// 去重
	seen := make(map[string]bool)
	var uniqueDeps []string
	for _, dep := range dependencies {
		if !seen[dep] {
			seen[dep] = true
			uniqueDeps = append(uniqueDeps, dep)
		}
	}

	return uniqueDeps
}

// findFieldProvider 查找提供指定字段的服务
func (p *FederatedPlanner) findFieldProvider(typeName, fieldName string, entities []federationtypes.FederatedEntity) string {
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

// collectRequiredServices 收集所需服务
func (p *FederatedPlanner) collectRequiredServices(entities []federationtypes.FederatedEntity) []string {
	serviceSet := make(map[string]bool)
	for _, entity := range entities {
		serviceSet[entity.ServiceName] = true
	}

	var services []string
	for service := range serviceSet {
		services = append(services, service)
	}

	sort.Strings(services)
	return services
}

// topologicalSort 拓扑排序
func (p *FederatedPlanner) topologicalSort(graph map[string][]string) ([]string, error) {
	// 计算入度
	inDegree := make(map[string]int)
	for node := range graph {
		if _, exists := inDegree[node]; !exists {
			inDegree[node] = 0
		}
	}

	for _, neighbors := range graph {
		for _, neighbor := range neighbors {
			inDegree[neighbor]++
		}
	}

	// 找到所有入度为0的节点
	var queue []string
	for node, degree := range inDegree {
		if degree == 0 {
			queue = append(queue, node)
		}
	}

	var result []string
	for len(queue) > 0 {
		// 取出队列第一个元素
		current := queue[0]
		queue = queue[1:]
		result = append(result, current)

		// 更新邻居节点的入度
		for _, neighbor := range graph[current] {
			inDegree[neighbor]--
			if inDegree[neighbor] == 0 {
				queue = append(queue, neighbor)
			}
		}
	}

	// 检查是否有循环依赖
	if len(result) != len(inDegree) {
		return nil, errors.NewPlanningError("circular dependency detected")
	}

	return result, nil
}

// mergeEntityResolutions 合并相同服务的实体解析
func (p *FederatedPlanner) mergeEntityResolutions(resolutions []federationtypes.EntityResolution) []federationtypes.EntityResolution {
	serviceMap := make(map[string][]federationtypes.EntityResolution)

	// 按服务分组
	for _, resolution := range resolutions {
		serviceMap[resolution.ServiceName] = append(serviceMap[resolution.ServiceName], resolution)
	}

	var merged []federationtypes.EntityResolution
	for _, resolutionsForService := range serviceMap {
		if len(resolutionsForService) == 1 {
			merged = append(merged, resolutionsForService[0])
		} else {
			// 在实际实现中，这里可以合并相同服务的多个实体解析
			// 目前简化处理，保留所有解析
			merged = append(merged, resolutionsForService...)
		}
	}

	return merged
}

// optimizeDependencyOrder 优化依赖顺序
func (p *FederatedPlanner) optimizeDependencyOrder(order []string, entities []federationtypes.EntityResolution) []string {
	// 简化实现：保持原顺序
	// 在实际实现中，可以根据实体的复杂度、响应时间等因素进行优化
	return order
}
