package errors

import (
	"fmt"
	"net/http"
)

// ErrorCode 定义错误代码类型
type ErrorCode string

// StackFrame 堆栈帧信息
type StackFrame struct {
	Function string `json:"function"`
	File     string `json:"file"`
	Line     int    `json:"line"`
	Package  string `json:"package"`
}

// captureStackTrace 捕获堆栈跟踪（TinyGo兼容版本）
func captureStackTrace(skip int) []StackFrame {
	// TinyGo不支持runtime.Caller，返回简化的堆栈信息
	return []StackFrame{
		{
			Function: "unknown",
			File:     "wasm",
			Line:     0,
			Package:  "envoy-wasm-graphql-federation",
		},
	}
}

// getSeverityForCode 根据错误代码获取严重程度
func getSeverityForCode(code ErrorCode) string {
	switch code {
	case ErrCodeInternal, ErrCodeConfigInvalid, ErrCodeSchemaInvalid:
		return "critical"
	case ErrCodeServiceCall, ErrCodeTimeout, ErrCodeUnavailable:
		return "high"
	case ErrCodeQueryParsing, ErrCodeQueryValidation, ErrCodeQueryComplexity:
		return "medium"
	default:
		return "low"
	}
}

// getCategoryForCode 根据错误代码获取分类
func getCategoryForCode(code ErrorCode) string {
	switch code {
	case ErrCodeQueryParsing, ErrCodeQueryValidation, ErrCodeQueryComplexity:
		return "user"
	case ErrCodeServiceCall, ErrCodeTimeout, ErrCodeUnavailable, ErrCodeServiceNotFound:
		return "external"
	case ErrCodeConfigInvalid, ErrCodeSchemaInvalid:
		return "system"
	case ErrCodeInternal:
		return "system"
	default:
		return "unknown"
	}
}

// isRetryableCode 判断错误代码是否可重试
func isRetryableCode(code ErrorCode) bool {
	switch code {
	case ErrCodeTimeout, ErrCodeUnavailable, ErrCodeServiceCall:
		return true
	case ErrCodeRateLimit:
		return true // 可以稍后重试
	default:
		return false
	}
}

const (
	// 解析错误
	ErrCodeQueryParsing    ErrorCode = "QUERY_PARSING_ERROR"
	ErrCodeQueryValidation ErrorCode = "QUERY_VALIDATION_ERROR"
	ErrCodeQueryComplexity ErrorCode = "QUERY_COMPLEXITY_ERROR"

	// 执行错误
	ErrCodePlanningFailed  ErrorCode = "PLANNING_FAILED"
	ErrCodeExecutionFailed ErrorCode = "EXECUTION_FAILED"
	ErrCodeServiceCall     ErrorCode = "SERVICE_CALL_ERROR"
	ErrCodeTimeout         ErrorCode = "TIMEOUT_ERROR"

	// 配置错误
	ErrCodeConfigInvalid   ErrorCode = "CONFIG_INVALID"
	ErrCodeSchemaInvalid   ErrorCode = "SCHEMA_INVALID"
	ErrCodeServiceNotFound ErrorCode = "SERVICE_NOT_FOUND"

	// 系统错误
	ErrCodeInternal    ErrorCode = "INTERNAL_ERROR"
	ErrCodeUnavailable ErrorCode = "SERVICE_UNAVAILABLE"
	ErrCodeRateLimit   ErrorCode = "RATE_LIMIT_EXCEEDED"

	// Federation 相关错误
	ErrCodeDirectiveParsing ErrorCode = "DIRECTIVE_PARSING_ERROR"
	ErrCodeEntityResolution ErrorCode = "ENTITY_RESOLUTION_ERROR"
	ErrCodeDataExtraction   ErrorCode = "DATA_EXTRACTION_ERROR"
	ErrCodeQueryBuilding    ErrorCode = "QUERY_BUILDING_ERROR"
	ErrCodeValidation       ErrorCode = "VALIDATION_ERROR"
	ErrCodeParsing          ErrorCode = "PARSING_ERROR"
	ErrCodeResolution       ErrorCode = "RESOLUTION_ERROR"
)

// FederationError 表示联邦错误
type FederationError struct {
	Code       ErrorCode              `json:"code"`
	Message    string                 `json:"message"`
	Service    string                 `json:"service,omitempty"`
	Path       []interface{}          `json:"path,omitempty"`
	Locations  []ErrorLocation        `json:"locations,omitempty"`
	Extensions map[string]interface{} `json:"extensions,omitempty"`
	Cause      error                  `json:"-"`
}

// ErrorLocation 表示错误位置
type ErrorLocation struct {
	Line   int `json:"line"`
	Column int `json:"column"`
}

// Error 实现 error 接口
func (e *FederationError) Error() string {
	if e.Service != "" {
		return fmt.Sprintf("[%s] %s (service: %s)", e.Code, e.Message, e.Service)
	}
	return fmt.Sprintf("[%s] %s", e.Code, e.Message)
}

// ToGraphQLError 转换为 GraphQL 错误格式
func (e *FederationError) ToGraphQLError() map[string]interface{} {
	result := map[string]interface{}{
		"message": e.Message,
	}

	if len(e.Locations) > 0 {
		result["locations"] = e.Locations
	}

	if len(e.Path) > 0 {
		result["path"] = e.Path
	}

	extensions := make(map[string]interface{})
	extensions["code"] = string(e.Code)

	if e.Service != "" {
		extensions["service"] = e.Service
	}

	for k, v := range e.Extensions {
		extensions[k] = v
	}

	if len(extensions) > 0 {
		result["extensions"] = extensions
	}

	return result
}

// NewFederationError 创建新的联邦错误
func NewFederationError(code ErrorCode, message string, opts ...ErrorOption) *FederationError {
	err := &FederationError{
		Code:       code,
		Message:    message,
		Extensions: make(map[string]interface{}),
	}

	for _, opt := range opts {
		opt(err)
	}

	return err
}

// ErrorOption 定义错误选项函数类型
type ErrorOption func(*FederationError)

// WithService 设置服务名称
func WithService(service string) ErrorOption {
	return func(e *FederationError) {
		e.Service = service
	}
}

// WithPath 设置错误路径
func WithPath(path ...interface{}) ErrorOption {
	return func(e *FederationError) {
		e.Path = path
	}
}

// WithLocation 设置错误位置
func WithLocation(line, column int) ErrorOption {
	return func(e *FederationError) {
		e.Locations = append(e.Locations, ErrorLocation{
			Line:   line,
			Column: column,
		})
	}
}

// WithCause 设置原始错误
func WithCause(cause error) ErrorOption {
	return func(e *FederationError) {
		e.Cause = cause
		if e.Extensions == nil {
			e.Extensions = make(map[string]interface{})
		}
		e.Extensions["originalError"] = cause.Error()
	}
}

// WithExtension 添加扩展字段
func WithExtension(key string, value interface{}) ErrorOption {
	return func(e *FederationError) {
		if e.Extensions == nil {
			e.Extensions = make(map[string]interface{})
		}
		e.Extensions[key] = value
	}
}

// 预定义错误构造函数

// NewQueryParsingError 创建查询解析错误
func NewQueryParsingError(message string, opts ...ErrorOption) *FederationError {
	return NewFederationError(ErrCodeQueryParsing, message, opts...)
}

// NewQueryValidationError 创建查询验证错误
func NewQueryValidationError(message string, opts ...ErrorOption) *FederationError {
	return NewFederationError(ErrCodeQueryValidation, message, opts...)
}

// NewQueryComplexityError 创建查询复杂度错误
func NewQueryComplexityError(message string, opts ...ErrorOption) *FederationError {
	return NewFederationError(ErrCodeQueryComplexity, message, opts...)
}

// NewPlanningError 创建规划错误
func NewPlanningError(message string, opts ...ErrorOption) *FederationError {
	return NewFederationError(ErrCodePlanningFailed, message, opts...)
}

// NewExecutionError 创建执行错误
func NewExecutionError(message string, opts ...ErrorOption) *FederationError {
	return NewFederationError(ErrCodeExecutionFailed, message, opts...)
}

// NewServiceCallError 创建服务调用错误
func NewServiceCallError(service string, message string, opts ...ErrorOption) *FederationError {
	opts = append(opts, WithService(service))
	return NewFederationError(ErrCodeServiceCall, message, opts...)
}

// NewTimeoutError 创建超时错误
func NewTimeoutError(service string, message string, opts ...ErrorOption) *FederationError {
	opts = append(opts, WithService(service))
	return NewFederationError(ErrCodeTimeout, message, opts...)
}

// NewConfigError 创建配置错误
func NewConfigError(message string, opts ...ErrorOption) *FederationError {
	return NewFederationError(ErrCodeConfigInvalid, message, opts...)
}

// NewSchemaError 创建模式错误
func NewSchemaError(message string, opts ...ErrorOption) *FederationError {
	return NewFederationError(ErrCodeSchemaInvalid, message, opts...)
}

// NewServiceNotFoundError 创建服务未找到错误
func NewServiceNotFoundError(service string, opts ...ErrorOption) *FederationError {
	message := fmt.Sprintf("Service '%s' not found", service)
	opts = append(opts, WithService(service))
	return NewFederationError(ErrCodeServiceNotFound, message, opts...)
}

// NewInternalError 创建内部错误
func NewInternalError(message string, opts ...ErrorOption) *FederationError {
	return NewFederationError(ErrCodeInternal, message, opts...)
}

// NewUnavailableError 创建服务不可用错误
func NewUnavailableError(service string, message string, opts ...ErrorOption) *FederationError {
	opts = append(opts, WithService(service))
	return NewFederationError(ErrCodeUnavailable, message, opts...)
}

// NewRateLimitError 创建速率限制错误
func NewRateLimitError(message string, opts ...ErrorOption) *FederationError {
	return NewFederationError(ErrCodeRateLimit, message, opts...)
}

// NewServiceError 创建服务错误
func NewServiceError(message string, opts ...ErrorOption) *FederationError {
	return NewFederationError(ErrCodeServiceCall, message, opts...)
}

// NewMergeError 创建合并错误
func NewMergeError(message string, opts ...ErrorOption) *FederationError {
	return NewFederationError(ErrCodeExecutionFailed, message, opts...)
}

// NewBatchError 创建批处理错误
func NewBatchError(message string, opts ...ErrorOption) *FederationError {
	return NewFederationError(ErrCodeExecutionFailed, message, opts...)
}

// NewDirectiveParsingError 创建指令解析错误
func NewDirectiveParsingError(message string, opts ...ErrorOption) *FederationError {
	return NewFederationError(ErrCodeDirectiveParsing, message, opts...)
}

// NewEntityResolutionError 创建实体解析错误
func NewEntityResolutionError(message string, opts ...ErrorOption) *FederationError {
	return NewFederationError(ErrCodeEntityResolution, message, opts...)
}

// NewDataExtractionError 创建数据提取错误
func NewDataExtractionError(message string, opts ...ErrorOption) *FederationError {
	return NewFederationError(ErrCodeDataExtraction, message, opts...)
}

// NewQueryBuildingError 创建查询构建错误
func NewQueryBuildingError(message string, opts ...ErrorOption) *FederationError {
	return NewFederationError(ErrCodeQueryBuilding, message, opts...)
}

// NewValidationError 创建验证错误
func NewValidationError(message string, opts ...ErrorOption) *FederationError {
	return NewFederationError(ErrCodeValidation, message, opts...)
}

// NewParsingError 创建解析错误
func NewParsingError(message string, opts ...ErrorOption) *FederationError {
	return NewFederationError(ErrCodeParsing, message, opts...)
}

// NewResolutionError 创建解析错误
func NewResolutionError(message string, opts ...ErrorOption) *FederationError {
	return NewFederationError(ErrCodeResolution, message, opts...)
}

// IsRetryableError 判断错误是否可重试
func IsRetryableError(err error) bool {
	if fedErr, ok := err.(*FederationError); ok {
		switch fedErr.Code {
		case ErrCodeTimeout, ErrCodeUnavailable:
			return true
		case ErrCodeServiceCall:
			// 检查 HTTP 状态码
			if statusCode, exists := fedErr.Extensions["statusCode"]; exists {
				if code, ok := statusCode.(int); ok {
					return code >= 500 || code == http.StatusTooManyRequests
				}
			}
			return true
		}
	}
	return false
}

// GetErrorSeverity 获取错误严重程度
func GetErrorSeverity(err error) string {
	if fedErr, ok := err.(*FederationError); ok {
		switch fedErr.Code {
		case ErrCodeInternal, ErrCodeConfigInvalid, ErrCodeSchemaInvalid:
			return "HIGH"
		case ErrCodeServiceCall, ErrCodeTimeout, ErrCodeUnavailable:
			return "MEDIUM"
		case ErrCodeQueryParsing, ErrCodeQueryValidation, ErrCodeQueryComplexity:
			return "LOW"
		default:
			return "MEDIUM"
		}
	}
	return "UNKNOWN"
}

// AggregateErrors 聚合多个错误
func AggregateErrors(errors []*FederationError) []*FederationError {
	if len(errors) == 0 {
		return nil
	}

	// 去重和合并相似错误
	errorMap := make(map[string]*FederationError)

	for _, err := range errors {
		key := fmt.Sprintf("%s:%s:%s", err.Code, err.Service, err.Message)
		if existing, exists := errorMap[key]; exists {
			// 合并路径和位置信息
			existing.Path = append(existing.Path, err.Path...)
			existing.Locations = append(existing.Locations, err.Locations...)
		} else {
			errorMap[key] = err
		}
	}

	result := make([]*FederationError, 0, len(errorMap))
	for _, err := range errorMap {
		result = append(result, err)
	}

	return result
}

// ErrorCollector 错误收集器
type ErrorCollector struct {
	errors []*FederationError
}

// NewErrorCollector 创建新的错误收集器
func NewErrorCollector() *ErrorCollector {
	return &ErrorCollector{
		errors: make([]*FederationError, 0),
	}
}

// Add 添加错误
func (ec *ErrorCollector) Add(err *FederationError) {
	if err != nil {
		ec.errors = append(ec.errors, err)
	}
}

// AddError 添加普通错误
func (ec *ErrorCollector) AddError(err error) {
	if err != nil {
		if fedErr, ok := err.(*FederationError); ok {
			ec.Add(fedErr)
		} else {
			ec.Add(NewInternalError(err.Error()))
		}
	}
}

// HasErrors 检查是否有错误
func (ec *ErrorCollector) HasErrors() bool {
	return len(ec.errors) > 0
}

// Count 获取错误数量
func (ec *ErrorCollector) Count() int {
	return len(ec.errors)
}

// GetErrors 获取所有错误
func (ec *ErrorCollector) GetErrors() []*FederationError {
	return AggregateErrors(ec.errors)
}

// ToGraphQLErrors 转换为GraphQL错误格式
func (ec *ErrorCollector) ToGraphQLErrors() []map[string]interface{} {
	errors := ec.GetErrors()
	result := make([]map[string]interface{}, len(errors))
	for i, err := range errors {
		result[i] = err.ToGraphQLError()
	}
	return result
}

// Clear 清空错误
func (ec *ErrorCollector) Clear() {
	ec.errors = ec.errors[:0]
}

// ErrorHandler 错误处理器接口
type ErrorHandler interface {
	HandleError(err *FederationError) error
	CanHandle(err *FederationError) bool
	GetPriority() int
}

// ErrorRegistry 错误处理器注册表
type ErrorRegistry struct {
	handlers []ErrorHandler
}

// NewErrorRegistry 创建错误处理器注册表
func NewErrorRegistry() *ErrorRegistry {
	return &ErrorRegistry{
		handlers: make([]ErrorHandler, 0),
	}
}

// RegisterHandler 注册错误处理器
func (er *ErrorRegistry) RegisterHandler(handler ErrorHandler) {
	er.handlers = append(er.handlers, handler)
	// 按优先级排序
	for i := len(er.handlers) - 1; i > 0; i-- {
		if er.handlers[i].GetPriority() > er.handlers[i-1].GetPriority() {
			er.handlers[i], er.handlers[i-1] = er.handlers[i-1], er.handlers[i]
		} else {
			break
		}
	}
}

// HandleError 处理错误
func (er *ErrorRegistry) HandleError(err *FederationError) error {
	for _, handler := range er.handlers {
		if handler.CanHandle(err) {
			return handler.HandleError(err)
		}
	}
	return err // 无处理器能处理时返回原错误
}

// RecoveryHandler 恢复处理器
type RecoveryHandler struct {
	collector *ErrorCollector
}

// NewRecoveryHandler 创建恢复处理器
func NewRecoveryHandler() *RecoveryHandler {
	return &RecoveryHandler{
		collector: NewErrorCollector(),
	}
}

// Recover 恢复函数
func (rh *RecoveryHandler) Recover() {
	if r := recover(); r != nil {
		switch v := r.(type) {
		case *FederationError:
			rh.collector.Add(v)
		case error:
			rh.collector.AddError(v)
		default:
			rh.collector.Add(NewInternalError(fmt.Sprintf("panic: %v", r)))
		}
	}
}

// GetErrors 获取恢复过程中收集的错误
func (rh *RecoveryHandler) GetErrors() []*FederationError {
	return rh.collector.GetErrors()
}

// HasErrors 检查是否有错误
func (rh *RecoveryHandler) HasErrors() bool {
	return rh.collector.HasErrors()
}

// ErrorContext 错误上下文
type ErrorContext struct {
	RequestID string                 `json:"requestId,omitempty"`
	Operation string                 `json:"operation,omitempty"`
	Service   string                 `json:"service,omitempty"`
	Query     string                 `json:"query,omitempty"`
	Variables map[string]interface{} `json:"variables,omitempty"`
	Headers   map[string]string      `json:"headers,omitempty"`
	Timestamp string                 `json:"timestamp,omitempty"`
	UserAgent string                 `json:"userAgent,omitempty"`
	ClientIP  string                 `json:"clientIp,omitempty"`
}

// EnrichError 丰富错误信息
func EnrichError(err *FederationError, ctx *ErrorContext) *FederationError {
	if err == nil || ctx == nil {
		return err
	}

	if err.Extensions == nil {
		err.Extensions = make(map[string]interface{})
	}

	err.Extensions["context"] = ctx

	if ctx.RequestID != "" {
		err.Extensions["requestId"] = ctx.RequestID
	}

	if ctx.Operation != "" {
		err.Extensions["operation"] = ctx.Operation
	}

	if ctx.Service != "" && err.Service == "" {
		err.Service = ctx.Service
	}

	return err
}

// SanitizeError 清理错误信息（移除敏感信息）
func SanitizeError(err *FederationError) *FederationError {
	if err == nil {
		return nil
	}

	// 创建副本
	sanitized := &FederationError{
		Code:      err.Code,
		Message:   err.Message,
		Service:   err.Service,
		Path:      err.Path,
		Locations: err.Locations,
	}

	// 过滤敏感扩展字段
	if err.Extensions != nil {
		sanitized.Extensions = make(map[string]interface{})
		allowedKeys := []string{"code", "service", "timestamp", "requestId"}
		for _, key := range allowedKeys {
			if val, exists := err.Extensions[key]; exists {
				sanitized.Extensions[key] = val
			}
		}
	}

	return sanitized
}
