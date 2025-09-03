package merger

import (
	"context"
	"envoy-wasm-graphql-federation/pkg/jsonutil"
	"fmt"
	"reflect"
	"sort"

	"envoy-wasm-graphql-federation/pkg/errors"
	federationtypes "envoy-wasm-graphql-federation/pkg/types"
)

// ResponseMerger 实现GraphQL响应合并器
type ResponseMerger struct {
	logger federationtypes.Logger
	config *MergerConfig
}

// MergerConfig 合并器配置
type MergerConfig struct {
	MaxDepth       int                    // 最大合并深度
	ConflictPolicy ConflictPolicy         // 冲突处理策略
	NullPolicy     NullPolicy             // null值处理策略
	TypeMapping    map[string]string      // 类型映射
	FieldMapping   map[string]FieldMerger // 字段合并器映射
	EnableMetrics  bool                   // 是否启用指标收集
}

// ConflictPolicy 冲突处理策略
type ConflictPolicy string

const (
	ConflictPolicyFirst ConflictPolicy = "first" // 使用第一个值
	ConflictPolicyLast  ConflictPolicy = "last"  // 使用最后一个值
	ConflictPolicyMerge ConflictPolicy = "merge" // 尝试合并
	ConflictPolicyError ConflictPolicy = "error" // 抛出错误
)

// NullPolicy null值处理策略
type NullPolicy string

const (
	NullPolicySkip     NullPolicy = "skip"     // 跳过null值
	NullPolicyKeep     NullPolicy = "keep"     // 保留null值
	NullPolicyOverride NullPolicy = "override" // null覆盖非null
)

// FieldMerger 字段合并器接口
type FieldMerger interface {
	MergeField(fieldName string, values []interface{}) (interface{}, error)
}

// MergeResult 合并结果
type MergeResult struct {
	Data       interface{}                    `json:"data,omitempty"`
	Errors     []federationtypes.GraphQLError `json:"errors,omitempty"`
	Extensions map[string]interface{}         `json:"extensions,omitempty"`
	Metadata   *MergeMetadata                 `json:"metadata,omitempty"`
}

// MergeMetadata 合并元数据
type MergeMetadata struct {
	MergedServices []string                      `json:"mergedServices"`
	ConflictCount  int                           `json:"conflictCount"`
	MergeStrategy  federationtypes.MergeStrategy `json:"mergeStrategy"`
	ProcessingTime string                        `json:"processingTime"`
	FieldCount     int                           `json:"fieldCount"`
}

// NewResponseMerger 创建新的响应合并器
func NewResponseMerger(config *MergerConfig, logger federationtypes.Logger) federationtypes.ResponseMerger {
	if config == nil {
		config = DefaultMergerConfig()
	}

	return &ResponseMerger{
		logger: logger,
		config: config,
	}
}

// DefaultMergerConfig 返回默认配置
func DefaultMergerConfig() *MergerConfig {
	return &MergerConfig{
		MaxDepth:       10,
		ConflictPolicy: ConflictPolicyFirst,
		NullPolicy:     NullPolicySkip,
		TypeMapping:    make(map[string]string),
		FieldMapping:   make(map[string]FieldMerger),
		EnableMetrics:  true,
	}
}

// MergeResponses 合并多个服务响应
func (m *ResponseMerger) MergeResponses(ctx context.Context, responses []*federationtypes.ServiceResponse, plan *federationtypes.ExecutionPlan) (*federationtypes.GraphQLResponse, error) {
	if len(responses) == 0 {
		return &federationtypes.GraphQLResponse{
			Data: nil,
		}, nil
	}

	// 如果 plan 为 nil，创建一个默认的计划
	if plan == nil {
		plan = &federationtypes.ExecutionPlan{
			MergeStrategy: federationtypes.MergeStrategyShallow,
		}
	}

	m.logger.Debug("Merging responses",
		"responseCount", len(responses),
		"strategy", plan.MergeStrategy,
	)

	// 根据策略选择合并方法
	switch plan.MergeStrategy {
	case federationtypes.MergeStrategyDeep:
		return m.mergeDeep(ctx, responses, plan)
	case federationtypes.MergeStrategyShallow:
		return m.mergeShallow(ctx, responses, plan)
	default:
		return m.mergeShallow(ctx, responses, plan)
	}
}

// mergeDeep 深度合并响应
func (m *ResponseMerger) mergeDeep(ctx context.Context, responses []*federationtypes.ServiceResponse, plan *federationtypes.ExecutionPlan) (*federationtypes.GraphQLResponse, error) {
	result := &federationtypes.GraphQLResponse{
		Extensions: make(map[string]interface{}),
	}

	var allErrors []federationtypes.GraphQLError
	var validResponses []*federationtypes.ServiceResponse
	mergedServices := make([]string, 0, len(responses))

	// 收集有效响应和错误
	for _, resp := range responses {
		if resp.Error != nil {
			// 将服务错误转换为GraphQL错误
			graphqlErr := federationtypes.GraphQLError{
				Message: fmt.Sprintf("Service %s error: %s", resp.Service, resp.Error.Error()),
				Extensions: map[string]interface{}{
					"service": resp.Service,
					"code":    "SERVICE_ERROR",
				},
			}
			allErrors = append(allErrors, graphqlErr)
			continue
		}

		if resp.Errors != nil {
			allErrors = append(allErrors, resp.Errors...)
		}

		if resp.Data != nil {
			validResponses = append(validResponses, resp)
			mergedServices = append(mergedServices, resp.Service)
		}
	}

	// 如果没有有效数据，返回错误
	if len(validResponses) == 0 {
		result.Errors = allErrors
		return result, nil
	}

	// 深度合并数据
	mergedData, err := m.mergeDataDeep(validResponses, 0)
	if err != nil {
		return nil, errors.NewMergeError("deep merge failed: " + err.Error())
	}

	result.Data = mergedData
	result.Errors = m.MergeErrors(allErrors)
	result.Extensions = m.mergeExtensionsDeep(validResponses)

	m.logger.Debug("Deep merge completed",
		"services", mergedServices,
		"errors", len(result.Errors),
	)

	return result, nil
}

// mergeShallow 浅合并响应
func (m *ResponseMerger) mergeShallow(ctx context.Context, responses []*federationtypes.ServiceResponse, plan *federationtypes.ExecutionPlan) (*federationtypes.GraphQLResponse, error) {
	result := &federationtypes.GraphQLResponse{
		Data:       make(map[string]interface{}),
		Extensions: make(map[string]interface{}),
	}

	var allErrors []federationtypes.GraphQLError
	dataMap := result.Data.(map[string]interface{})
	mergedServices := make([]string, 0, len(responses))

	// 浅合并每个响应
	for _, resp := range responses {
		if resp.Error != nil {
			graphqlErr := federationtypes.GraphQLError{
				Message: fmt.Sprintf("Service %s error: %s", resp.Service, resp.Error.Error()),
				Extensions: map[string]interface{}{
					"service": resp.Service,
					"code":    "SERVICE_ERROR",
				},
			}
			allErrors = append(allErrors, graphqlErr)
			continue
		}

		if resp.Errors != nil {
			allErrors = append(allErrors, resp.Errors...)
		}

		if resp.Data != nil {
			mergedServices = append(mergedServices, resp.Service)

			// 将响应数据合并到结果中
			if respData, ok := resp.Data.(map[string]interface{}); ok {
				for key, value := range respData {
					if existing, exists := dataMap[key]; exists {
						// 处理字段冲突
						mergedValue, err := m.resolveFieldConflict(key, existing, value)
						if err != nil {
							m.logger.Warn("Field conflict resolution failed",
								"field", key,
								"service", resp.Service,
								"error", err,
							)
							continue
						}
						dataMap[key] = mergedValue
					} else {
						dataMap[key] = value
					}
				}
			}
		}
	}

	result.Errors = m.MergeErrors(allErrors)
	result.Extensions = m.MergeExtensions(m.extractExtensions(responses))

	m.logger.Debug("Shallow merge completed",
		"services", mergedServices,
		"fields", len(dataMap),
		"errors", len(result.Errors),
	)

	return result, nil
}

// mergeDataDeep 深度合并数据
func (m *ResponseMerger) mergeDataDeep(responses []*federationtypes.ServiceResponse, depth int) (interface{}, error) {
	if depth > m.config.MaxDepth {
		return nil, fmt.Errorf("maximum merge depth %d exceeded", m.config.MaxDepth)
	}

	if len(responses) == 0 {
		return nil, nil
	}

	if len(responses) == 1 {
		return responses[0].Data, nil
	}

	// 检查所有响应的数据类型
	var dataItems []interface{}
	for _, resp := range responses {
		if resp.Data != nil {
			dataItems = append(dataItems, resp.Data)
		}
	}

	if len(dataItems) == 0 {
		return nil, nil
	}

	// 根据第一个数据项的类型决定合并策略
	firstItem := dataItems[0]
	switch firstType := firstItem.(type) {
	case map[string]interface{}:
		return m.mergeObjects(dataItems, depth)
	case []interface{}:
		return m.mergeArrays(dataItems, depth)
	default:
		// 对于基本类型，使用冲突解决策略
		return m.resolvePrimitiveConflict(dataItems, reflect.TypeOf(firstType).String())
	}
}

// mergeObjects 合并对象
func (m *ResponseMerger) mergeObjects(objects []interface{}, depth int) (map[string]interface{}, error) {
	result := make(map[string]interface{})

	for _, obj := range objects {
		objMap, ok := obj.(map[string]interface{})
		if !ok {
			continue
		}

		for key, value := range objMap {
			if existing, exists := result[key]; exists {
				// 递归合并子对象
				if m.shouldMergeRecursively(existing, value) {
					mergedValue, err := m.mergeDataDeep([]*federationtypes.ServiceResponse{
						{Data: existing},
						{Data: value},
					}, depth+1)
					if err != nil {
						return nil, err
					}
					result[key] = mergedValue
				} else {
					// 使用冲突解决策略
					resolvedValue, err := m.resolveFieldConflict(key, existing, value)
					if err != nil {
						return nil, err
					}
					result[key] = resolvedValue
				}
			} else {
				result[key] = value
			}
		}
	}

	return result, nil
}

// mergeArrays 合并数组
func (m *ResponseMerger) mergeArrays(arrays []interface{}, depth int) ([]interface{}, error) {
	var result []interface{}

	for _, arr := range arrays {
		arrSlice, ok := arr.([]interface{})
		if !ok {
			continue
		}
		result = append(result, arrSlice...)
	}

	// 去重（基于JSON序列化比较）
	return m.deduplicateArray(result), nil
}

// shouldMergeRecursively 判断是否应该递归合并
func (m *ResponseMerger) shouldMergeRecursively(existing, value interface{}) bool {
	// 如果两个值都是对象，递归合并
	_, existingIsObj := existing.(map[string]interface{})
	_, valueIsObj := value.(map[string]interface{})

	if existingIsObj && valueIsObj {
		return true
	}

	// 如果两个值都是数组，合并数组
	_, existingIsArr := existing.([]interface{})
	_, valueIsArr := value.([]interface{})

	return existingIsArr && valueIsArr
}

// resolveFieldConflict 解决字段冲突
func (m *ResponseMerger) resolveFieldConflict(fieldName string, existing, value interface{}) (interface{}, error) {
	// 检查是否有自定义字段合并器
	if merger, ok := m.config.FieldMapping[fieldName]; ok {
		return merger.MergeField(fieldName, []interface{}{existing, value})
	}

	// 处理null值
	if value == nil {
		switch m.config.NullPolicy {
		case NullPolicySkip:
			return existing, nil
		case NullPolicyKeep:
			return value, nil
		case NullPolicyOverride:
			return value, nil
		}
	}

	if existing == nil {
		return value, nil
	}

	// 使用冲突策略
	switch m.config.ConflictPolicy {
	case ConflictPolicyFirst:
		return existing, nil
	case ConflictPolicyLast:
		return value, nil
	case ConflictPolicyMerge:
		return m.attemptMerge(existing, value)
	case ConflictPolicyError:
		return nil, fmt.Errorf("field conflict detected for %s", fieldName)
	default:
		return existing, nil
	}
}

// attemptMerge 尝试合并两个值
func (m *ResponseMerger) attemptMerge(existing, value interface{}) (interface{}, error) {
	// 如果类型相同，尝试合并
	if reflect.TypeOf(existing) == reflect.TypeOf(value) {
		switch existing.(type) {
		case map[string]interface{}:
			return m.mergeObjects([]interface{}{existing, value}, 0)
		case []interface{}:
			return m.mergeArrays([]interface{}{existing, value}, 0)
		case string:
			// 字符串合并（用空格连接）
			return fmt.Sprintf("%s %s", existing, value), nil
		case int, int64, float64:
			// 数值合并（求和）
			return m.mergeNumbers(existing, value)
		}
	}

	// 无法合并，返回第一个值
	return existing, nil
}

// mergeNumbers 合并数值
func (m *ResponseMerger) mergeNumbers(a, b interface{}) (interface{}, error) {
	aVal := reflect.ValueOf(a)
	bVal := reflect.ValueOf(b)

	if aVal.Kind() == reflect.Float64 || bVal.Kind() == reflect.Float64 {
		aFloat := m.toFloat64(a)
		bFloat := m.toFloat64(b)
		return aFloat + bFloat, nil
	}

	aInt := m.toInt64(a)
	bInt := m.toInt64(b)
	return aInt + bInt, nil
}

// toFloat64 转换为float64
func (m *ResponseMerger) toFloat64(val interface{}) float64 {
	switch v := val.(type) {
	case int:
		return float64(v)
	case int64:
		return float64(v)
	case float64:
		return v
	case float32:
		return float64(v)
	default:
		return 0
	}
}

// toInt64 转换为int64
func (m *ResponseMerger) toInt64(val interface{}) int64 {
	switch v := val.(type) {
	case int:
		return int64(v)
	case int64:
		return v
	case float64:
		return int64(v)
	case float32:
		return int64(v)
	default:
		return 0
	}
}

// resolvePrimitiveConflict 解决基本类型冲突
func (m *ResponseMerger) resolvePrimitiveConflict(values []interface{}, typeName string) (interface{}, error) {
	if len(values) == 0 {
		return nil, nil
	}

	switch m.config.ConflictPolicy {
	case ConflictPolicyFirst:
		return values[0], nil
	case ConflictPolicyLast:
		return values[len(values)-1], nil
	case ConflictPolicyError:
		return nil, fmt.Errorf("primitive type conflict for type %s", typeName)
	default:
		return values[0], nil
	}
}

// deduplicateArray 数组去重
func (m *ResponseMerger) deduplicateArray(arr []interface{}) []interface{} {
	seen := make(map[string]bool)
	var result []interface{}

	for _, item := range arr {
		// 使用JSON序列化作为唯一性标识
		jsonBytes, err := jsonutil.Marshal(item)
		if err != nil {
			// 序列化失败，直接添加
			result = append(result, item)
			continue
		}

		key := string(jsonBytes)
		if !seen[key] {
			seen[key] = true
			result = append(result, item)
		}
	}

	return result
}

// MergeErrors 合并错误信息
func (m *ResponseMerger) MergeErrors(errors []federationtypes.GraphQLError) []federationtypes.GraphQLError {
	if len(errors) == 0 {
		return nil
	}

	// 去重错误
	seen := make(map[string]bool)
	var uniqueErrors []federationtypes.GraphQLError

	for _, err := range errors {
		// 使用消息作为唯一性标识
		key := err.Message
		if err.Extensions != nil {
			if code, ok := err.Extensions["code"]; ok {
				key = fmt.Sprintf("%s:%s", key, code)
			}
		}

		if !seen[key] {
			seen[key] = true
			uniqueErrors = append(uniqueErrors, err)
		}
	}

	// 按严重程度排序
	sort.Slice(uniqueErrors, func(i, j int) bool {
		return m.getErrorSeverity(uniqueErrors[i]) > m.getErrorSeverity(uniqueErrors[j])
	})

	return uniqueErrors
}

// getErrorSeverity 获取错误严重程度
func (m *ResponseMerger) getErrorSeverity(err federationtypes.GraphQLError) int {
	if err.Extensions != nil {
		if code, ok := err.Extensions["code"]; ok {
			switch code {
			case "INTERNAL_ERROR":
				return 100
			case "SERVICE_ERROR":
				return 90
			case "VALIDATION_ERROR":
				return 80
			case "AUTHORIZATION_ERROR":
				return 70
			case "RATE_LIMIT_ERROR":
				return 60
			default:
				return 50
			}
		}
	}
	return 50
}

// MergeExtensions 合并扩展字段
func (m *ResponseMerger) MergeExtensions(extensions []map[string]interface{}) map[string]interface{} {
	if len(extensions) == 0 {
		return nil
	}

	result := make(map[string]interface{})

	for _, ext := range extensions {
		for key, value := range ext {
			if existing, exists := result[key]; exists {
				// 尝试合并扩展字段
				if merged, err := m.attemptMerge(existing, value); err == nil {
					result[key] = merged
				} else {
					// 合并失败，使用最后一个值
					result[key] = value
				}
			} else {
				result[key] = value
			}
		}
	}

	return result
}

// mergeExtensionsDeep 深度合并扩展字段
func (m *ResponseMerger) mergeExtensionsDeep(responses []*federationtypes.ServiceResponse) map[string]interface{} {
	extensions := m.extractExtensions(responses)
	merged := m.MergeExtensions(extensions)

	// 添加合并元数据
	if merged == nil {
		merged = make(map[string]interface{})
	}

	services := make([]string, 0, len(responses))
	for _, resp := range responses {
		services = append(services, resp.Service)
	}

	merged["merge_metadata"] = map[string]interface{}{
		"merged_services": services,
		"merge_strategy":  "deep",
		"response_count":  len(responses),
	}

	return merged
}

// extractExtensions 提取扩展字段
func (m *ResponseMerger) extractExtensions(responses []*federationtypes.ServiceResponse) []map[string]interface{} {
	var extensions []map[string]interface{}

	for _, resp := range responses {
		if resp.Headers != nil {
			ext := make(map[string]interface{})
			ext["service"] = resp.Service
			ext["headers"] = resp.Headers
			ext["latency"] = resp.Latency.String()

			if resp.Metadata != nil {
				ext["metadata"] = resp.Metadata
			}

			extensions = append(extensions, ext)
		}
	}

	return extensions
}
