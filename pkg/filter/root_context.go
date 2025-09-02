package filter

import (
	"envoy-wasm-graphql-federation/pkg/jsonutil"
	"fmt"
	"time"

	"github.com/tetratelabs/proxy-wasm-go-sdk/proxywasm"
	"github.com/tetratelabs/proxy-wasm-go-sdk/proxywasm/types"

	"envoy-wasm-graphql-federation/pkg/config"
	"envoy-wasm-graphql-federation/pkg/federation"
	federationtypes "envoy-wasm-graphql-federation/pkg/types"
	"envoy-wasm-graphql-federation/pkg/utils"
)

// RootContext 表示 WASM 扩展的根上下文
type RootContext struct {
	// 核心组件
	federation *federation.Engine
	config     *federationtypes.FederationConfig
	logger     federationtypes.Logger

	// 状态
	initialized bool
}

// NewRootContext 创建新的根上下文
func NewRootContext(vmConfigurationSize int) *RootContext {
	return &RootContext{
		logger: utils.NewLogger("graphql-federation"),
	}
}

// OnVMStart VM 启动时调用
func (ctx *RootContext) OnVMStart(vmConfigurationSize int) types.OnVMStartStatus {
	ctx.logger.Info("GraphQL Federation WASM extension starting...")

	// 读取 VM 配置
	vmConfig, err := proxywasm.GetVMConfiguration()
	if err != nil {
		ctx.logger.Error("Failed to get VM configuration", "error", err)
		return types.OnVMStartStatusFailed
	}

	ctx.logger.Debug("VM configuration received", "size", len(vmConfig))

	return types.OnVMStartStatusOK
}

// OnPluginStart 插件启动时调用
func (ctx *RootContext) OnPluginStart(pluginConfigurationSize int) types.OnPluginStartStatus {
	ctx.logger.Info("GraphQL Federation plugin starting...")

	// 读取插件配置
	pluginConfig, err := proxywasm.GetPluginConfiguration()
	if err != nil {
		ctx.logger.Error("Failed to get plugin configuration", "error", err)
		return types.OnPluginStartStatusFailed
	}

	// 解析配置
	if err := ctx.loadConfiguration(pluginConfig); err != nil {
		ctx.logger.Error("Failed to load configuration", "error", err)
		return types.OnPluginStartStatusFailed
	}

	// 初始化联邦引擎
	if err := ctx.initializeFederation(); err != nil {
		ctx.logger.Error("Failed to initialize federation engine", "error", err)
		return types.OnPluginStartStatusFailed
	}

	ctx.initialized = true
	ctx.logger.Info("GraphQL Federation plugin started successfully",
		"services", len(ctx.config.Services),
		"queryPlanEnabled", ctx.config.EnableQueryPlan,
		"cachingEnabled", ctx.config.EnableCaching,
	)

	return types.OnPluginStartStatusOK
}

// NewHttpContext 创建 HTTP 上下文
func (ctx *RootContext) NewHttpContext(contextID uint32) types.HttpContext {
	if !ctx.initialized {
		ctx.logger.Error("Plugin not initialized, cannot create HTTP context")
		return nil
	}

	return NewHTTPFilterContext(ctx)
}

// NewTcpContext 创建 TCP 上下文（暂不支持）
func (ctx *RootContext) NewTcpContext(contextID uint32) types.TcpContext {
	// 当前不支持 TCP 过滤
	return nil
}

// OnTick 定时器回调
func (ctx *RootContext) OnTick() {
	if !ctx.initialized {
		return
	}

	// 执行定期任务
	ctx.performHealthChecks()
	ctx.collectMetrics()
	ctx.refreshSchemas()
}

// OnPluginDone 插件结束时调用
func (ctx *RootContext) OnPluginDone() bool {
	ctx.logger.Info("GraphQL Federation plugin shutting down...")

	// 清理资源
	if ctx.federation != nil {
		if err := ctx.federation.Shutdown(); err != nil {
			ctx.logger.Error("Failed to shutdown federation engine", "error", err)
		}
		ctx.federation = nil
	}

	ctx.initialized = false
	ctx.logger.Info("GraphQL Federation plugin shutdown completed")

	return true
}

// OnQueueReady 队列就绪回调
func (ctx *RootContext) OnQueueReady(queueID uint32) {
	ctx.logger.Debug("Queue ready", "queueID", queueID)
}

// loadConfiguration 加载配置
func (ctx *RootContext) loadConfiguration(configData []byte) error {
	if len(configData) == 0 {
		return fmt.Errorf("empty configuration")
	}

	ctx.logger.Debug("Loading configuration", "size", len(configData))

	// 解析 JSON 配置
	federationConfig := &federationtypes.FederationConfig{}
	if err := jsonutil.Unmarshal(configData, federationConfig); err != nil {
		return fmt.Errorf("failed to parse configuration JSON: %w", err)
	}

	// 验证配置
	configManager := config.NewManager(ctx.logger)
	if err := configManager.ValidateConfig(federationConfig); err != nil {
		return fmt.Errorf("invalid configuration: %w", err)
	}

	// 设置默认值
	ctx.setConfigDefaults(federationConfig)

	ctx.config = federationConfig
	ctx.logger.Info("Configuration loaded successfully",
		"services", len(federationConfig.Services),
		"maxQueryDepth", federationConfig.MaxQueryDepth,
		"queryTimeout", federationConfig.QueryTimeout,
	)

	return nil
}

// setConfigDefaults 设置配置默认值
func (ctx *RootContext) setConfigDefaults(config *federationtypes.FederationConfig) {
	if config.MaxQueryDepth == 0 {
		config.MaxQueryDepth = 10
	}

	if config.QueryTimeout == 0 {
		config.QueryTimeout = 30 * time.Second
	}

	// 设置服务默认值
	for i := range config.Services {
		service := &config.Services[i]
		if service.Timeout == 0 {
			service.Timeout = 5 * time.Second
		}
		if service.Weight == 0 {
			service.Weight = 1
		}
	}
}

// initializeFederation 初始化联邦引擎
func (ctx *RootContext) initializeFederation() error {
	if ctx.federation != nil {
		// 关闭现有引擎
		_ = ctx.federation.Shutdown()
	}

	// 创建新的联邦引擎
	engine, err := federation.NewEngine(ctx.config, ctx.logger)
	if err != nil {
		return fmt.Errorf("failed to create federation engine: %w", err)
	}

	// 初始化引擎
	if err := engine.Initialize(ctx.config); err != nil {
		return fmt.Errorf("failed to initialize federation engine: %w", err)
	}

	ctx.federation = engine
	return nil
}

// performHealthChecks 执行健康检查
func (ctx *RootContext) performHealthChecks() {
	if ctx.federation == nil {
		return
	}

	// 获取引擎状态
	status := ctx.federation.GetStatus()

	// 记录服务状态
	for serviceName, serviceStatus := range status.Services {
		if !serviceStatus.Healthy {
			ctx.logger.Warn("Service unhealthy",
				"service", serviceName,
				"lastCheck", serviceStatus.LastCheck,
				"errorRate", serviceStatus.ErrorRate,
			)
		}
	}
}

// collectMetrics 收集指标
func (ctx *RootContext) collectMetrics() {
	if ctx.federation == nil {
		return
	}

	status := ctx.federation.GetStatus()

	// 记录关键指标
	ctx.logger.Debug("Engine metrics",
		"uptime", status.Uptime,
		"queryCount", status.QueryCount,
		"errorCount", status.ErrorCount,
	)
}

// refreshSchemas 刷新模式
func (ctx *RootContext) refreshSchemas() {
	if ctx.federation == nil {
		return
	}

	// 定期刷新模式（具体实现依赖于 federation 引擎）
	ctx.logger.Debug("Refreshing schemas")
}

// GetConfig 获取配置
func (ctx *RootContext) GetConfig() *federationtypes.FederationConfig {
	return ctx.config
}

// GetFederation 获取联邦引擎
func (ctx *RootContext) GetFederation() *federation.Engine {
	return ctx.federation
}

// IsInitialized 检查是否已初始化
func (ctx *RootContext) IsInitialized() bool {
	return ctx.initialized
}
