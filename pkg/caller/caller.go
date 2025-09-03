package caller

import (
	"context"
	"envoy-wasm-graphql-federation/pkg/jsonutil"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/proxy-wasm/proxy-wasm-go-sdk/proxywasm"

	"envoy-wasm-graphql-federation/pkg/errors"
	federationtypes "envoy-wasm-graphql-federation/pkg/types"
)

// WASMCaller 实现基于WASM代理的服务调用器
type WASMCaller struct {
	logger      federationtypes.Logger
	healthCache sync.Map // 健康状态缓存
	metrics     *CallerMetrics
	config      *CallerConfig
}

// CallerConfig 调用器配置
type CallerConfig struct {
	DefaultTimeout   time.Duration
	MaxRetries       int
	HealthCheckCache time.Duration
	ConnectTimeout   time.Duration
	ReadTimeout      time.Duration
	WriteTimeout     time.Duration
	MaxIdleConns     int
	MaxConnsPerHost  int
	IdleConnTimeout  time.Duration
}

// CallerMetrics 调用器指标
type CallerMetrics struct {
	TotalCalls      int64
	SuccessfulCalls int64
	FailedCalls     int64
	AvgLatency      int64 // 纳秒
	TimeoutCount    int64
	RetryCount      int64
}

// HealthStatus 健康状态
type HealthStatus struct {
	Healthy    bool
	LastCheck  time.Time
	Latency    time.Duration
	Error      error
	CheckCount int64
	FailCount  int64
}

// NewHTTPCaller 创建新的WASM调用器
func NewHTTPCaller(config *CallerConfig, logger federationtypes.Logger) federationtypes.ServiceCaller {
	if config == nil {
		config = DefaultCallerConfig()
	}

	return &WASMCaller{
		logger:  logger,
		metrics: &CallerMetrics{},
		config:  config,
	}
}

// DefaultCallerConfig 返回默认配置
func DefaultCallerConfig() *CallerConfig {
	return &CallerConfig{
		DefaultTimeout:   10 * time.Second,
		MaxRetries:       3,
		HealthCheckCache: 30 * time.Second,
		ConnectTimeout:   5 * time.Second,
		ReadTimeout:      10 * time.Second,
		WriteTimeout:     10 * time.Second,
		MaxIdleConns:     100,
		MaxConnsPerHost:  10,
		IdleConnTimeout:  90 * time.Second,
	}
}

// Call 调用单个服务（WASM版本）
func (c *WASMCaller) Call(ctx context.Context, call *federationtypes.ServiceCall) (*federationtypes.ServiceResponse, error) {
	if call == nil {
		return nil, errors.NewServiceError("call is nil")
	}

	if call.Service == nil {
		return nil, errors.NewServiceError("service config is nil")
	}

	atomic.AddInt64(&c.metrics.TotalCalls, 1)
	startTime := time.Now()

	c.logger.Debug("Calling service",
		"service", call.Service.Name,
		"endpoint", call.Service.Endpoint,
	)

	// 构建GraphQL请求体
	request := &federationtypes.GraphQLRequest{
		Query:         call.SubQuery.Query,
		Variables:     call.SubQuery.Variables,
		OperationName: call.SubQuery.OperationName,
	}

	// 序列化请求体
	requestBody, err := jsonutil.Marshal(request)
	if err != nil {
		c.recordFailure()
		return nil, errors.NewServiceError("failed to marshal request: " + err.Error())
	}

	// 构建HTTP头
	headers := [][2]string{
		{"content-type", "application/json"},
		{"user-agent", "envoy-wasm-graphql-federation"},
	}

	// 添加服务特定的头部
	if call.Service.Headers != nil {
		for key, value := range call.Service.Headers {
			headers = append(headers, [2]string{key, value})
		}
	}

	// 使用WASM HTTP调用
	// 注意：在实际的WASM环境中，我们需要使用适当的cluster名称
	// 这里我们简化处理，假设endpoint就是cluster名称
	clusterName := c.extractClusterName(call.Service.Endpoint)

	// 发起HTTP调用（这是一个简化版本，实际中需要更复杂的实现）
	// 在WASM环境中，我们通常通过配置的upstream cluster来调用
	return c.makeWASMHTTPCall(clusterName, requestBody, headers, call, startTime)
}

// CallBatch 批量调用服务（使用channel实现并发控制）
func (c *WASMCaller) CallBatch(ctx context.Context, calls []*federationtypes.ServiceCall) ([]*federationtypes.ServiceResponse, error) {
	if len(calls) == 0 {
		return nil, nil
	}

	c.logger.Debug("Executing batch calls with channel-based concurrency", "count", len(calls))

	// 使用channel收集结果
	type callResult struct {
		index    int
		response *federationtypes.ServiceResponse
		err      error
	}

	resultChan := make(chan callResult, len(calls))
	responses := make([]*federationtypes.ServiceResponse, len(calls))

	// 使用goroutine并发执行调用
	var wg sync.WaitGroup
	for i, call := range calls {
		wg.Add(1)
		go func(idx int, serviceCall *federationtypes.ServiceCall) {
			defer wg.Done()

			resp, err := c.Call(ctx, serviceCall)

			// 通过channel发送结果
			select {
			case resultChan <- callResult{index: idx, response: resp, err: err}:
			case <-ctx.Done():
				// 上下文取消，直接返回
				return
			}
		}(i, call)
	}

	// 等待所有goroutine完成
	go func() {
		wg.Wait()
		close(resultChan)
	}()

	// 收集结果
	var callErrors []error
	for result := range resultChan {
		if result.err != nil {
			callErrors = append(callErrors, fmt.Errorf("call %d failed: %v", result.index, result.err))
			c.logger.Error("Batch call failed",
				"index", result.index,
				"error", result.err)
		} else {
			responses[result.index] = result.response
		}
	}

	// 检查是否有错误
	if len(callErrors) > 0 {
		// 构建错误消息
		errorMsg := fmt.Sprintf("batch call completed with %d errors out of %d calls", len(callErrors), len(calls))
		for i, err := range callErrors {
			errorMsg += fmt.Sprintf("; error %d: %v", i+1, err)
		}
		// 使用errors包的NewBatchError方法
		return responses, errors.NewBatchError(errorMsg)
	}

	c.logger.Debug("Batch calls completed successfully", "count", len(calls))
	return responses, nil
}

// IsHealthy 检查服务健康状态（简化WASM版本）
func (c *WASMCaller) IsHealthy(ctx context.Context, service *federationtypes.ServiceConfig) bool {
	if service == nil {
		return false
	}

	// 在WASM环境中，我们简化健康检查逻辑
	// 检查缓存
	if cached, ok := c.healthCache.Load(service.Name); ok {
		status := cached.(*HealthStatus)
		if time.Since(status.LastCheck) < c.config.HealthCheckCache {
			return status.Healthy
		}
	}

	// 在WASM环境中，我们假设服务健康（实际中应该通过配置或其他机制来检查）
	healthy := true

	// 更新缓存
	status := &HealthStatus{
		Healthy:   healthy,
		LastCheck: time.Now(),
	}
	c.healthCache.Store(service.Name, status)

	return healthy
}

// extractClusterName 从Domain或URL中提取cluster名称
func (c *WASMCaller) extractClusterName(endpoint string) string {
	// 简化处理：移除http://或https://前缀
	if strings.HasPrefix(endpoint, "http://") {
		endpoint = endpoint[7:]
	} else if strings.HasPrefix(endpoint, "https://") {
		endpoint = endpoint[8:]
	}

	// 移除路径部分
	if idx := strings.Index(endpoint, "/"); idx > 0 {
		endpoint = endpoint[:idx]
	}

	// 移除端口号（如果有）
	if idx := strings.Index(endpoint, ":"); idx > 0 {
		endpoint = endpoint[:idx]
	}

	return endpoint
}

// makeWASMHTTPCall 使用WASM进行HTTP调用
func (c *WASMCaller) makeWASMHTTPCall(clusterName string, requestBody []byte, headers [][2]string, call *federationtypes.ServiceCall, startTime time.Time) (*federationtypes.ServiceResponse, error) {
	c.logger.Debug("Making WASM HTTP call",
		"cluster", clusterName,
		"service", call.Service.Name,
		"bodySize", len(requestBody),
	)

	// 构建HTTP调用的路径（通常是GraphQL端点）
	path := "/graphql"
	if call.Service.Path != "" {
		path = call.Service.Path
	}

	// 添加必要的HTTP方法头
	methodHeaders := [][2]string{
		{":method", "POST"},
		{":path", path},
		{":authority", clusterName},
	}
	// 合并头部
	allHeaders := append(methodHeaders, headers...)

	// 记录调用开始
	atomic.AddInt64(&c.metrics.TotalCalls, 1)

	proxywasm.LogDebugf("Dispatching HTTP call to cluster: %s, path: %s", clusterName, path)

	// 使用proxywasm.DispatchHttpCall进行实际的HTTP调用
	// 创建处理器
	var handler *WASMHTTPCallHandler
	calloutID, err := proxywasm.DispatchHttpCall(
		clusterName,   // 上游集群名称
		allHeaders,    // HTTP头部（包括方法和路径）
		requestBody,   // 请求体
		[][2]string{}, // 跟踪头（通常为空）
		uint32(call.Service.Timeout.Milliseconds()), // 超时时间（毫秒）
		func(numHeaders, bodySize, numTrailers int) {
			// HTTP调用响应回调
			if handler != nil {
				handler.OnHttpCallResponse(numHeaders, bodySize, numTrailers)
			}
		},
	)

	// 初始化处理器
	handler = NewWASMHTTPCallHandler(calloutID)

	if err != nil {
		c.recordFailure()
		return nil, errors.NewServiceError(fmt.Sprintf("failed to dispatch HTTP call: %v", err))
	}

	c.logger.Debug("HTTP call dispatched", "calloutID", calloutID)

	// 由于proxy-wasm的HTTP调用是异步的，我们使用channel进行同步等待
	// 通过handler的Wait方法等待响应，该方法使用channel实现异步通信
	response, err := handler.Wait(call.Service.Timeout)

	// 清理资源
	defer handler.Close()

	if err != nil {
		c.recordFailure()
		proxywasm.LogErrorf("HTTP call failed, calloutID=%d, error=%v", calloutID, err)
		return nil, fmt.Errorf("HTTP call failed: %v", err)
	}

	// 更新指标
	latency := time.Since(startTime)
	c.updateLatency(latency)
	atomic.AddInt64(&c.metrics.SuccessfulCalls, 1)

	// 返回响应
	response.Service = call.Service.Name
	response.Latency = latency
	return response, nil
}

// WASMHTTPCallHandler 处理WASM HTTP调用的回调
type WASMHTTPCallHandler struct {
	calloutID    uint32
	responseChan chan *federationtypes.ServiceResponse
	errorChan    chan error
	processed    bool
	mutex        sync.Mutex
}

// NewWASMHTTPCallHandler 创建新的HTTP调用处理器
func NewWASMHTTPCallHandler(calloutID uint32) *WASMHTTPCallHandler {
	return &WASMHTTPCallHandler{
		calloutID:    calloutID,
		responseChan: make(chan *federationtypes.ServiceResponse, 1),
		errorChan:    make(chan error, 1),
		processed:    false,
	}
}

// OnHttpCallResponse 处理HTTP调用响应（使用channel实现异步通信）
func (h *WASMHTTPCallHandler) OnHttpCallResponse(numHeaders, bodySize, numTrailers int) {
	h.mutex.Lock()
	defer h.mutex.Unlock()

	// 防止重复处理
	if h.processed {
		proxywasm.LogWarnf("HTTP response already processed for calloutID: %d", h.calloutID)
		return
	}
	h.processed = true

	proxywasm.LogDebugf("Received HTTP response: headers=%d, bodySize=%d, trailers=%d, calloutID=%d",
		numHeaders, bodySize, numTrailers, h.calloutID)

	// 获取响应头
	responseHeaders, err := proxywasm.GetHttpCallResponseHeaders()
	if err != nil {
		proxywasm.LogErrorf("Failed to get response headers: %v", err)
		h.sendError(fmt.Errorf("failed to get response headers: %v", err))
		return
	}

	// 获取响应体
	responseBody, err := proxywasm.GetHttpCallResponseBody(0, bodySize)
	if err != nil {
		proxywasm.LogErrorf("Failed to get response body: %v", err)
		h.sendError(fmt.Errorf("failed to get response body: %v", err))
		return
	}

	// 解析响应状态码和头部
	status := "200" // 默认值
	headerMap := make(map[string]string)
	for _, header := range responseHeaders {
		if header[0] == ":status" {
			status = header[1]
		} else {
			headerMap[header[0]] = header[1]
		}
	}

	proxywasm.LogInfof("HTTP call response: status=%s, bodySize=%d, calloutID=%d", status, bodySize, h.calloutID)

	// 创建响应对象
	response := &federationtypes.ServiceResponse{
		Headers: headerMap,
		Metadata: map[string]interface{}{
			"status_code":    status,
			"callout_id":     h.calloutID,
			"body_size":      bodySize,
			"headers_count":  numHeaders,
			"trailers_count": numTrailers,
		},
	}

	// 解析GraphQL响应体
	if bodySize > 0 && len(responseBody) > 0 {
		var graphqlResponse federationtypes.GraphQLResponse
		if err := jsonutil.Unmarshal(responseBody, &graphqlResponse); err != nil {
			proxywasm.LogErrorf("Failed to parse GraphQL response: %v", err)
			// 即使解析失败，也要返回原始响应数据
			response.Metadata["raw_body"] = string(responseBody)
			response.Metadata["parse_error"] = err.Error()
		} else {
			proxywasm.LogDebugf("GraphQL response parsed successfully, calloutID=%d", h.calloutID)
			response.Data = graphqlResponse.Data
			response.Errors = graphqlResponse.Errors
			// 合并extensions到metadata
			if graphqlResponse.Extensions != nil {
				for k, v := range graphqlResponse.Extensions {
					response.Metadata[k] = v
				}
			}
		}
	} else {
		proxywasm.LogDebugf("Empty response body, calloutID=%d", h.calloutID)
	}

	// 通过channel发送响应
	h.sendResponse(response)
}

// sendResponse 通过channel发送响应
func (h *WASMHTTPCallHandler) sendResponse(response *federationtypes.ServiceResponse) {
	select {
	case h.responseChan <- response:
		proxywasm.LogDebugf("Response sent successfully via channel, calloutID=%d", h.calloutID)
	default:
		// channel已满或已关闭，记录警告
		proxywasm.LogWarnf("Response channel is full or closed, calloutID=%d", h.calloutID)
	}
}

// sendError 通过channel发送错误
func (h *WASMHTTPCallHandler) sendError(err error) {
	select {
	case h.errorChan <- err:
		proxywasm.LogDebugf("Error sent successfully via channel, calloutID=%d, error=%v", h.calloutID, err)
	default:
		// channel已满或已关闭，记录警告
		proxywasm.LogWarnf("Error channel is full or closed, calloutID=%d, error=%v", h.calloutID, err)
	}
}

// Wait 通过channel等待响应完成
func (h *WASMHTTPCallHandler) Wait(timeout time.Duration) (*federationtypes.ServiceResponse, error) {
	proxywasm.LogDebugf("Waiting for HTTP response via channel, calloutID=%d, timeout=%v", h.calloutID, timeout)

	// 使用select语句同时等待响应、错误和超时
	select {
	case response := <-h.responseChan:
		proxywasm.LogDebugf("Received response via channel, calloutID=%d", h.calloutID)
		return response, nil

	case err := <-h.errorChan:
		proxywasm.LogErrorf("Received error via channel, calloutID=%d, error=%v", h.calloutID, err)
		return nil, err

	case <-time.After(timeout):
		proxywasm.LogErrorf("HTTP call timeout after %v, calloutID=%d", timeout, h.calloutID)
		return nil, fmt.Errorf("HTTP call timeout after %v for calloutID %d", timeout, h.calloutID)
	}
}

// Close 关闭channel资源
func (h *WASMHTTPCallHandler) Close() {
	h.mutex.Lock()
	defer h.mutex.Unlock()

	// 关闭channel防止内存泄漏
	select {
	case <-h.responseChan:
	default:
		close(h.responseChan)
	}

	select {
	case <-h.errorChan:
	default:
		close(h.errorChan)
	}

	proxywasm.LogDebugf("HTTP call handler closed, calloutID=%d", h.calloutID)
}

// recordFailure 记录失败
func (c *WASMCaller) recordFailure() {
	atomic.AddInt64(&c.metrics.FailedCalls, 1)
}

// updateLatency 更新平均延迟
func (c *WASMCaller) updateLatency(latency time.Duration) {
	// 简单的移动平均
	currentAvg := atomic.LoadInt64(&c.metrics.AvgLatency)
	newAvg := (currentAvg + latency.Nanoseconds()) / 2
	atomic.StoreInt64(&c.metrics.AvgLatency, newAvg)
}

// GetMetrics 获取调用器指标
func (c *WASMCaller) GetMetrics() *CallerMetrics {
	return &CallerMetrics{
		TotalCalls:      atomic.LoadInt64(&c.metrics.TotalCalls),
		SuccessfulCalls: atomic.LoadInt64(&c.metrics.SuccessfulCalls),
		FailedCalls:     atomic.LoadInt64(&c.metrics.FailedCalls),
		AvgLatency:      atomic.LoadInt64(&c.metrics.AvgLatency),
		TimeoutCount:    atomic.LoadInt64(&c.metrics.TimeoutCount),
		RetryCount:      atomic.LoadInt64(&c.metrics.RetryCount),
	}
}

// GetHealthStatus 获取服务健康状态
func (c *WASMCaller) GetHealthStatus(serviceName string) *HealthStatus {
	if cached, ok := c.healthCache.Load(serviceName); ok {
		status := cached.(*HealthStatus)
		// 返回副本避免并发问题
		return &HealthStatus{
			Healthy:    status.Healthy,
			LastCheck:  status.LastCheck,
			Latency:    status.Latency,
			Error:      status.Error,
			CheckCount: status.CheckCount,
			FailCount:  status.FailCount,
		}
	}
	return nil
}

// ClearHealthCache 清理健康检查缓存
func (c *WASMCaller) ClearHealthCache() {
	c.healthCache.Range(func(key, value interface{}) bool {
		c.healthCache.Delete(key)
		return true
	})
}
