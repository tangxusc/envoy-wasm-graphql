package types

import (
	"context"
	"time"
)

// GraphQLParser 接口定义 GraphQL 查询解析器
type GraphQLParser interface {
	// ParseQuery 解析 GraphQL 查询
	ParseQuery(query string) (*ParsedQuery, error)

	// ValidateQuery 验证查询合法性
	ValidateQuery(query *ParsedQuery, schema *Schema) error

	// ExtractFields 提取查询字段信息
	ExtractFields(query *ParsedQuery) ([]FieldPath, error)
}

// QueryPlanner 接口定义查询规划器
type QueryPlanner interface {
	// CreateExecutionPlan 创建执行计划
	CreateExecutionPlan(ctx context.Context, query *ParsedQuery, services []ServiceConfig) (*ExecutionPlan, error)

	// OptimizePlan 优化执行计划
	OptimizePlan(plan *ExecutionPlan) (*ExecutionPlan, error)

	// ValidatePlan 验证执行计划
	ValidatePlan(plan *ExecutionPlan) error
}

// ServiceCaller 接口定义服务调用器
type ServiceCaller interface {
	// Call 调用单个服务
	Call(ctx context.Context, call *ServiceCall) (*ServiceResponse, error)

	// CallBatch 批量调用服务
	CallBatch(ctx context.Context, calls []*ServiceCall) ([]*ServiceResponse, error)

	// IsHealthy 检查服务健康状态
	IsHealthy(ctx context.Context, service *ServiceConfig) bool
}

// ResponseMerger 接口定义响应合并器
type ResponseMerger interface {
	// MergeResponses 合并多个服务响应
	MergeResponses(ctx context.Context, responses []*ServiceResponse, plan *ExecutionPlan) (*GraphQLResponse, error)

	// MergeErrors 合并错误信息
	MergeErrors(errors []GraphQLError) []GraphQLError

	// MergeExtensions 合并扩展字段
	MergeExtensions(extensions []map[string]interface{}) map[string]interface{}
}

// ConfigManager 接口定义配置管理器
type ConfigManager interface {
	// LoadConfig 加载配置
	LoadConfig(data []byte) (*FederationConfig, error)

	// ValidateConfig 验证配置
	ValidateConfig(config *FederationConfig) error

	// ReloadConfig 重新加载配置
	ReloadConfig(data []byte) error

	// GetServiceConfig 获取服务配置
	GetServiceConfig(serviceName string) (*ServiceConfig, error)
}

// SchemaRegistry 接口定义模式注册中心
type SchemaRegistry interface {
	// RegisterSchema 注册模式
	RegisterSchema(serviceName string, schema string) error

	// GetSchema 获取模式
	GetSchema(serviceName string) (*SchemaInfo, error)

	// GetFederatedSchema 获取联邦模式
	GetFederatedSchema() (*Schema, error)

	// ValidateSchema 验证模式
	ValidateSchema(schema string) error

	// RefreshSchemas 刷新所有模式
	RefreshSchemas(ctx context.Context) error
}

// CacheManager 接口定义缓存管理器
type CacheManager interface {
	// Get 获取缓存值
	Get(key string) (interface{}, bool)

	// Set 设置缓存值
	Set(key string, value interface{}, expiration time.Duration) error

	// Delete 删除缓存
	Delete(key string) error

	// Clear 清空缓存
	Clear() error

	// Stats 获取缓存统计信息
	Stats() CacheStats
}

// ErrorHandler 接口定义错误处理器
type ErrorHandler interface {
	// HandleError 处理错误
	HandleError(ctx context.Context, err error) *GraphQLError

	// HandleServiceError 处理服务错误
	HandleServiceError(ctx context.Context, service string, err error) *GraphQLError

	// HandleValidationError 处理验证错误
	HandleValidationError(ctx context.Context, err error) *GraphQLError
}

// Logger 接口定义日志记录器
type Logger interface {
	// Debug 记录调试信息
	Debug(msg string, fields ...interface{})

	// Info 记录信息
	Info(msg string, fields ...interface{})

	// Warn 记录警告
	Warn(msg string, fields ...interface{})

	// Error 记录错误
	Error(msg string, fields ...interface{})

	// Fatal 记录致命错误
	Fatal(msg string, fields ...interface{})
}

// MetricsCollector 接口定义指标收集器
type MetricsCollector interface {
	// IncrementCounter 增加计数器
	IncrementCounter(name string, labels map[string]string)

	// RecordHistogram 记录直方图
	RecordHistogram(name string, value float64, labels map[string]string)

	// SetGauge 设置仪表盘
	SetGauge(name string, value float64, labels map[string]string)

	// GetMetrics 获取指标
	GetMetrics() map[string]interface{}
}

// FederationEngine 接口定义联邦引擎
type FederationEngine interface {
	// Initialize 初始化引擎
	Initialize(config *FederationConfig) error

	// ExecuteQuery 执行查询
	ExecuteQuery(ctx context.Context, request *GraphQLRequest) (*GraphQLResponse, error)

	// Shutdown 关闭引擎
	Shutdown() error

	// GetStatus 获取状态
	GetStatus() EngineStatus
}

// FederationDirectiveParser 接口定义 Federation 指令解析器
type FederationDirectiveParser interface {
	// ParseDirectives 解析类型上的 Federation 指令
	ParseDirectives(typeDef string) (*EntityDirectives, error)

	// ParseKeyDirective 解析 @key 指令
	ParseKeyDirective(directive string) (*KeyDirective, error)

	// ParseExternalDirective 解析 @external 指令
	ParseExternalDirective(directive string) (*ExternalDirective, error)

	// ParseRequiresDirective 解析 @requires 指令
	ParseRequiresDirective(directive string) (*RequiresDirective, error)

	// ParseProvidesDirective 解析 @provides 指令
	ParseProvidesDirective(directive string) (*ProvidesDirective, error)

	// ValidateDirectives 验证指令的有效性
	ValidateDirectives(directives *EntityDirectives) error
}

// FederationPlanner 接口定义联邦查询规划器
type FederationPlanner interface {
	// PlanEntityResolution 规划实体解析
	PlanEntityResolution(entities []FederatedEntity, query *ParsedQuery) (*FederationPlan, error)

	// BuildRepresentationQuery 构建实体表示查询
	BuildRepresentationQuery(entity *FederatedEntity, representations []RepresentationRequest) (string, error)

	// AnalyzeDependencies 分析实体依赖关系
	AnalyzeDependencies(entities []FederatedEntity) ([]string, error)

	// OptimizeFederationPlan 优化联邦执行计划
	OptimizeFederationPlan(plan *FederationPlan) (*FederationPlan, error)
}

// EntityResolver 接口定义实体解析器
type EntityResolver interface {
	// ResolveEntity 解析单个实体
	ResolveEntity(ctx context.Context, serviceName string, representation RepresentationRequest) (interface{}, error)

	// ResolveBatchEntities 批量解析实体
	ResolveBatchEntities(ctx context.Context, serviceName string, representations []RepresentationRequest) ([]interface{}, error)

	// ValidateRepresentation 验证实体表示的有效性
	ValidateRepresentation(entity *FederatedEntity, representation RepresentationRequest) error
}

// FederationValidator 接口定义联邦验证器
type FederationValidator interface {
	// ValidateFederatedSchema 验证联邦模式
	ValidateFederatedSchema(schema *FederatedSchema) error

	// ValidateEntityCompatibility 验证实体兼容性
	ValidateEntityCompatibility(entities []FederatedEntity) error

	// ValidateKeyFields 验证键字段
	ValidateKeyFields(entity *FederatedEntity, keyFields []string) error

	// ValidateRequiredFields 验证必需字段
	ValidateRequiredFields(entity *FederatedEntity, requiredFields []string) error
}

// 辅助类型定义

// ParsedQuery 表示解析后的查询
type ParsedQuery struct {
	AST        interface{} // GraphQL AST
	Operation  string
	Variables  map[string]interface{}
	Fragments  map[string]interface{}
	Complexity int
	Depth      int
}

// Schema 表示 GraphQL 模式
type Schema struct {
	SDL       string
	Types     map[string]*TypeDefinition
	Queries   map[string]*FieldDefinition
	Mutations map[string]*FieldDefinition
	Version   string
}

// TypeDefinition 表示类型定义
type TypeDefinition struct {
	Name        string
	Kind        string
	Fields      map[string]*FieldDefinition
	Interfaces  []string
	Description string
}

// FieldDefinition 表示字段定义
type FieldDefinition struct {
	Name        string
	Type        string
	Arguments   map[string]*ArgumentDefinition
	Resolver    string
	Description string
}

// ArgumentDefinition 表示参数定义
type ArgumentDefinition struct {
	Name         string
	Type         string
	DefaultValue interface{}
	Description  string
}

// FieldPath 表示字段路径
type FieldPath struct {
	Service string
	Path    []string
	Type    string
}

// CacheStats 表示缓存统计
type CacheStats struct {
	HitCount    int64
	MissCount   int64
	EntryCount  int64
	MemoryUsage int64
}

// EngineStatus 表示引擎状态
type EngineStatus struct {
	Status     string
	Uptime     time.Duration
	QueryCount int64
	ErrorCount int64
	Services   map[string]ServiceStatus
}

// ServiceStatus 表示服务状态
type ServiceStatus struct {
	Name         string
	Healthy      bool
	LastCheck    time.Time
	ResponseTime time.Duration
	ErrorRate    float64
}
