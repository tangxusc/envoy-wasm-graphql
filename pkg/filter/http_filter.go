package filter

import (
	"envoy-wasm-graphql-federation/pkg/jsonutil"
	"fmt"
	"strings"
	"time"

	"github.com/proxy-wasm/proxy-wasm-go-sdk/proxywasm"
	"github.com/proxy-wasm/proxy-wasm-go-sdk/proxywasm/types"

	"envoy-wasm-graphql-federation/pkg/errors"
	"envoy-wasm-graphql-federation/pkg/federation"
	federationtypes "envoy-wasm-graphql-federation/pkg/types"
	"envoy-wasm-graphql-federation/pkg/utils"
)

// HTTPFilterContext 表示 HTTP 过滤器上下文
type HTTPFilterContext struct {
	types.DefaultHttpContext

	// 核心组件
	federation *federation.Engine
	config     *federationtypes.FederationConfig
	logger     federationtypes.Logger

	// 请求状态
	requestBody  []byte
	responseBody []byte
	requestID    string
	startTime    time.Time

	// GraphQL 相关
	graphqlRequest  *federationtypes.GraphQLRequest
	graphqlResponse *federationtypes.GraphQLResponse

	// 错误状态
	lastError error
}

// NewHTTPFilterContext 创建新的 HTTP 过滤器上下文
func NewHTTPFilterContext(rootContext *RootContext) *HTTPFilterContext {
	return &HTTPFilterContext{
		federation: rootContext.federation,
		config:     rootContext.config,
		logger:     rootContext.logger,
		requestID:  utils.GenerateRequestID(),
		startTime:  time.Now(),
	}
}

// OnHttpRequestHeaders 处理 HTTP 请求头
func (ctx *HTTPFilterContext) OnHttpRequestHeaders(numHeaders int, endOfStream bool) types.Action {
	// 记录请求开始
	ctx.logger.Info("Processing GraphQL request",
		"requestId", ctx.requestID,
		"path", ctx.getRequestPath(),
		"method", ctx.getRequestMethod(),
	)

	// 验证请求方法
	method := ctx.getRequestMethod()
	if method != "POST" && method != "GET" {
		ctx.logger.Warn("Unsupported HTTP method", "method", method)
		return ctx.sendErrorResponse(400, "Only POST and GET methods are supported")
	}

	// 验证 Content-Type (仅对 POST 请求)
	if method == "POST" {
		contentType := ctx.getRequestHeader("content-type")
		if !ctx.isValidContentType(contentType) {
			ctx.logger.Warn("Invalid content type", "contentType", contentType)
			return ctx.sendErrorResponse(400, "Invalid content type")
		}
	}

	// 检查请求路径是否为 GraphQL 端点
	path := ctx.getRequestPath()
	if !ctx.isGraphQLEndpoint(path) {
		// 不是 GraphQL 请求，继续传递
		return types.ActionContinue
	}

	// 如果是 GET 请求，尝试从查询参数获取 GraphQL 查询
	if method == "GET" {
		if err := ctx.handleGetRequest(); err != nil {
			ctx.logger.Error("Failed to handle GET request", "error", err)
			return ctx.sendErrorResponse(400, "Invalid GraphQL query parameters")
		}

		// GET 请求没有请求体，直接处理
		return ctx.processGraphQLRequest()
	}

	// POST 请求继续读取请求体
	return types.ActionContinue
}

// OnHttpRequestBody 处理 HTTP 请求体
func (ctx *HTTPFilterContext) OnHttpRequestBody(bodySize int, endOfStream bool) types.Action {
	if !endOfStream {
		// 还有更多数据，继续等待
		return types.ActionPause
	}

	// 读取完整请求体
	body, err := proxywasm.GetHttpRequestBody(0, bodySize)
	if err != nil {
		ctx.logger.Error("Failed to get request body", "error", err)
		return ctx.sendErrorResponse(500, "Failed to read request body")
	}

	ctx.requestBody = body

	// 解析 GraphQL 请求
	if err := ctx.parseGraphQLRequest(); err != nil {
		ctx.logger.Error("Failed to parse GraphQL request", "error", err)
		return ctx.sendErrorResponse(400, "Invalid GraphQL request")
	}

	// 处理 GraphQL 请求
	return ctx.processGraphQLRequest()
}

// OnHttpResponseHeaders 处理 HTTP 响应头
func (ctx *HTTPFilterContext) OnHttpResponseHeaders(numHeaders int, endOfStream bool) types.Action {
	// 如果没有处理 GraphQL 请求，直接继续
	if ctx.graphqlResponse == nil {
		return types.ActionContinue
	}

	// 设置响应头
	_ = proxywasm.ReplaceHttpResponseHeader("content-type", "application/json")
	_ = proxywasm.AddHttpResponseHeader("x-graphql-federation", "true")
	_ = proxywasm.AddHttpResponseHeader("x-request-id", ctx.requestID)

	return types.ActionContinue
}

// OnHttpResponseBody 处理 HTTP 响应体
func (ctx *HTTPFilterContext) OnHttpResponseBody(bodySize int, endOfStream bool) types.Action {
	// 如果没有处理 GraphQL 请求，直接继续
	if ctx.graphqlResponse == nil {
		return types.ActionContinue
	}

	if !endOfStream {
		return types.ActionPause
	}

	// 替换响应体为 GraphQL 联邦响应
	responseBody, err := jsonutil.Marshal(ctx.graphqlResponse)
	if err != nil {
		ctx.logger.Error("Failed to marshal GraphQL response", "error", err)
		return ctx.sendErrorResponse(500, "Failed to generate response")
	}

	if err := proxywasm.ReplaceHttpResponseBody(responseBody); err != nil {
		ctx.logger.Error("Failed to replace response body", "error", err)
		return types.ActionContinue
	}

	return types.ActionContinue
}

// OnHttpStreamDone 请求处理完成
func (ctx *HTTPFilterContext) OnHttpStreamDone() {
	duration := time.Since(ctx.startTime)

	if ctx.graphqlResponse != nil {
		ctx.logger.Info("GraphQL request completed",
			"requestId", ctx.requestID,
			"duration", duration,
			"hasErrors", len(ctx.graphqlResponse.Errors) > 0,
		)
	}
}

// parseGraphQLRequest 解析 GraphQL 请求
func (ctx *HTTPFilterContext) parseGraphQLRequest() error {
	if len(ctx.requestBody) == 0 {
		return fmt.Errorf("empty request body")
	}

	var request federationtypes.GraphQLRequest
	if err := jsonutil.Unmarshal(ctx.requestBody, &request); err != nil {
		return fmt.Errorf("failed to parse JSON: %w", err)
	}

	// 验证请求
	if strings.TrimSpace(request.Query) == "" {
		return fmt.Errorf("query is required")
	}

	ctx.graphqlRequest = &request
	return nil
}

// handleGetRequest 处理 GET 请求
func (ctx *HTTPFilterContext) handleGetRequest() error {
	// 从查询参数获取 GraphQL 查询
	queryParam := ctx.getQueryParam("query")
	if queryParam == "" {
		return fmt.Errorf("query parameter is required")
	}

	request := &federationtypes.GraphQLRequest{
		Query: queryParam,
	}

	// 获取变量参数
	if variablesParam := ctx.getQueryParam("variables"); variablesParam != "" {
		var variables map[string]interface{}
		if err := jsonutil.Unmarshal([]byte(variablesParam), &variables); err != nil {
			return fmt.Errorf("invalid variables parameter: %w", err)
		}
		request.Variables = variables
	}

	// 获取操作名称
	if operationName := ctx.getQueryParam("operationName"); operationName != "" {
		request.OperationName = operationName
	}

	ctx.graphqlRequest = request
	return nil
}

// processGraphQLRequest 处理 GraphQL 请求
func (ctx *HTTPFilterContext) processGraphQLRequest() types.Action {
	if ctx.graphqlRequest == nil {
		return ctx.sendErrorResponse(400, "No GraphQL request to process")
	}

	// 创建执行上下文
	execCtx := &federationtypes.ExecutionContext{
		RequestID: ctx.requestID,
		QueryContext: &federationtypes.QueryContext{
			Query:     ctx.graphqlRequest.Query,
			Variables: ctx.graphqlRequest.Variables,
			Operation: ctx.graphqlRequest.OperationName,
			RequestID: ctx.requestID,
			Headers:   ctx.getRequestHeaders(),
		},
		StartTime: ctx.startTime,
		Config:    ctx.config,
	}

	// 执行 GraphQL 查询
	response, err := ctx.federation.ExecuteQuery(execCtx, ctx.graphqlRequest)
	if err != nil {
		ctx.logger.Error("Failed to execute GraphQL query", "error", err)

		// 如果是联邦错误，转换为 GraphQL 错误响应
		if fedErr, ok := err.(*errors.FederationError); ok {
			ctx.graphqlResponse = &federationtypes.GraphQLResponse{
				Errors: []federationtypes.GraphQLError{
					{
						Message:    fedErr.Message,
						Extensions: fedErr.ToGraphQLError()["extensions"].(map[string]interface{}),
					},
				},
			}
		} else {
			ctx.graphqlResponse = &federationtypes.GraphQLResponse{
				Errors: []federationtypes.GraphQLError{
					{
						Message: "Internal server error",
						Extensions: map[string]interface{}{
							"code": "INTERNAL_ERROR",
						},
					},
				},
			}
		}
	} else {
		ctx.graphqlResponse = response
	}

	// 阻止请求继续传递到上游服务
	return types.ActionPause
}

// sendErrorResponse 发送错误响应
func (ctx *HTTPFilterContext) sendErrorResponse(statusCode int, message string) types.Action {
	errorResponse := &federationtypes.GraphQLResponse{
		Errors: []federationtypes.GraphQLError{
			{
				Message: message,
				Extensions: map[string]interface{}{
					"code": "REQUEST_ERROR",
				},
			},
		},
	}

	responseBody, _ := jsonutil.Marshal(errorResponse)

	_ = proxywasm.SendHttpResponse(uint32(statusCode), [][2]string{
		{"content-type", "application/json"},
		{"x-request-id", ctx.requestID},
	}, responseBody, -1)

	return types.ActionPause
}

// 辅助方法

func (ctx *HTTPFilterContext) getRequestMethod() string {
	method, _ := proxywasm.GetHttpRequestHeader(":method")
	return method
}

func (ctx *HTTPFilterContext) getRequestPath() string {
	path, _ := proxywasm.GetHttpRequestHeader(":path")
	return path
}

func (ctx *HTTPFilterContext) getRequestHeader(name string) string {
	header, _ := proxywasm.GetHttpRequestHeader(name)
	return header
}

func (ctx *HTTPFilterContext) getRequestHeaders() map[string]string {
	headers := make(map[string]string)

	headerPairs, _ := proxywasm.GetHttpRequestHeaders()
	for _, pair := range headerPairs {
		headers[pair[0]] = pair[1]
	}

	return headers
}

func (ctx *HTTPFilterContext) getQueryParam(name string) string {
	path := ctx.getRequestPath()
	if idx := strings.Index(path, "?"); idx > 0 {
		query := path[idx+1:]
		return utils.GetQueryParam(query, name)
	}
	return ""
}

func (ctx *HTTPFilterContext) isValidContentType(contentType string) bool {
	contentType = strings.ToLower(strings.TrimSpace(contentType))
	return contentType == "application/json" ||
		contentType == "application/graphql" ||
		strings.HasPrefix(contentType, "application/json")
}

func (ctx *HTTPFilterContext) isGraphQLEndpoint(path string) bool {
	// 移除查询参数
	if idx := strings.Index(path, "?"); idx > 0 {
		path = path[:idx]
	}

	// 检查是否为 GraphQL 端点
	return path == "/graphql" ||
		path == "/graphql/" ||
		strings.HasSuffix(path, "/graphql") ||
		strings.HasSuffix(path, "/graphql/")
}
