package errors

import (
	"testing"
)

func TestErrorCodeConstants(t *testing.T) {
	// 解析错误
	if ErrCodeQueryParsing != "QUERY_PARSING_ERROR" {
		t.Errorf("Expected ErrCodeQueryParsing to be 'QUERY_PARSING_ERROR', got %s", ErrCodeQueryParsing)
	}

	if ErrCodeQueryValidation != "QUERY_VALIDATION_ERROR" {
		t.Errorf("Expected ErrCodeQueryValidation to be 'QUERY_VALIDATION_ERROR', got %s", ErrCodeQueryValidation)
	}

	if ErrCodeQueryComplexity != "QUERY_COMPLEXITY_ERROR" {
		t.Errorf("Expected ErrCodeQueryComplexity to be 'QUERY_COMPLEXITY_ERROR', got %s", ErrCodeQueryComplexity)
	}

	// 执行错误
	if ErrCodePlanningFailed != "PLANNING_FAILED" {
		t.Errorf("Expected ErrCodePlanningFailed to be 'PLANNING_FAILED', got %s", ErrCodePlanningFailed)
	}

	if ErrCodeExecutionFailed != "EXECUTION_FAILED" {
		t.Errorf("Expected ErrCodeExecutionFailed to be 'EXECUTION_FAILED', got %s", ErrCodeExecutionFailed)
	}

	if ErrCodeServiceCall != "SERVICE_CALL_ERROR" {
		t.Errorf("Expected ErrCodeServiceCall to be 'SERVICE_CALL_ERROR', got %s", ErrCodeServiceCall)
	}

	if ErrCodeTimeout != "TIMEOUT_ERROR" {
		t.Errorf("Expected ErrCodeTimeout to be 'TIMEOUT_ERROR', got %s", ErrCodeTimeout)
	}

	// 配置错误
	if ErrCodeConfigInvalid != "CONFIG_INVALID" {
		t.Errorf("Expected ErrCodeConfigInvalid to be 'CONFIG_INVALID', got %s", ErrCodeConfigInvalid)
	}

	if ErrCodeSchemaInvalid != "SCHEMA_INVALID" {
		t.Errorf("Expected ErrCodeSchemaInvalid to be 'SCHEMA_INVALID', got %s", ErrCodeSchemaInvalid)
	}

	if ErrCodeServiceNotFound != "SERVICE_NOT_FOUND" {
		t.Errorf("Expected ErrCodeServiceNotFound to be 'SERVICE_NOT_FOUND', got %s", ErrCodeServiceNotFound)
	}

	// 系统错误
	if ErrCodeInternal != "INTERNAL_ERROR" {
		t.Errorf("Expected ErrCodeInternal to be 'INTERNAL_ERROR', got %s", ErrCodeInternal)
	}

	if ErrCodeUnavailable != "SERVICE_UNAVAILABLE" {
		t.Errorf("Expected ErrCodeUnavailable to be 'SERVICE_UNAVAILABLE', got %s", ErrCodeUnavailable)
	}

	if ErrCodeRateLimit != "RATE_LIMIT_EXCEEDED" {
		t.Errorf("Expected ErrCodeRateLimit to be 'RATE_LIMIT_EXCEEDED', got %s", ErrCodeRateLimit)
	}

	// Federation 相关错误
	if ErrCodeDirectiveParsing != "DIRECTIVE_PARSING_ERROR" {
		t.Errorf("Expected ErrCodeDirectiveParsing to be 'DIRECTIVE_PARSING_ERROR', got %s", ErrCodeDirectiveParsing)
	}

	if ErrCodeEntityResolution != "ENTITY_RESOLUTION_ERROR" {
		t.Errorf("Expected ErrCodeEntityResolution to be 'ENTITY_RESOLUTION_ERROR', got %s", ErrCodeEntityResolution)
	}
}

func TestFederationError_Struct(t *testing.T) {
	err := &FederationError{
		Code:    ErrCodeQueryParsing,
		Message: "Failed to parse query",
		Service: "users-service",
		Path:    []interface{}{"user", "profile"},
		Locations: []ErrorLocation{
			{Line: 1, Column: 5},
		},
		Extensions: map[string]interface{}{
			"details": "Invalid syntax",
		},
	}

	if err.Code != ErrCodeQueryParsing {
		t.Errorf("Expected Code to be ErrCodeQueryParsing, got %s", err.Code)
	}

	if err.Message != "Failed to parse query" {
		t.Errorf("Expected Message to match, got %s", err.Message)
	}

	if err.Service != "users-service" {
		t.Errorf("Expected Service to be 'users-service', got %s", err.Service)
	}

	if len(err.Path) != 2 {
		t.Errorf("Expected Path to have 2 elements, got %d", len(err.Path))
	}

	if len(err.Locations) != 1 {
		t.Errorf("Expected Locations to have 1 element, got %d", len(err.Locations))
	}

	if err.Locations[0].Line != 1 {
		t.Errorf("Expected Line to be 1, got %d", err.Locations[0].Line)
	}

	if details, ok := err.Extensions["details"]; !ok || details != "Invalid syntax" {
		t.Errorf("Expected Extensions to contain 'details' with value 'Invalid syntax', got %v", err.Extensions)
	}
}

func TestFederationError_Error(t *testing.T) {
	// 带服务名称的错误
	errWithService := &FederationError{
		Code:    ErrCodeQueryParsing,
		Message: "Failed to parse query",
		Service: "users-service",
	}

	expectedWithService := "[QUERY_PARSING_ERROR] Failed to parse query (service: users-service)"
	if errWithService.Error() != expectedWithService {
		t.Errorf("Expected error message '%s', got '%s'", expectedWithService, errWithService.Error())
	}

	// 不带服务名称的错误
	errWithoutService := &FederationError{
		Code:    ErrCodeQueryParsing,
		Message: "Failed to parse query",
	}

	expectedWithoutService := "[QUERY_PARSING_ERROR] Failed to parse query"
	if errWithoutService.Error() != expectedWithoutService {
		t.Errorf("Expected error message '%s', got '%s'", expectedWithoutService, errWithoutService.Error())
	}
}

func TestFederationError_ToGraphQLError(t *testing.T) {
	err := &FederationError{
		Code:    ErrCodeQueryParsing,
		Message: "Failed to parse query",
		Service: "users-service",
		Path:    []interface{}{"user", "profile"},
		Locations: []ErrorLocation{
			{Line: 1, Column: 5},
		},
		Extensions: map[string]interface{}{
			"details": "Invalid syntax",
		},
	}

	graphQLError := err.ToGraphQLError()

	if message, ok := graphQLError["message"]; !ok || message != "Failed to parse query" {
		t.Errorf("Expected message to be 'Failed to parse query', got %v", message)
	}

	if locations, ok := graphQLError["locations"]; !ok {
		t.Error("Expected locations to be present")
	} else if locs, ok := locations.([]ErrorLocation); !ok || len(locs) != 1 {
		t.Errorf("Expected locations to have 1 element, got %v", locations)
	}

	if path, ok := graphQLError["path"]; !ok {
		t.Error("Expected path to be present")
	} else if p, ok := path.([]interface{}); !ok || len(p) != 2 {
		t.Errorf("Expected path to have 2 elements, got %v", path)
	}

	if extensions, ok := graphQLError["extensions"]; !ok {
		t.Error("Expected extensions to be present")
	} else if exts, ok := extensions.(map[string]interface{}); !ok {
		t.Errorf("Expected extensions to be a map, got %v", extensions)
	} else {
		if code, ok := exts["code"]; !ok || code != string(ErrCodeQueryParsing) {
			t.Errorf("Expected code to be 'QUERY_PARSING_ERROR', got %v", code)
		}

		if service, ok := exts["service"]; !ok || service != "users-service" {
			t.Errorf("Expected service to be 'users-service', got %v", service)
		}

		if details, ok := exts["details"]; !ok || details != "Invalid syntax" {
			t.Errorf("Expected details to be 'Invalid syntax', got %v", details)
		}
	}
}

func TestNewFederationError(t *testing.T) {
	err := NewFederationError(ErrCodeQueryParsing, "Failed to parse query")

	if err == nil {
		t.Fatal("NewFederationError() returned nil")
	}

	if err.Code != ErrCodeQueryParsing {
		t.Errorf("Expected Code to be ErrCodeQueryParsing, got %s", err.Code)
	}

	if err.Message != "Failed to parse query" {
		t.Errorf("Expected Message to match, got %s", err.Message)
	}

	if err.Extensions == nil {
		t.Error("Expected Extensions to be initialized")
	}
}

func TestWithService(t *testing.T) {
	err := NewFederationError(ErrCodeQueryParsing, "Failed to parse query", WithService("users-service"))

	if err.Service != "users-service" {
		t.Errorf("Expected Service to be 'users-service', got %s", err.Service)
	}
}

func TestWithPath(t *testing.T) {
	err := NewFederationError(ErrCodeQueryParsing, "Failed to parse query", WithPath("user", "profile"))

	if len(err.Path) != 2 {
		t.Errorf("Expected Path to have 2 elements, got %d", len(err.Path))
	}

	if err.Path[0] != "user" {
		t.Errorf("Expected first path element to be 'user', got %v", err.Path[0])
	}

	if err.Path[1] != "profile" {
		t.Errorf("Expected second path element to be 'profile', got %v", err.Path[1])
	}
}

func TestWithLocation(t *testing.T) {
	err := NewFederationError(ErrCodeQueryParsing, "Failed to parse query", WithLocation(1, 5))

	if len(err.Locations) != 1 {
		t.Errorf("Expected Locations to have 1 element, got %d", len(err.Locations))
	}

	if err.Locations[0].Line != 1 {
		t.Errorf("Expected Line to be 1, got %d", err.Locations[0].Line)
	}

	if err.Locations[0].Column != 5 {
		t.Errorf("Expected Column to be 5, got %d", err.Locations[0].Column)
	}
}

func TestErrorLocation_Struct(t *testing.T) {
	location := &ErrorLocation{
		Line:   10,
		Column: 5,
	}

	if location.Line != 10 {
		t.Errorf("Expected Line to be 10, got %d", location.Line)
	}

	if location.Column != 5 {
		t.Errorf("Expected Column to be 5, got %d", location.Column)
	}
}

func TestGetSeverityForCode(t *testing.T) {
	// 测试关键错误
	if getSeverityForCode(ErrCodeInternal) != "critical" {
		t.Errorf("Expected severity for ErrCodeInternal to be 'critical'")
	}

	if getSeverityForCode(ErrCodeConfigInvalid) != "critical" {
		t.Errorf("Expected severity for ErrCodeConfigInvalid to be 'critical'")
	}

	if getSeverityForCode(ErrCodeSchemaInvalid) != "critical" {
		t.Errorf("Expected severity for ErrCodeSchemaInvalid to be 'critical'")
	}

	// 测试高严重性错误
	if getSeverityForCode(ErrCodeServiceCall) != "high" {
		t.Errorf("Expected severity for ErrCodeServiceCall to be 'high'")
	}

	if getSeverityForCode(ErrCodeTimeout) != "high" {
		t.Errorf("Expected severity for ErrCodeTimeout to be 'high'")
	}

	if getSeverityForCode(ErrCodeUnavailable) != "high" {
		t.Errorf("Expected severity for ErrCodeUnavailable to be 'high'")
	}

	// 测试中等严重性错误
	if getSeverityForCode(ErrCodeQueryParsing) != "medium" {
		t.Errorf("Expected severity for ErrCodeQueryParsing to be 'medium'")
	}

	if getSeverityForCode(ErrCodeQueryValidation) != "medium" {
		t.Errorf("Expected severity for ErrCodeQueryValidation to be 'medium'")
	}

	if getSeverityForCode(ErrCodeQueryComplexity) != "medium" {
		t.Errorf("Expected severity for ErrCodeQueryComplexity to be 'medium'")
	}

	// 测试低严重性错误
	if getSeverityForCode("UNKNOWN_ERROR") != "low" {
		t.Errorf("Expected severity for unknown error to be 'low'")
	}
}

func TestGetCategoryForCode(t *testing.T) {
	// 测试用户错误
	if getCategoryForCode(ErrCodeQueryParsing) != "user" {
		t.Errorf("Expected category for ErrCodeQueryParsing to be 'user'")
	}

	if getCategoryForCode(ErrCodeQueryValidation) != "user" {
		t.Errorf("Expected category for ErrCodeQueryValidation to be 'user'")
	}

	if getCategoryForCode(ErrCodeQueryComplexity) != "user" {
		t.Errorf("Expected category for ErrCodeQueryComplexity to be 'user'")
	}

	// 测试外部错误
	if getCategoryForCode(ErrCodeServiceCall) != "external" {
		t.Errorf("Expected category for ErrCodeServiceCall to be 'external'")
	}

	if getCategoryForCode(ErrCodeTimeout) != "external" {
		t.Errorf("Expected category for ErrCodeTimeout to be 'external'")
	}

	if getCategoryForCode(ErrCodeUnavailable) != "external" {
		t.Errorf("Expected category for ErrCodeUnavailable to be 'external'")
	}

	if getCategoryForCode(ErrCodeServiceNotFound) != "external" {
		t.Errorf("Expected category for ErrCodeServiceNotFound to be 'external'")
	}

	// 测试系统错误
	if getCategoryForCode(ErrCodeConfigInvalid) != "system" {
		t.Errorf("Expected category for ErrCodeConfigInvalid to be 'system'")
	}

	if getCategoryForCode(ErrCodeSchemaInvalid) != "system" {
		t.Errorf("Expected category for ErrCodeSchemaInvalid to be 'system'")
	}

	if getCategoryForCode(ErrCodeInternal) != "system" {
		t.Errorf("Expected category for ErrCodeInternal to be 'system'")
	}

	// 测试未知错误
	if getCategoryForCode("UNKNOWN_ERROR") != "unknown" {
		t.Errorf("Expected category for unknown error to be 'unknown'")
	}
}

func TestIsRetryableCode(t *testing.T) {
	// 测试可重试错误
	if !isRetryableCode(ErrCodeTimeout) {
		t.Errorf("Expected ErrCodeTimeout to be retryable")
	}

	if !isRetryableCode(ErrCodeUnavailable) {
		t.Errorf("Expected ErrCodeUnavailable to be retryable")
	}

	if !isRetryableCode(ErrCodeServiceCall) {
		t.Errorf("Expected ErrCodeServiceCall to be retryable")
	}

	if !isRetryableCode(ErrCodeRateLimit) {
		t.Errorf("Expected ErrCodeRateLimit to be retryable")
	}

	// 测试不可重试错误
	if isRetryableCode(ErrCodeQueryParsing) {
		t.Errorf("Expected ErrCodeQueryParsing to not be retryable")
	}

	if isRetryableCode(ErrCodeQueryValidation) {
		t.Errorf("Expected ErrCodeQueryValidation to not be retryable")
	}

	if isRetryableCode(ErrCodeConfigInvalid) {
		t.Errorf("Expected ErrCodeConfigInvalid to not be retryable")
	}

	if isRetryableCode(ErrCodeSchemaInvalid) {
		t.Errorf("Expected ErrCodeSchemaInvalid to not be retryable")
	}

	if isRetryableCode(ErrCodeInternal) {
		t.Errorf("Expected ErrCodeInternal to not be retryable")
	}
}
