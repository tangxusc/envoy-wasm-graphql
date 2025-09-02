package types

import (
	"time"
)

// QueryContext 表示 GraphQL 查询上下文
type QueryContext struct {
	Query     string                 `json:"query"`
	Variables map[string]interface{} `json:"variables,omitempty"`
	Operation string                 `json:"operationName,omitempty"`
	RequestID string                 `json:"requestId"`
	UserID    string                 `json:"userId,omitempty"`
	Headers   map[string]string      `json:"headers,omitempty"`
}

// ExecutionPlan 表示查询执行计划
type ExecutionPlan struct {
	SubQueries    []SubQuery             `json:"subQueries"`
	Dependencies  map[string][]string    `json:"dependencies"`
	MergeStrategy MergeStrategy          `json:"mergeStrategy"`
	Metadata      map[string]interface{} `json:"metadata,omitempty"`
}

// SubQuery 表示子查询
type SubQuery struct {
	ServiceName   string                 `json:"serviceName"`
	Query         string                 `json:"query"`
	OperationName string                 `json:"operationName,omitempty"`
	Variables     map[string]interface{} `json:"variables,omitempty"`
	Path          []string               `json:"path"`
	Headers       map[string]string      `json:"headers,omitempty"`
	Timeout       time.Duration          `json:"timeout"`
	RetryCount    int                    `json:"retryCount,omitempty"`
}

// ServiceConfig 表示服务配置
type ServiceConfig struct {
	Name        string            `json:"name"`
	Endpoint    string            `json:"endpoint"`
	Path        string            `json:"path,omitempty"` // GraphQL端点路径，默认为/graphql
	Schema      string            `json:"schema"`
	Weight      int               `json:"weight,omitempty"`
	Timeout     time.Duration     `json:"timeout"`
	MaxRetries  int               `json:"maxRetries,omitempty"`
	Headers     map[string]string `json:"headers,omitempty"`
	HealthCheck *HealthCheck      `json:"healthCheck,omitempty"`
}

// HealthCheck 表示健康检查配置
type HealthCheck struct {
	Enabled  bool          `json:"enabled"`
	Interval time.Duration `json:"interval"`
	Timeout  time.Duration `json:"timeout"`
	Path     string        `json:"path"`
}

// FederationConfig 表示联邦配置
type FederationConfig struct {
	Services         []ServiceConfig `json:"services"`
	EnableQueryPlan  bool            `json:"enableQueryPlanning"`
	EnableCaching    bool            `json:"enableCaching"`
	MaxQueryDepth    int             `json:"maxQueryDepth"`
	QueryTimeout     time.Duration   `json:"queryTimeout"`
	EnableIntrospect bool            `json:"enableIntrospection"`
	DebugMode        bool            `json:"debugMode"`
}

// GraphQLRequest 表示 GraphQL 请求
type GraphQLRequest struct {
	Query         string                 `json:"query"`
	Variables     map[string]interface{} `json:"variables,omitempty"`
	OperationName string                 `json:"operationName,omitempty"`
}

// GraphQLResponse 表示 GraphQL 响应
type GraphQLResponse struct {
	Data       interface{}            `json:"data,omitempty"`
	Errors     []GraphQLError         `json:"errors,omitempty"`
	Extensions map[string]interface{} `json:"extensions,omitempty"`
}

// GraphQLError 表示 GraphQL 错误
type GraphQLError struct {
	Message    string                 `json:"message"`
	Locations  []ErrorLocation        `json:"locations,omitempty"`
	Path       []interface{}          `json:"path,omitempty"`
	Extensions map[string]interface{} `json:"extensions,omitempty"`
}

// ErrorLocation 表示错误位置
type ErrorLocation struct {
	Line   int `json:"line"`
	Column int `json:"column"`
}

// MergeStrategy 表示合并策略
type MergeStrategy string

const (
	MergeStrategyDeep    MergeStrategy = "deep"
	MergeStrategyShallow MergeStrategy = "shallow"
	MergeStrategyCustom  MergeStrategy = "custom"
)

// ServiceCall 表示服务调用
type ServiceCall struct {
	Service   *ServiceConfig
	SubQuery  *SubQuery
	Context   *QueryContext
	StartTime time.Time
}

// ServiceResponse 表示服务响应
type ServiceResponse struct {
	Data       interface{}            `json:"data,omitempty"`
	Errors     []GraphQLError         `json:"errors,omitempty"`
	Metadata   map[string]interface{} `json:"metadata,omitempty"`
	Service    string                 `json:"service"`
	Latency    time.Duration          `json:"latency"`
	Error      error                  `json:"-"`
	StatusCode int                    `json:"statusCode"`
	Headers    map[string]string      `json:"headers,omitempty"`
}

// ExecutionContext 表示执行上下文
type ExecutionContext struct {
	RequestID    string
	QueryContext *QueryContext
	Plan         *ExecutionPlan
	StartTime    time.Time
	Config       *FederationConfig
	Metrics      *Metrics
}

// Metrics 表示性能指标
type Metrics struct {
	QueryLatency     time.Duration
	ServiceCallCount int
	ErrorCount       int
	CacheHitCount    int
	CacheMissCount   int
}

// SchemaInfo 表示模式信息
type SchemaInfo struct {
	ServiceName string
	Schema      string
	Version     string
	Types       []TypeInfo
	UpdatedAt   time.Time
}

// TypeInfo 表示类型信息
type TypeInfo struct {
	Name   string
	Kind   string
	Fields []FieldInfo
}

// FieldInfo 表示字段信息
type FieldInfo struct {
	Name string
	Type string
	Args []ArgumentInfo
}

// ArgumentInfo 表示参数信息
type ArgumentInfo struct {
	Name string
	Type string
}

// CacheEntry 表示缓存条目
type CacheEntry struct {
	Key       string
	Value     interface{}
	ExpiresAt time.Time
	CreatedAt time.Time
}

// FilterState 表示过滤器状态
type FilterState int

const (
	FilterStateUninitialized FilterState = iota
	FilterStateInitialized
	FilterStateProcessing
	FilterStateComplete
	FilterStateError
)

// Federation 指令相关类型定义

// FederationDirective 表示 Federation 指令类型
type FederationDirective string

const (
	DirectiveKey      FederationDirective = "key"
	DirectiveExternal FederationDirective = "external"
	DirectiveRequires FederationDirective = "requires"
	DirectiveProvides FederationDirective = "provides"
	DirectiveExtends  FederationDirective = "extends"
)

// KeyDirective 表示 @key 指令
type KeyDirective struct {
	Fields     string `json:"fields"`     // 键字段选择集
	Resolvable bool   `json:"resolvable"` // 是否可解析，默认为 true
}

// ExternalDirective 表示 @external 指令
type ExternalDirective struct {
	Reason string `json:"reason,omitempty"` // 外部字段的原因说明
}

// RequiresDirective 表示 @requires 指令
type RequiresDirective struct {
	Fields string `json:"fields"` // 必需字段选择集
}

// ProvidesDirective 表示 @provides 指令
type ProvidesDirective struct {
	Fields string `json:"fields"` // 提供字段选择集
}

// EntityDirectives 表示实体上的指令集合
type EntityDirectives struct {
	Keys     []KeyDirective     `json:"keys,omitempty"`
	External *ExternalDirective `json:"external,omitempty"`
	Requires *RequiresDirective `json:"requires,omitempty"`
	Provides *ProvidesDirective `json:"provides,omitempty"`
}

// FederatedEntity 表示联邦实体
type FederatedEntity struct {
	TypeName    string           `json:"typeName"`
	ServiceName string           `json:"serviceName"`
	Directives  EntityDirectives `json:"directives"`
	Fields      []FederatedField `json:"fields"`
}

// FederatedField 表示联邦字段
type FederatedField struct {
	Name       string           `json:"name"`
	Type       string           `json:"type"`
	Directives EntityDirectives `json:"directives"`
	Arguments  []ArgumentInfo   `json:"arguments,omitempty"`
}

// FederatedSchema 表示联邦模式
type FederatedSchema struct {
	Services []ServiceSchema   `json:"services"`
	Entities []FederatedEntity `json:"entities"`
	Version  string            `json:"version"`
}

// ServiceSchema 表示服务模式
type ServiceSchema struct {
	ServiceName string            `json:"serviceName"`
	SDL         string            `json:"sdl"`
	Entities    []FederatedEntity `json:"entities"`
	Types       []TypeInfo        `json:"types"`
}

// RepresentationRequest 表示实体表示请求
type RepresentationRequest struct {
	TypeName       string                 `json:"__typename"`
	Representation map[string]interface{} `json:"representation"`
}

// EntityResolution 表示实体解析信息
type EntityResolution struct {
	TypeName    string   `json:"typeName"`
	ServiceName string   `json:"serviceName"`
	KeyFields   []string `json:"keyFields"`
	Query       string   `json:"query"`
}

// FederationPlan 表示联邦执行计划
type FederationPlan struct {
	Entities         []EntityResolution      `json:"entities"`
	Representations  []RepresentationRequest `json:"representations"`
	RequiredServices []string                `json:"requiredServices"`
	DependencyOrder  []string                `json:"dependencyOrder"`
}
