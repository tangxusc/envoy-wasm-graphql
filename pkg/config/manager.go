package config

import (
	"envoy-wasm-graphql-federation/pkg/jsonutil"
	"fmt"
	"strings"
	"sync"
	"time"

	"envoy-wasm-graphql-federation/pkg/errors"
	federationtypes "envoy-wasm-graphql-federation/pkg/types"
	"envoy-wasm-graphql-federation/pkg/utils"
)

// Manager 实现增强的配置管理器
type Manager struct {
	logger          federationtypes.Logger
	config          *federationtypes.FederationConfig
	mutex           sync.RWMutex
	validationLevel ValidationLevel
	reloadHandlers  []ReloadHandler
	changeDetectors []ChangeDetector
	validators      []ConfigValidator
	lastUpdate      time.Time
	version         string
	metrics         *ConfigMetrics
}

// ValidationLevel 验证级别
type ValidationLevel int

const (
	ValidationLevelBasic ValidationLevel = iota
	ValidationLevelStrict
	ValidationLevelExtended
)

// ReloadHandler 配置重载处理器
type ReloadHandler interface {
	OnConfigReload(oldConfig, newConfig *federationtypes.FederationConfig) error
	GetName() string
}

// ChangeDetector 变更检测器
type ChangeDetector interface {
	DetectChanges(oldConfig, newConfig *federationtypes.FederationConfig) []ConfigChange
	GetName() string
}

// ConfigValidator 配置验证器
type ConfigValidator interface {
	Validate(config *federationtypes.FederationConfig) []ValidationError
	GetName() string
}

// ConfigChange 配置变更
type ConfigChange struct {
	Type        ChangeType             `json:"type"`
	Path        string                 `json:"path"`
	OldValue    interface{}            `json:"oldValue,omitempty"`
	NewValue    interface{}            `json:"newValue,omitempty"`
	Description string                 `json:"description"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// ChangeType 变更类型
type ChangeType string

const (
	ChangeTypeAdded    ChangeType = "added"
	ChangeTypeModified ChangeType = "modified"
	ChangeTypeRemoved  ChangeType = "removed"
)

// ValidationError 验证错误
type ValidationError struct {
	Path       string                 `json:"path"`
	Message    string                 `json:"message"`
	Severity   ErrorSeverity          `json:"severity"`
	Code       string                 `json:"code"`
	Suggestion string                 `json:"suggestion,omitempty"`
	Metadata   map[string]interface{} `json:"metadata,omitempty"`
}

// ErrorSeverity 错误严重程度
type ErrorSeverity string

const (
	SeverityError   ErrorSeverity = "error"
	SeverityWarning ErrorSeverity = "warning"
	SeverityInfo    ErrorSeverity = "info"
)

// ConfigMetrics 配置指标
type ConfigMetrics struct {
	ReloadCount       int64           `json:"reloadCount"`
	ValidationCount   int64           `json:"validationCount"`
	ValidationErrors  int64           `json:"validationErrors"`
	LastReloadTime    time.Time       `json:"lastReloadTime"`
	LastValidation    time.Time       `json:"lastValidation"`
	AverageReloadTime time.Duration   `json:"averageReloadTime"`
	ConfigVersion     string          `json:"configVersion"`
	ServiceCount      int             `json:"serviceCount"`
	ServiceHealth     map[string]bool `json:"serviceHealth"`
}

// NewManager 创建增强的配置管理器
func NewManager(logger federationtypes.Logger) federationtypes.ConfigManager {
	return NewManagerWithOptions(logger, ManagerOptions{
		ValidationLevel: ValidationLevelBasic,
	})
}

// ManagerOptions 管理器选项
type ManagerOptions struct {
	ValidationLevel ValidationLevel
	ReloadHandlers  []ReloadHandler
	ChangeDetectors []ChangeDetector
	Validators      []ConfigValidator
}

// NewManagerWithOptions 使用选项创建管理器
func NewManagerWithOptions(logger federationtypes.Logger, options ManagerOptions) *Manager {
	m := &Manager{
		logger:          logger,
		validationLevel: options.ValidationLevel,
		reloadHandlers:  options.ReloadHandlers,
		changeDetectors: options.ChangeDetectors,
		validators:      options.Validators,
		lastUpdate:      time.Now(),
		version:         "1.0.0",
		metrics:         &ConfigMetrics{},
	}

	// 添加默认验证器
	if len(m.validators) == 0 {
		m.validators = append(m.validators, NewDefaultValidator())
	}

	// 添加默认变更检测器
	if len(m.changeDetectors) == 0 {
		m.changeDetectors = append(m.changeDetectors, NewDefaultChangeDetector())
	}

	return m
}

// LoadConfig 加载配置（增强版）
func (m *Manager) LoadConfig(data []byte) (*federationtypes.FederationConfig, error) {
	if len(data) == 0 {
		return nil, errors.NewConfigError("configuration data is empty")
	}

	m.logger.Debug("Loading configuration", "size", len(data))
	startTime := time.Now()

	// 解析配置
	var newConfig federationtypes.FederationConfig
	if err := jsonutil.Unmarshal(data, &newConfig); err != nil {
		return nil, errors.NewConfigError("failed to parse configuration JSON: " + err.Error())
	}

	// 设置默认值
	m.setDefaults(&newConfig)

	// 验证配置
	if err := m.validateConfigEnhanced(&newConfig); err != nil {
		m.metrics.ValidationErrors++
		return nil, err
	}

	// 检测变更
	m.mutex.Lock()
	oldConfig := m.config
	changes := m.detectChanges(oldConfig, &newConfig)
	m.mutex.Unlock()

	// 处理配置变更
	if len(changes) > 0 {
		m.logger.Info("Configuration changes detected", "changes", len(changes))
		for _, change := range changes {
			m.logger.Debug("Config change", "type", change.Type, "path", change.Path, "description", change.Description)
		}

		// 调用重载处理器
		if err := m.handleConfigReload(oldConfig, &newConfig, changes); err != nil {
			return nil, fmt.Errorf("configuration reload failed: %w", err)
		}
	}

	// 更新配置
	m.mutex.Lock()
	m.config = &newConfig
	m.lastUpdate = time.Now()
	m.version = m.generateConfigVersion(&newConfig)
	m.metrics.ReloadCount++
	m.metrics.LastReloadTime = time.Now()
	m.updateMetrics(&newConfig)
	m.mutex.Unlock()

	reloadDuration := time.Since(startTime)
	m.updateAverageReloadTime(reloadDuration)

	m.logger.Info("Configuration loaded successfully",
		"version", m.version,
		"services", len(newConfig.Services),
		"queryPlanning", newConfig.EnableQueryPlan,
		"caching", newConfig.EnableCaching,
		"changes", len(changes),
		"duration", reloadDuration,
	)

	return &newConfig, nil
}

// ValidateConfig 验证配置
func (m *Manager) ValidateConfig(config *federationtypes.FederationConfig) error {
	return m.validateConfigEnhanced(config)
}

// validateConfigEnhanced 增强的配置验证
func (m *Manager) validateConfigEnhanced(config *federationtypes.FederationConfig) error {
	m.metrics.ValidationCount++
	m.metrics.LastValidation = time.Now()

	var allErrors []ValidationError

	// 调用所有验证器
	for _, validator := range m.validators {
		errors := validator.Validate(config)
		allErrors = append(allErrors, errors...)
	}

	// 检查错误严重程度
	var criticalErrors []ValidationError
	var warnings []ValidationError

	for _, err := range allErrors {
		if err.Severity == SeverityError {
			criticalErrors = append(criticalErrors, err)
		} else if err.Severity == SeverityWarning {
			warnings = append(warnings, err)
		}
	}

	// 记录警告
	for _, warning := range warnings {
		m.logger.Warn("Configuration validation warning",
			"path", warning.Path,
			"message", warning.Message,
			"code", warning.Code,
		)
	}

	// 如果有关键错误，返回失败
	if len(criticalErrors) > 0 {
		mainError := criticalErrors[0]
		errorMsg := fmt.Sprintf("%s (and %d more errors)", mainError.Message, len(criticalErrors)-1)
		if len(criticalErrors) == 1 {
			errorMsg = mainError.Message
		}

		// 创建配置错误
		return fmt.Errorf("configuration validation failed: %s", errorMsg)
	}

	m.logger.Debug("Configuration validation passed",
		"warnings", len(warnings),
		"validationTime", time.Since(m.metrics.LastValidation),
	)

	return nil
}

// detectChanges 检测配置变更
func (m *Manager) detectChanges(oldConfig, newConfig *federationtypes.FederationConfig) []ConfigChange {
	if oldConfig == nil {
		return []ConfigChange{{
			Type:        ChangeTypeAdded,
			Path:        "root",
			NewValue:    "initial configuration",
			Description: "Initial configuration loaded",
		}}
	}

	var allChanges []ConfigChange

	// 调用所有变更检测器
	for _, detector := range m.changeDetectors {
		changes := detector.DetectChanges(oldConfig, newConfig)
		allChanges = append(allChanges, changes...)
	}

	return allChanges
}

// handleConfigReload 处理配置重载
func (m *Manager) handleConfigReload(oldConfig, newConfig *federationtypes.FederationConfig, changes []ConfigChange) error {
	// 调用所有重载处理器
	for _, handler := range m.reloadHandlers {
		if err := handler.OnConfigReload(oldConfig, newConfig); err != nil {
			m.logger.Error("Config reload handler failed",
				"handler", handler.GetName(),
				"error", err,
			)
			return fmt.Errorf("handler %s failed: %w", handler.GetName(), err)
		}
	}

	return nil
}

// generateConfigVersion 生成配置版本
func (m *Manager) generateConfigVersion(config *federationtypes.FederationConfig) string {
	// 使用配置内容的哈希值作为版本
	data, _ := jsonutil.Marshal(config)
	return fmt.Sprintf("v%d-%x", time.Now().Unix(), utils.HashString(string(data)))
}

// updateMetrics 更新指标
func (m *Manager) updateMetrics(config *federationtypes.FederationConfig) {
	m.metrics.ConfigVersion = m.version
	m.metrics.ServiceCount = len(config.Services)

	// 初始化服务健康状态
	if m.metrics.ServiceHealth == nil {
		m.metrics.ServiceHealth = make(map[string]bool)
	}

	for _, service := range config.Services {
		if _, exists := m.metrics.ServiceHealth[service.Name]; !exists {
			m.metrics.ServiceHealth[service.Name] = true // 默认健康
		}
	}
}

// updateAverageReloadTime 更新平均重载时间
func (m *Manager) updateAverageReloadTime(duration time.Duration) {
	if m.metrics.ReloadCount == 1 {
		m.metrics.AverageReloadTime = duration
	} else {
		// 简单的移动平均
		m.metrics.AverageReloadTime = (m.metrics.AverageReloadTime + duration) / 2
	}
}

// AddReloadHandler 添加重载处理器
func (m *Manager) AddReloadHandler(handler ReloadHandler) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.reloadHandlers = append(m.reloadHandlers, handler)
}

// AddChangeDetector 添加变更检测器
func (m *Manager) AddChangeDetector(detector ChangeDetector) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.changeDetectors = append(m.changeDetectors, detector)
}

// AddValidator 添加验证器
func (m *Manager) AddValidator(validator ConfigValidator) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.validators = append(m.validators, validator)
}

// GetVersion 获取配置版本
func (m *Manager) GetVersion() string {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	return m.version
}

// GetLastUpdate 获取最后更新时间
func (m *Manager) GetLastUpdate() time.Time {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	return m.lastUpdate
}

// GetMetrics 获取配置指标
func (m *Manager) GetMetrics() ConfigMetrics {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	// 返回副本
	metrics := *m.metrics
	if metrics.ServiceHealth != nil {
		healthCopy := make(map[string]bool)
		for k, v := range m.metrics.ServiceHealth {
			healthCopy[k] = v
		}
		metrics.ServiceHealth = healthCopy
	}

	return metrics
}

// validateConfig 验证配置
func (m *Manager) validateConfig(config *federationtypes.FederationConfig) error {
	if config == nil {
		return errors.NewConfigError("configuration is nil")
	}

	// 验证服务配置
	if len(config.Services) == 0 {
		return errors.NewConfigError("no services configured")
	}

	serviceNames := make(map[string]bool)
	for i, service := range config.Services {
		if err := m.validateServiceConfig(&service, i); err != nil {
			return err
		}

		// 检查服务名称重复
		if serviceNames[service.Name] {
			return errors.NewConfigError(fmt.Sprintf("duplicate service name: %s", service.Name))
		}
		serviceNames[service.Name] = true
	}

	// 验证全局配置
	if err := m.validateGlobalConfig(config); err != nil {
		return err
	}

	m.logger.Debug("Configuration validation passed")
	return nil
}

// ReloadConfig 重新加载配置
func (m *Manager) ReloadConfig(data []byte) error {
	m.logger.Info("Reloading configuration")

	newConfig, err := m.LoadConfig(data)
	if err != nil {
		return fmt.Errorf("failed to load new configuration: %w", err)
	}

	// 比较配置变化
	changes := m.compareConfigurations(m.config, newConfig)
	if len(changes) > 0 {
		m.logger.Info("Configuration changes detected", "changes", len(changes))
		for _, change := range changes {
			m.logger.Debug("Configuration change", "type", change)
		}
	}

	m.config = newConfig
	m.logger.Info("Configuration reloaded successfully")
	return nil
}

// GetServiceConfig 获取服务配置
func (m *Manager) GetServiceConfig(serviceName string) (*federationtypes.ServiceConfig, error) {
	if m.config == nil {
		return nil, errors.NewConfigError("configuration not loaded")
	}

	for _, service := range m.config.Services {
		if service.Name == serviceName {
			return &service, nil
		}
	}

	return nil, errors.NewServiceNotFoundError(serviceName)
}

// setDefaults 设置默认值
func (m *Manager) setDefaults(config *federationtypes.FederationConfig) {
	// 全局默认值
	if config.MaxQueryDepth == 0 {
		config.MaxQueryDepth = 10
	}

	if config.QueryTimeout == 0 {
		config.QueryTimeout = 30 * time.Second
	}

	// 服务默认值
	for i := range config.Services {
		service := &config.Services[i]

		if service.Timeout == 0 {
			service.Timeout = 5 * time.Second
		}

		if service.Weight == 0 {
			service.Weight = 1
		}

		if service.Headers == nil {
			service.Headers = make(map[string]string)
		}

		// 设置健康检查默认值
		if service.HealthCheck == nil {
			service.HealthCheck = &federationtypes.HealthCheck{
				Enabled:  false,
				Interval: 30 * time.Second,
				Timeout:  5 * time.Second,
				Path:     "/health",
			}
		} else {
			if service.HealthCheck.Interval == 0 {
				service.HealthCheck.Interval = 30 * time.Second
			}
			if service.HealthCheck.Timeout == 0 {
				service.HealthCheck.Timeout = 5 * time.Second
			}
			if service.HealthCheck.Path == "" {
				service.HealthCheck.Path = "/health"
			}
		}
	}
}

// validateServiceConfig 验证服务配置
func (m *Manager) validateServiceConfig(service *federationtypes.ServiceConfig, index int) error {
	prefix := fmt.Sprintf("service[%d]", index)

	// 验证服务名称
	if service.Name == "" {
		return errors.NewConfigError(fmt.Sprintf("%s: name is required", prefix))
	}

	if !utils.IsValidGraphQLName(service.Name) {
		return errors.NewConfigError(fmt.Sprintf("%s: invalid service name '%s'", prefix, service.Name))
	}

	// 验证端点
	if service.Endpoint == "" {
		return errors.NewConfigError(fmt.Sprintf("%s: endpoint is required", prefix))
	}

	if !utils.IsValidURL(service.Endpoint) {
		return errors.NewConfigError(fmt.Sprintf("%s: invalid endpoint URL '%s'", prefix, service.Endpoint))
	}

	// 验证模式
	if strings.TrimSpace(service.Schema) == "" {
		return errors.NewConfigError(fmt.Sprintf("%s: schema is required", prefix))
	}

	// 验证权重
	if service.Weight < 0 {
		return errors.NewConfigError(fmt.Sprintf("%s: weight cannot be negative", prefix))
	}

	// 验证超时
	if service.Timeout < 0 {
		return errors.NewConfigError(fmt.Sprintf("%s: timeout cannot be negative", prefix))
	}

	// 验证健康检查配置
	if service.HealthCheck != nil {
		if err := m.validateHealthCheckConfig(service.HealthCheck, prefix); err != nil {
			return err
		}
	}

	return nil
}

// validateHealthCheckConfig 验证健康检查配置
func (m *Manager) validateHealthCheckConfig(hc *federationtypes.HealthCheck, prefix string) error {
	if hc.Interval < 0 {
		return errors.NewConfigError(fmt.Sprintf("%s.healthCheck: interval cannot be negative", prefix))
	}

	if hc.Timeout < 0 {
		return errors.NewConfigError(fmt.Sprintf("%s.healthCheck: timeout cannot be negative", prefix))
	}

	if hc.Timeout > hc.Interval {
		return errors.NewConfigError(fmt.Sprintf("%s.healthCheck: timeout cannot be greater than interval", prefix))
	}

	return nil
}

// validateGlobalConfig 验证全局配置
func (m *Manager) validateGlobalConfig(config *federationtypes.FederationConfig) error {
	// 验证查询深度限制
	if config.MaxQueryDepth < 0 {
		return errors.NewConfigError("maxQueryDepth cannot be negative")
	}

	if config.MaxQueryDepth > 100 {
		return errors.NewConfigError("maxQueryDepth cannot exceed 100")
	}

	// 验证查询超时
	if config.QueryTimeout < 0 {
		return errors.NewConfigError("queryTimeout cannot be negative")
	}

	if config.QueryTimeout > 5*time.Minute {
		return errors.NewConfigError("queryTimeout cannot exceed 5 minutes")
	}

	return nil
}

// compareConfigurations 比较配置变化
func (m *Manager) compareConfigurations(old, new *federationtypes.FederationConfig) []string {
	var changes []string

	if old == nil {
		changes = append(changes, "initial_configuration")
		return changes
	}

	// 比较全局设置
	if old.EnableQueryPlan != new.EnableQueryPlan {
		changes = append(changes, "query_planning_changed")
	}

	if old.EnableCaching != new.EnableCaching {
		changes = append(changes, "caching_changed")
	}

	if old.MaxQueryDepth != new.MaxQueryDepth {
		changes = append(changes, "max_query_depth_changed")
	}

	if old.QueryTimeout != new.QueryTimeout {
		changes = append(changes, "query_timeout_changed")
	}

	// 比较服务配置
	oldServices := make(map[string]*federationtypes.ServiceConfig)
	for i := range old.Services {
		service := &old.Services[i]
		oldServices[service.Name] = service
	}

	newServices := make(map[string]*federationtypes.ServiceConfig)
	for i := range new.Services {
		service := &new.Services[i]
		newServices[service.Name] = service
	}

	// 检查新增和删除的服务
	for name := range newServices {
		if _, exists := oldServices[name]; !exists {
			changes = append(changes, fmt.Sprintf("service_added:%s", name))
		}
	}

	for name := range oldServices {
		if _, exists := newServices[name]; !exists {
			changes = append(changes, fmt.Sprintf("service_removed:%s", name))
		}
	}

	// 检查修改的服务
	for name, newService := range newServices {
		if oldService, exists := oldServices[name]; exists {
			serviceChanges := m.compareServiceConfigs(oldService, newService)
			for _, change := range serviceChanges {
				changes = append(changes, fmt.Sprintf("service_changed:%s:%s", name, change))
			}
		}
	}

	return changes
}

// compareServiceConfigs 比较服务配置
func (m *Manager) compareServiceConfigs(old, new *federationtypes.ServiceConfig) []string {
	var changes []string

	if old.Endpoint != new.Endpoint {
		changes = append(changes, "endpoint")
	}

	if old.Schema != new.Schema {
		changes = append(changes, "schema")
	}

	if old.Weight != new.Weight {
		changes = append(changes, "weight")
	}

	if old.Timeout != new.Timeout {
		changes = append(changes, "timeout")
	}

	// 比较健康检查配置
	if (old.HealthCheck == nil) != (new.HealthCheck == nil) {
		changes = append(changes, "health_check")
	} else if old.HealthCheck != nil && new.HealthCheck != nil {
		if old.HealthCheck.Enabled != new.HealthCheck.Enabled ||
			old.HealthCheck.Interval != new.HealthCheck.Interval ||
			old.HealthCheck.Timeout != new.HealthCheck.Timeout ||
			old.HealthCheck.Path != new.HealthCheck.Path {
			changes = append(changes, "health_check")
		}
	}

	return changes
}

// GetConfig 获取当前配置
func (m *Manager) GetConfig() *federationtypes.FederationConfig {
	return m.config
}

// GetServices 获取所有服务配置
func (m *Manager) GetServices() []federationtypes.ServiceConfig {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	if m.config == nil {
		return nil
	}
	return m.config.Services
}

// DefaultValidator 默认配置验证器
type DefaultValidator struct{}

// NewDefaultValidator 创建默认验证器
func NewDefaultValidator() ConfigValidator {
	return &DefaultValidator{}
}

// Validate 验证配置
func (v *DefaultValidator) Validate(config *federationtypes.FederationConfig) []ValidationError {
	var errors []ValidationError

	// 验证基本配置
	if len(config.Services) == 0 {
		errors = append(errors, ValidationError{
			Path:     "services",
			Message:  "At least one service must be configured",
			Severity: SeverityError,
			Code:     "NO_SERVICES",
		})
	}

	// 验证超时设置
	if config.QueryTimeout <= 0 {
		errors = append(errors, ValidationError{
			Path:       "queryTimeout",
			Message:    "Query timeout must be positive",
			Severity:   SeverityError,
			Code:       "INVALID_TIMEOUT",
			Suggestion: "Set queryTimeout to a positive duration like '30s'",
		})
	}

	// 验证最大深度
	if config.MaxQueryDepth <= 0 {
		errors = append(errors, ValidationError{
			Path:       "maxQueryDepth",
			Message:    "Maximum query depth must be positive",
			Severity:   SeverityWarning,
			Code:       "INVALID_DEPTH",
			Suggestion: "Set maxQueryDepth to a reasonable value like 10",
		})
	}

	// 验证服务配置
	serviceNames := make(map[string]bool)
	for i, service := range config.Services {
		path := fmt.Sprintf("services[%d]", i)

		// 检查服务名称
		if service.Name == "" {
			errors = append(errors, ValidationError{
				Path:     path + ".name",
				Message:  "Service name cannot be empty",
				Severity: SeverityError,
				Code:     "EMPTY_SERVICE_NAME",
			})
		} else if serviceNames[service.Name] {
			errors = append(errors, ValidationError{
				Path:     path + ".name",
				Message:  fmt.Sprintf("Duplicate service name: %s", service.Name),
				Severity: SeverityError,
				Code:     "DUPLICATE_SERVICE_NAME",
			})
		} else {
			serviceNames[service.Name] = true
		}

		// 检查服务端点
		if service.Endpoint == "" {
			errors = append(errors, ValidationError{
				Path:     path + ".endpoint",
				Message:  "Service endpoint cannot be empty",
				Severity: SeverityError,
				Code:     "EMPTY_ENDPOINT",
			})
		} else if !utils.IsValidURL(service.Endpoint) {
			errors = append(errors, ValidationError{
				Path:     path + ".endpoint",
				Message:  "Invalid endpoint URL format",
				Severity: SeverityError,
				Code:     "INVALID_ENDPOINT_URL",
			})
		}

		// 检查超时设置
		if service.Timeout <= 0 {
			errors = append(errors, ValidationError{
				Path:       path + ".timeout",
				Message:    "Service timeout must be positive",
				Severity:   SeverityWarning,
				Code:       "INVALID_SERVICE_TIMEOUT",
				Suggestion: "Set timeout to a positive duration like '5s'",
			})
		}

		// 检查权重
		if service.Weight <= 0 {
			errors = append(errors, ValidationError{
				Path:       path + ".weight",
				Message:    "Service weight must be positive",
				Severity:   SeverityWarning,
				Code:       "INVALID_WEIGHT",
				Suggestion: "Set weight to a positive integer like 1",
			})
		}
	}

	return errors
}

// GetName 获取验证器名称
func (v *DefaultValidator) GetName() string {
	return "DefaultValidator"
}

// DefaultChangeDetector 默认变更检测器
type DefaultChangeDetector struct{}

// NewDefaultChangeDetector 创建默认变更检测器
func NewDefaultChangeDetector() ChangeDetector {
	return &DefaultChangeDetector{}
}

// DetectChanges 检测配置变更
func (d *DefaultChangeDetector) DetectChanges(oldConfig, newConfig *federationtypes.FederationConfig) []ConfigChange {
	var changes []ConfigChange

	if oldConfig == nil {
		return changes
	}

	// 检测全局配置变更
	if oldConfig.EnableQueryPlan != newConfig.EnableQueryPlan {
		changes = append(changes, ConfigChange{
			Type:        ChangeTypeModified,
			Path:        "enableQueryPlan",
			OldValue:    oldConfig.EnableQueryPlan,
			NewValue:    newConfig.EnableQueryPlan,
			Description: "Query planning configuration changed",
		})
	}

	if oldConfig.EnableCaching != newConfig.EnableCaching {
		changes = append(changes, ConfigChange{
			Type:        ChangeTypeModified,
			Path:        "enableCaching",
			OldValue:    oldConfig.EnableCaching,
			NewValue:    newConfig.EnableCaching,
			Description: "Caching configuration changed",
		})
	}

	if oldConfig.MaxQueryDepth != newConfig.MaxQueryDepth {
		changes = append(changes, ConfigChange{
			Type:        ChangeTypeModified,
			Path:        "maxQueryDepth",
			OldValue:    oldConfig.MaxQueryDepth,
			NewValue:    newConfig.MaxQueryDepth,
			Description: "Maximum query depth changed",
		})
	}

	if oldConfig.QueryTimeout != newConfig.QueryTimeout {
		changes = append(changes, ConfigChange{
			Type:        ChangeTypeModified,
			Path:        "queryTimeout",
			OldValue:    oldConfig.QueryTimeout,
			NewValue:    newConfig.QueryTimeout,
			Description: "Query timeout changed",
		})
	}

	// 检测服务变更
	oldServices := make(map[string]federationtypes.ServiceConfig)
	for _, service := range oldConfig.Services {
		oldServices[service.Name] = service
	}

	newServices := make(map[string]federationtypes.ServiceConfig)
	for _, service := range newConfig.Services {
		newServices[service.Name] = service
	}

	// 检测新增的服务
	for name, service := range newServices {
		if _, exists := oldServices[name]; !exists {
			changes = append(changes, ConfigChange{
				Type:        ChangeTypeAdded,
				Path:        fmt.Sprintf("services.%s", name),
				NewValue:    service.Endpoint,
				Description: fmt.Sprintf("Service %s added", name),
			})
		}
	}

	// 检测删除的服务
	for name, service := range oldServices {
		if _, exists := newServices[name]; !exists {
			changes = append(changes, ConfigChange{
				Type:        ChangeTypeRemoved,
				Path:        fmt.Sprintf("services.%s", name),
				OldValue:    service.Endpoint,
				Description: fmt.Sprintf("Service %s removed", name),
			})
		}
	}

	// 检测修改的服务
	for name, newService := range newServices {
		if oldService, exists := oldServices[name]; exists {
			serviceChanges := d.compareServices(name, oldService, newService)
			changes = append(changes, serviceChanges...)
		}
	}

	return changes
}

// compareServices 比较服务配置
func (d *DefaultChangeDetector) compareServices(name string, oldService, newService federationtypes.ServiceConfig) []ConfigChange {
	var changes []ConfigChange
	basePath := fmt.Sprintf("services.%s", name)

	if oldService.Endpoint != newService.Endpoint {
		changes = append(changes, ConfigChange{
			Type:        ChangeTypeModified,
			Path:        basePath + ".endpoint",
			OldValue:    oldService.Endpoint,
			NewValue:    newService.Endpoint,
			Description: fmt.Sprintf("Service %s endpoint changed", name),
		})
	}

	if oldService.Schema != newService.Schema {
		changes = append(changes, ConfigChange{
			Type:        ChangeTypeModified,
			Path:        basePath + ".schema",
			OldValue:    "[schema]",
			NewValue:    "[schema]",
			Description: fmt.Sprintf("Service %s schema changed", name),
		})
	}

	if oldService.Weight != newService.Weight {
		changes = append(changes, ConfigChange{
			Type:        ChangeTypeModified,
			Path:        basePath + ".weight",
			OldValue:    oldService.Weight,
			NewValue:    newService.Weight,
			Description: fmt.Sprintf("Service %s weight changed", name),
		})
	}

	if oldService.Timeout != newService.Timeout {
		changes = append(changes, ConfigChange{
			Type:        ChangeTypeModified,
			Path:        basePath + ".timeout",
			OldValue:    oldService.Timeout,
			NewValue:    newService.Timeout,
			Description: fmt.Sprintf("Service %s timeout changed", name),
		})
	}

	return changes
}

// GetName 获取检测器名称
func (d *DefaultChangeDetector) GetName() string {
	return "DefaultChangeDetector"
}

// IsServiceEnabled 检查服务是否启用
func (m *Manager) IsServiceEnabled(serviceName string) bool {
	service, err := m.GetServiceConfig(serviceName)
	if err != nil {
		return false
	}

	// 完整的启用检查逻辑
	// 1. 检查权重是否大于0
	if service.Weight <= 0 {
		return false
	}

	// 2. 检查端点是否有效
	if service.Endpoint == "" {
		return false
	}

	// 3. 检查超时配置是否合理
	if service.Timeout <= 0 {
		return false
	}

	// 4. 检查是否在禁用列表中
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	if m.config != nil {
		// 检查全局禁用列表（如果存在）
		for _, disabledService := range m.getDisabledServices() {
			if disabledService == serviceName {
				return false
			}
		}
	}

	// 5. 检查服务健康状态（如果有健康检查器）
	return m.checkServiceHealth(serviceName)
}

// getDisabledServices 获取禁用的服务列表
func (m *Manager) getDisabledServices() []string {
	// 这里可以从配置中读取禁用的服务列表
	// 暂时返回空列表
	return []string{}
}

// checkServiceHealth 检查服务健康状态
func (m *Manager) checkServiceHealth(serviceName string) bool {
	// 这里可以实现实际的健康检查逻辑
	// 暂时返回true，表示服务健康
	return true
}

// GetServiceWeight 获取服务权重
func (m *Manager) GetServiceWeight(serviceName string) int {
	service, err := m.GetServiceConfig(serviceName)
	if err != nil {
		return 0
	}
	return service.Weight
}

// GetServiceTimeout 获取服务超时时间
func (m *Manager) GetServiceTimeout(serviceName string) time.Duration {
	service, err := m.GetServiceConfig(serviceName)
	if err != nil {
		return 5 * time.Second // 默认超时
	}
	return service.Timeout
}
