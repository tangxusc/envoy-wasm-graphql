package federation

import (
	"context"
	"fmt"
	"strings"

	"github.com/tidwall/gjson"

	"envoy-wasm-graphql-federation/pkg/errors"
	"envoy-wasm-graphql-federation/pkg/jsonutil"
	federationtypes "envoy-wasm-graphql-federation/pkg/types"
)

// EntityResolverImpl 实现实体解析器
type EntityResolverImpl struct {
	logger        federationtypes.Logger
	serviceCaller federationtypes.ServiceCaller
}

// NewEntityResolver 创建新的实体解析器
func NewEntityResolver(logger federationtypes.Logger, caller federationtypes.ServiceCaller) federationtypes.EntityResolver {
	return &EntityResolverImpl{
		logger:        logger,
		serviceCaller: caller,
	}
}

// ResolveEntity 解析单个实体
func (r *EntityResolverImpl) ResolveEntity(ctx context.Context, serviceName string, representation federationtypes.RepresentationRequest) (interface{}, error) {
	if serviceName == "" {
		return nil, errors.NewResolutionError("service name cannot be empty")
	}

	r.logger.Debug("Resolving entity", "service", serviceName, "typename", representation.TypeName)

	// 构建 _entities 查询
	query, err := r.buildEntityQuery(representation)
	if err != nil {
		return nil, fmt.Errorf("failed to build entity query: %w", err)
	}

	// 准备变量
	variables := map[string]interface{}{
		"representations": []interface{}{representation.Representation},
	}

	// 创建服务调用
	serviceCall := &federationtypes.ServiceCall{
		Service: &federationtypes.ServiceConfig{
			Name: serviceName,
		},
		SubQuery: &federationtypes.SubQuery{
			ServiceName: serviceName,
			Query:       query,
			Variables:   variables,
		},
		Context: &federationtypes.QueryContext{
			RequestID: "entity-resolution",
		},
	}

	// 调用服务
	response, err := r.serviceCaller.Call(ctx, serviceCall)
	if err != nil {
		return nil, fmt.Errorf("service call failed: %w", err)
	}

	// 处理响应
	if response.Error != nil {
		return nil, fmt.Errorf("service returned error: %w", response.Error)
	}

	// 提取实体数据
	entityData, err := r.extractEntityFromResponse(response, representation.TypeName)
	if err != nil {
		return nil, fmt.Errorf("failed to extract entity data: %w", err)
	}

	r.logger.Debug("Entity resolved successfully", "service", serviceName, "typename", representation.TypeName)
	return entityData, nil
}

// ResolveBatchEntities 批量解析实体
func (r *EntityResolverImpl) ResolveBatchEntities(ctx context.Context, serviceName string, representations []federationtypes.RepresentationRequest) ([]interface{}, error) {
	if serviceName == "" {
		return nil, errors.NewResolutionError("service name cannot be empty")
	}

	if len(representations) == 0 {
		return []interface{}{}, nil
	}

	r.logger.Debug("Resolving batch entities", "service", serviceName, "count", len(representations))

	// 按类型分组表示
	typeGroups := r.groupRepresentationsByType(representations)
	var allResults []interface{}

	for typeName, typeRepresentations := range typeGroups {
		// 构建批量查询
		query, err := r.buildBatchEntityQuery(typeName, typeRepresentations)
		if err != nil {
			return nil, fmt.Errorf("failed to build batch query for type %s: %w", typeName, err)
		}

		// 准备变量
		variables := map[string]interface{}{
			"representations": r.extractRepresentationData(typeRepresentations),
		}

		// 创建服务调用
		serviceCall := &federationtypes.ServiceCall{
			Service: &federationtypes.ServiceConfig{
				Name: serviceName,
			},
			SubQuery: &federationtypes.SubQuery{
				ServiceName: serviceName,
				Query:       query,
				Variables:   variables,
			},
			Context: &federationtypes.QueryContext{
				RequestID: "batch-entity-resolution",
			},
		}

		// 调用服务
		response, err := r.serviceCaller.Call(ctx, serviceCall)
		if err != nil {
			return nil, fmt.Errorf("batch service call failed: %w", err)
		}

		// 处理响应
		if response.Error != nil {
			return nil, fmt.Errorf("service returned error: %w", response.Error)
		}

		// 提取实体数据
		entities, err := r.extractEntitiesFromResponse(response, typeName)
		if err != nil {
			return nil, fmt.Errorf("failed to extract entities data: %w", err)
		}

		allResults = append(allResults, entities...)
	}

	r.logger.Debug("Batch entities resolved successfully", "service", serviceName, "totalCount", len(allResults))
	return allResults, nil
}

// ValidateRepresentation 验证实体表示的有效性
func (r *EntityResolverImpl) ValidateRepresentation(entity *federationtypes.FederatedEntity, representation federationtypes.RepresentationRequest) error {
	if entity == nil {
		return errors.NewValidationError("entity cannot be nil")
	}

	if representation.TypeName == "" {
		return errors.NewValidationError("representation typename cannot be empty")
	}

	if representation.TypeName != entity.TypeName {
		return fmt.Errorf("representation typename %s does not match entity typename %s", representation.TypeName, entity.TypeName)
	}

	// 验证表示包含所有必需的键字段
	err := r.validateKeyFields(entity, representation.Representation)
	if err != nil {
		return fmt.Errorf("key field validation failed: %w", err)
	}

	r.logger.Debug("Representation validated successfully", "typename", representation.TypeName)
	return nil
}

// 私有辅助方法

// buildEntityQuery 构建单个实体查询
func (r *EntityResolverImpl) buildEntityQuery(representation federationtypes.RepresentationRequest) (string, error) {
	if representation.TypeName == "" {
		return "", errors.NewQueryBuildingError("typename cannot be empty")
	}

	query := fmt.Sprintf(`
		query($representations: [_Any!]!) {
			_entities(representations: $representations) {
				... on %s {
					__typename
				}
			}
		}
	`, representation.TypeName)

	return query, nil
}

// buildBatchEntityQuery 构建批量实体查询
func (r *EntityResolverImpl) buildBatchEntityQuery(typeName string, representations []federationtypes.RepresentationRequest) (string, error) {
	if typeName == "" {
		return "", errors.NewQueryBuildingError("typename cannot be empty")
	}

	if len(representations) == 0 {
		return "", errors.NewQueryBuildingError("no representations provided")
	}

	query := fmt.Sprintf(`
		query($representations: [_Any!]!) {
			_entities(representations: $representations) {
				... on %s {
					__typename
				}
			}
		}
	`, typeName)

	return query, nil
}

// groupRepresentationsByType 按类型分组表示
func (r *EntityResolverImpl) groupRepresentationsByType(representations []federationtypes.RepresentationRequest) map[string][]federationtypes.RepresentationRequest {
	groups := make(map[string][]federationtypes.RepresentationRequest)

	for _, repr := range representations {
		typeName := repr.TypeName
		groups[typeName] = append(groups[typeName], repr)
	}

	return groups
}

// extractRepresentationData 提取表示数据
func (r *EntityResolverImpl) extractRepresentationData(representations []federationtypes.RepresentationRequest) []interface{} {
	var data []interface{}

	for _, repr := range representations {
		// 添加 __typename 字段
		reprData := make(map[string]interface{})
		for key, value := range repr.Representation {
			reprData[key] = value
		}
		reprData["__typename"] = repr.TypeName
		data = append(data, reprData)
	}

	return data
}

// extractEntityFromResponse 从响应中提取实体数据
func (r *EntityResolverImpl) extractEntityFromResponse(response *federationtypes.ServiceResponse, typeName string) (interface{}, error) {
	if response.Data == nil {
		return nil, errors.NewDataExtractionError("response data is nil")
	}

	// 使用 jsonutil 解析响应数据
	dataStr, err := jsonutil.Marshal(response.Data)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal response data: %w", err)
	}

	// 提取 _entities 数组
	entitiesValue := gjson.Get(string(dataStr), "_entities")
	if !entitiesValue.Exists() {
		return nil, errors.NewDataExtractionError("_entities field not found in response")
	}

	// 获取第一个实体（单个实体解析）
	if entitiesValue.IsArray() {
		entities := entitiesValue.Array()
		if len(entities) > 0 {
			return entities[0].Value(), nil
		}
	}

	return nil, errors.NewDataExtractionError("no entity found in response")
}

// extractEntitiesFromResponse 从响应中提取多个实体数据
func (r *EntityResolverImpl) extractEntitiesFromResponse(response *federationtypes.ServiceResponse, typeName string) ([]interface{}, error) {
	if response.Data == nil {
		return nil, errors.NewDataExtractionError("response data is nil")
	}

	// 使用 jsonutil 解析响应数据
	dataStr, err := jsonutil.Marshal(response.Data)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal response data: %w", err)
	}

	// 提取 _entities 数组
	entitiesValue := gjson.Get(string(dataStr), "_entities")
	if !entitiesValue.Exists() {
		return nil, errors.NewDataExtractionError("_entities field not found in response")
	}

	var results []interface{}
	if entitiesValue.IsArray() {
		entities := entitiesValue.Array()
		for _, entity := range entities {
			results = append(results, entity.Value())
		}
	}

	return results, nil
}

// validateKeyFields 验证键字段
func (r *EntityResolverImpl) validateKeyFields(entity *federationtypes.FederatedEntity, representation map[string]interface{}) error {
	// 收集所有键字段
	requiredKeys := make(map[string]bool)

	for _, key := range entity.Directives.Keys {
		// 解析字段选择集
		fields := r.parseFieldSelection(key.Fields)
		for _, field := range fields {
			requiredKeys[field] = true
		}
	}

	// 检查表示是否包含所有必需的键字段
	for keyField := range requiredKeys {
		if _, exists := representation[keyField]; !exists {
			return fmt.Errorf("missing required key field: %s", keyField)
		}
	}

	return nil
}

// parseFieldSelection 解析字段选择集
func (r *EntityResolverImpl) parseFieldSelection(fields string) []string {
	// 简单实现：按空格分割
	// 在实际实现中，可能需要更复杂的解析逻辑
	var result []string
	for _, field := range strings.Fields(fields) {
		if field != "" {
			result = append(result, field)
		}
	}
	return result
}
