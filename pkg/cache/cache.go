package cache

import (
	"envoy-wasm-graphql-federation/pkg/jsonutil"
	"fmt"
	"reflect"
	"strings"
	"sync"
	"time"

	federationtypes "envoy-wasm-graphql-federation/pkg/types"
)

// Cache 缓存接口定义
type Cache interface {
	// 查询结果缓存
	GetQuery(key string) (*federationtypes.GraphQLResponse, bool)
	SetQuery(key string, response *federationtypes.GraphQLResponse, ttl time.Duration) error
	InvalidateQuery(pattern string) error

	// 模式缓存
	GetSchema(serviceName string) (*federationtypes.Schema, bool)
	SetSchema(serviceName string, schema *federationtypes.Schema, ttl time.Duration) error
	InvalidateSchema(serviceName string) error

	// 执行计划缓存
	GetPlan(key string) (*federationtypes.ExecutionPlan, bool)
	SetPlan(key string, plan *federationtypes.ExecutionPlan, ttl time.Duration) error
	InvalidatePlan(pattern string) error

	// 通用操作
	Clear() error
	Size() int
	Stats() CacheStats
}

// CacheConfig 缓存配置
type CacheConfig struct {
	// 基本配置
	Enabled         bool          `json:"enabled"`
	DefaultTTL      time.Duration `json:"defaultTTL"`
	MaxSize         int           `json:"maxSize"`
	CleanupInterval time.Duration `json:"cleanupInterval"`

	// 查询缓存配置
	QueryCache QueryCacheConfig `json:"queryCache"`

	// 模式缓存配置
	SchemaCache SchemaCacheConfig `json:"schemaCache"`

	// 计划缓存配置
	PlanCache PlanCacheConfig `json:"planCache"`

	// 性能配置
	EnableMetrics     bool `json:"enableMetrics"`
	EnableCompression bool `json:"enableCompression"`
}

// QueryCacheConfig 查询缓存配置
type QueryCacheConfig struct {
	Enabled    bool          `json:"enabled"`
	TTL        time.Duration `json:"ttl"`
	MaxSize    int           `json:"maxSize"`
	MaxKeySize int           `json:"maxKeySize"`
}

// SchemaCacheConfig 模式缓存配置
type SchemaCacheConfig struct {
	Enabled bool          `json:"enabled"`
	TTL     time.Duration `json:"ttl"`
	MaxSize int           `json:"maxSize"`
}

// PlanCacheConfig 计划缓存配置
type PlanCacheConfig struct {
	Enabled bool          `json:"enabled"`
	TTL     time.Duration `json:"ttl"`
	MaxSize int           `json:"maxSize"`
}

// CacheStats 缓存统计信息
type CacheStats struct {
	// 总体统计
	TotalHits   int64 `json:"totalHits"`
	TotalMisses int64 `json:"totalMisses"`
	TotalSets   int64 `json:"totalSets"`
	TotalEvicts int64 `json:"totalEvicts"`

	// 查询缓存统计
	QueryHits   int64 `json:"queryHits"`
	QueryMisses int64 `json:"queryMisses"`
	QuerySets   int64 `json:"querySets"`

	// 模式缓存统计
	SchemaHits   int64 `json:"schemaHits"`
	SchemaMisses int64 `json:"schemaMisses"`
	SchemaSets   int64 `json:"schemaSets"`

	// 计划缓存统计
	PlanHits   int64 `json:"planHits"`
	PlanMisses int64 `json:"planMisses"`
	PlanSets   int64 `json:"planSets"`

	// 性能统计
	HitRate     float64   `json:"hitRate"`
	Size        int       `json:"size"`
	LastCleanup time.Time `json:"lastCleanup"`
}

// CacheEntry 缓存条目
type CacheEntry struct {
	Key         string      `json:"key"`
	Value       interface{} `json:"value"`
	ExpiresAt   time.Time   `json:"expiresAt"`
	CreatedAt   time.Time   `json:"createdAt"`
	AccessedAt  time.Time   `json:"accessedAt"`
	AccessCount int64       `json:"accessCount"`
	Size        int         `json:"size"`
}

// MemoryCache 内存缓存实现
type MemoryCache struct {
	config *CacheConfig
	logger federationtypes.Logger
	mutex  sync.RWMutex

	// 分离的缓存存储
	queryCache  map[string]*CacheEntry
	schemaCache map[string]*CacheEntry
	planCache   map[string]*CacheEntry

	// 统计信息
	stats CacheStats

	// 清理相关
	cleanupTicker *time.Ticker
	stopCleanup   chan bool
}

// NewMemoryCache 创建新的内存缓存
func NewMemoryCache(config *CacheConfig, logger federationtypes.Logger) Cache {
	if config == nil {
		config = DefaultCacheConfig()
	}

	cache := &MemoryCache{
		config:      config,
		logger:      logger,
		queryCache:  make(map[string]*CacheEntry),
		schemaCache: make(map[string]*CacheEntry),
		planCache:   make(map[string]*CacheEntry),
		stats:       CacheStats{},
		stopCleanup: make(chan bool),
	}

	// 启动清理协程
	if config.CleanupInterval > 0 {
		cache.startCleanup()
	}

	return cache
}

// DefaultCacheConfig 返回默认缓存配置
func DefaultCacheConfig() *CacheConfig {
	return &CacheConfig{
		Enabled:         true,
		DefaultTTL:      5 * time.Minute,
		MaxSize:         1000,
		CleanupInterval: 1 * time.Minute,
		QueryCache: QueryCacheConfig{
			Enabled:    true,
			TTL:        2 * time.Minute,
			MaxSize:    500,
			MaxKeySize: 1024,
		},
		SchemaCache: SchemaCacheConfig{
			Enabled: true,
			TTL:     10 * time.Minute,
			MaxSize: 100,
		},
		PlanCache: PlanCacheConfig{
			Enabled: true,
			TTL:     5 * time.Minute,
			MaxSize: 200,
		},
		EnableMetrics:     true,
		EnableCompression: false,
	}
}

// GetQuery 获取查询结果
func (c *MemoryCache) GetQuery(key string) (*federationtypes.GraphQLResponse, bool) {
	if !c.config.Enabled || !c.config.QueryCache.Enabled {
		return nil, false
	}

	c.mutex.RLock()
	defer c.mutex.RUnlock()

	entry, exists := c.queryCache[key]
	if !exists {
		c.stats.QueryMisses++
		c.stats.TotalMisses++
		return nil, false
	}

	// 检查是否过期
	if time.Now().After(entry.ExpiresAt) {
		// 延迟删除，在下次清理时处理
		c.stats.QueryMisses++
		c.stats.TotalMisses++
		return nil, false
	}

	// 更新访问信息
	entry.AccessedAt = time.Now()
	entry.AccessCount++

	// 统计命中
	c.stats.QueryHits++
	c.stats.TotalHits++

	if response, ok := entry.Value.(*federationtypes.GraphQLResponse); ok {
		c.logger.Debug("Query cache hit", "key", c.truncateKey(key))
		return response, true
	}

	return nil, false
}

// SetQuery 设置查询结果
func (c *MemoryCache) SetQuery(key string, response *federationtypes.GraphQLResponse, ttl time.Duration) error {
	if !c.config.Enabled || !c.config.QueryCache.Enabled {
		return nil
	}

	if len(key) > c.config.QueryCache.MaxKeySize {
		return fmt.Errorf("key size %d exceeds maximum %d", len(key), c.config.QueryCache.MaxKeySize)
	}

	c.mutex.Lock()
	defer c.mutex.Unlock()

	// 检查容量
	if len(c.queryCache) >= c.config.QueryCache.MaxSize {
		c.evictOldestQuery()
	}

	// 计算过期时间
	if ttl <= 0 {
		ttl = c.config.QueryCache.TTL
	}

	// 创建缓存条目
	entry := &CacheEntry{
		Key:         key,
		Value:       response,
		ExpiresAt:   time.Now().Add(ttl),
		CreatedAt:   time.Now(),
		AccessedAt:  time.Now(),
		AccessCount: 0,
		Size:        c.calculateSize(response),
	}

	c.queryCache[key] = entry
	c.stats.QuerySets++
	c.stats.TotalSets++

	c.logger.Debug("Query cached", "key", c.truncateKey(key), "ttl", ttl)
	return nil
}

// InvalidateQuery 使查询缓存失效
func (c *MemoryCache) InvalidateQuery(pattern string) error {
	if !c.config.Enabled || !c.config.QueryCache.Enabled {
		return nil
	}

	c.mutex.Lock()
	defer c.mutex.Unlock()

	var toDelete []string
	for key := range c.queryCache {
		if c.matchPattern(key, pattern) {
			toDelete = append(toDelete, key)
		}
	}

	for _, key := range toDelete {
		delete(c.queryCache, key)
		c.stats.TotalEvicts++
	}

	c.logger.Debug("Query cache invalidated", "pattern", pattern, "count", len(toDelete))
	return nil
}

// GetSchema 获取模式
func (c *MemoryCache) GetSchema(serviceName string) (*federationtypes.Schema, bool) {
	if !c.config.Enabled || !c.config.SchemaCache.Enabled {
		return nil, false
	}

	c.mutex.RLock()
	defer c.mutex.RUnlock()

	entry, exists := c.schemaCache[serviceName]
	if !exists {
		c.stats.SchemaMisses++
		c.stats.TotalMisses++
		return nil, false
	}

	// 检查是否过期
	if time.Now().After(entry.ExpiresAt) {
		c.stats.SchemaMisses++
		c.stats.TotalMisses++
		return nil, false
	}

	// 更新访问信息
	entry.AccessedAt = time.Now()
	entry.AccessCount++

	// 统计命中
	c.stats.SchemaHits++
	c.stats.TotalHits++

	if schema, ok := entry.Value.(*federationtypes.Schema); ok {
		c.logger.Debug("Schema cache hit", "service", serviceName)
		return schema, true
	}

	return nil, false
}

// SetSchema 设置模式
func (c *MemoryCache) SetSchema(serviceName string, schema *federationtypes.Schema, ttl time.Duration) error {
	if !c.config.Enabled || !c.config.SchemaCache.Enabled {
		return nil
	}

	c.mutex.Lock()
	defer c.mutex.Unlock()

	// 检查容量
	if len(c.schemaCache) >= c.config.SchemaCache.MaxSize {
		c.evictOldestSchema()
	}

	// 计算过期时间
	if ttl <= 0 {
		ttl = c.config.SchemaCache.TTL
	}

	// 创建缓存条目
	entry := &CacheEntry{
		Key:         serviceName,
		Value:       schema,
		ExpiresAt:   time.Now().Add(ttl),
		CreatedAt:   time.Now(),
		AccessedAt:  time.Now(),
		AccessCount: 0,
		Size:        c.calculateSize(schema),
	}

	c.schemaCache[serviceName] = entry
	c.stats.SchemaSets++
	c.stats.TotalSets++

	c.logger.Debug("Schema cached", "service", serviceName, "ttl", ttl)
	return nil
}

// InvalidateSchema 使模式缓存失效
func (c *MemoryCache) InvalidateSchema(serviceName string) error {
	if !c.config.Enabled || !c.config.SchemaCache.Enabled {
		return nil
	}

	c.mutex.Lock()
	defer c.mutex.Unlock()

	if _, exists := c.schemaCache[serviceName]; exists {
		delete(c.schemaCache, serviceName)
		c.stats.TotalEvicts++
		c.logger.Debug("Schema cache invalidated", "service", serviceName)
	}

	return nil
}

// GetPlan 获取执行计划
func (c *MemoryCache) GetPlan(key string) (*federationtypes.ExecutionPlan, bool) {
	if !c.config.Enabled || !c.config.PlanCache.Enabled {
		return nil, false
	}

	c.mutex.RLock()
	defer c.mutex.RUnlock()

	entry, exists := c.planCache[key]
	if !exists {
		c.stats.PlanMisses++
		c.stats.TotalMisses++
		return nil, false
	}

	// 检查是否过期
	if time.Now().After(entry.ExpiresAt) {
		c.stats.PlanMisses++
		c.stats.TotalMisses++
		return nil, false
	}

	// 更新访问信息
	entry.AccessedAt = time.Now()
	entry.AccessCount++

	// 统计命中
	c.stats.PlanHits++
	c.stats.TotalHits++

	if plan, ok := entry.Value.(*federationtypes.ExecutionPlan); ok {
		c.logger.Debug("Plan cache hit", "key", c.truncateKey(key))
		return plan, true
	}

	return nil, false
}

// SetPlan 设置执行计划
func (c *MemoryCache) SetPlan(key string, plan *federationtypes.ExecutionPlan, ttl time.Duration) error {
	if !c.config.Enabled || !c.config.PlanCache.Enabled {
		return nil
	}

	c.mutex.Lock()
	defer c.mutex.Unlock()

	// 检查容量
	if len(c.planCache) >= c.config.PlanCache.MaxSize {
		c.evictOldestPlan()
	}

	// 计算过期时间
	if ttl <= 0 {
		ttl = c.config.PlanCache.TTL
	}

	// 创建缓存条目
	entry := &CacheEntry{
		Key:         key,
		Value:       plan,
		ExpiresAt:   time.Now().Add(ttl),
		CreatedAt:   time.Now(),
		AccessedAt:  time.Now(),
		AccessCount: 0,
		Size:        c.calculateSize(plan),
	}

	c.planCache[key] = entry
	c.stats.PlanSets++
	c.stats.TotalSets++

	c.logger.Debug("Plan cached", "key", c.truncateKey(key), "ttl", ttl)
	return nil
}

// InvalidatePlan 使执行计划缓存失效
func (c *MemoryCache) InvalidatePlan(pattern string) error {
	if !c.config.Enabled || !c.config.PlanCache.Enabled {
		return nil
	}

	c.mutex.Lock()
	defer c.mutex.Unlock()

	var toDelete []string
	for key := range c.planCache {
		if c.matchPattern(key, pattern) {
			toDelete = append(toDelete, key)
		}
	}

	for _, key := range toDelete {
		delete(c.planCache, key)
		c.stats.TotalEvicts++
	}

	c.logger.Debug("Plan cache invalidated", "pattern", pattern, "count", len(toDelete))
	return nil
}

// Clear 清空所有缓存
func (c *MemoryCache) Clear() error {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	queryCount := len(c.queryCache)
	schemaCount := len(c.schemaCache)
	planCount := len(c.planCache)

	c.queryCache = make(map[string]*CacheEntry)
	c.schemaCache = make(map[string]*CacheEntry)
	c.planCache = make(map[string]*CacheEntry)

	totalEvicted := queryCount + schemaCount + planCount
	c.stats.TotalEvicts += int64(totalEvicted)

	c.logger.Info("Cache cleared",
		"queryEntries", queryCount,
		"schemaEntries", schemaCount,
		"planEntries", planCount,
	)

	return nil
}

// Size 获取缓存大小
func (c *MemoryCache) Size() int {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	return len(c.queryCache) + len(c.schemaCache) + len(c.planCache)
}

// Stats 获取缓存统计信息
func (c *MemoryCache) Stats() CacheStats {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	// 计算命中率
	totalOperations := c.stats.TotalHits + c.stats.TotalMisses
	if totalOperations > 0 {
		c.stats.HitRate = float64(c.stats.TotalHits) / float64(totalOperations)
	}

	c.stats.Size = len(c.queryCache) + len(c.schemaCache) + len(c.planCache)

	// 返回统计信息副本
	return CacheStats{
		TotalHits:    c.stats.TotalHits,
		TotalMisses:  c.stats.TotalMisses,
		TotalSets:    c.stats.TotalSets,
		TotalEvicts:  c.stats.TotalEvicts,
		QueryHits:    c.stats.QueryHits,
		QueryMisses:  c.stats.QueryMisses,
		QuerySets:    c.stats.QuerySets,
		SchemaHits:   c.stats.SchemaHits,
		SchemaMisses: c.stats.SchemaMisses,
		SchemaSets:   c.stats.SchemaSets,
		PlanHits:     c.stats.PlanHits,
		PlanMisses:   c.stats.PlanMisses,
		PlanSets:     c.stats.PlanSets,
		HitRate:      c.stats.HitRate,
		Size:         c.stats.Size,
		LastCleanup:  c.stats.LastCleanup,
	}
}

// 私有方法

// startCleanup 启动清理协程
func (c *MemoryCache) startCleanup() {
	c.cleanupTicker = time.NewTicker(c.config.CleanupInterval)

	go func() {
		for {
			select {
			case <-c.cleanupTicker.C:
				c.cleanup()
			case <-c.stopCleanup:
				c.cleanupTicker.Stop()
				return
			}
		}
	}()
}

// cleanup 清理过期条目
func (c *MemoryCache) cleanup() {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	now := time.Now()
	evicted := 0

	// 清理查询缓存
	for key, entry := range c.queryCache {
		if now.After(entry.ExpiresAt) {
			delete(c.queryCache, key)
			evicted++
		}
	}

	// 清理模式缓存
	for key, entry := range c.schemaCache {
		if now.After(entry.ExpiresAt) {
			delete(c.schemaCache, key)
			evicted++
		}
	}

	// 清理计划缓存
	for key, entry := range c.planCache {
		if now.After(entry.ExpiresAt) {
			delete(c.planCache, key)
			evicted++
		}
	}

	c.stats.TotalEvicts += int64(evicted)
	c.stats.LastCleanup = now

	if evicted > 0 {
		c.logger.Debug("Cache cleanup completed", "evicted", evicted)
	}
}

// evictOldestQuery 驱逐最老的查询缓存
func (c *MemoryCache) evictOldestQuery() {
	var oldestKey string
	var oldestTime time.Time

	for key, entry := range c.queryCache {
		if oldestKey == "" || entry.AccessedAt.Before(oldestTime) {
			oldestKey = key
			oldestTime = entry.AccessedAt
		}
	}

	if oldestKey != "" {
		delete(c.queryCache, oldestKey)
		c.stats.TotalEvicts++
	}
}

// evictOldestSchema 驱逐最老的模式缓存
func (c *MemoryCache) evictOldestSchema() {
	var oldestKey string
	var oldestTime time.Time

	for key, entry := range c.schemaCache {
		if oldestKey == "" || entry.AccessedAt.Before(oldestTime) {
			oldestKey = key
			oldestTime = entry.AccessedAt
		}
	}

	if oldestKey != "" {
		delete(c.schemaCache, oldestKey)
		c.stats.TotalEvicts++
	}
}

// evictOldestPlan 驱逐最老的计划缓存
func (c *MemoryCache) evictOldestPlan() {
	var oldestKey string
	var oldestTime time.Time

	for key, entry := range c.planCache {
		if oldestKey == "" || entry.AccessedAt.Before(oldestTime) {
			oldestKey = key
			oldestTime = entry.AccessedAt
		}
	}

	if oldestKey != "" {
		delete(c.planCache, oldestKey)
		c.stats.TotalEvicts++
	}
}

// matchPattern 匹配模式
func (c *MemoryCache) matchPattern(key, pattern string) bool {
	// 完整的模式匹配，支持多个通配符 *
	if pattern == "*" {
		return true
	}

	if !strings.Contains(pattern, "*") {
		// 精确匹配
		return key == pattern
	}

	// 处理通配符匹配
	patternParts := strings.Split(pattern, "*")
	if len(patternParts) == 1 {
		return key == pattern
	}

	// 检查开头
	if len(patternParts[0]) > 0 && !strings.HasPrefix(key, patternParts[0]) {
		return false
	}

	// 检查结尾
	lastPart := patternParts[len(patternParts)-1]
	if len(lastPart) > 0 && !strings.HasSuffix(key, lastPart) {
		return false
	}

	// 检查中间部分
	currentPos := 0
	if len(patternParts[0]) > 0 {
		currentPos = len(patternParts[0])
	}

	for i := 1; i < len(patternParts)-1; i++ {
		part := patternParts[i]
		if len(part) == 0 {
			continue
		}

		index := strings.Index(key[currentPos:], part)
		if index == -1 {
			return false
		}

		currentPos += index + len(part)
	}

	return true
}

// calculateSize 计算对象大小
func (c *MemoryCache) calculateSize(obj interface{}) int {
	if obj == nil {
		return 0
	}

	// 尝试多种方法计算大小
	size := c.calculateSizeByType(obj)
	if size > 0 {
		return size
	}

	// 备用方法：使用JSON序列化
	if data, err := jsonutil.Marshal(obj); err == nil {
		return len(data)
	}

	// 最后的备用方法：估算大小
	return c.estimateSize(obj)
}

// calculateSizeByType 根据类型计算大小
func (c *MemoryCache) calculateSizeByType(obj interface{}) int {
	switch v := obj.(type) {
	case string:
		return len(v)
	case []byte:
		return len(v)
	case int, int8, int16, int32, int64:
		return 8
	case uint, uint8, uint16, uint32, uint64:
		return 8
	case float32:
		return 4
	case float64:
		return 8
	case bool:
		return 1
	case map[string]interface{}:
		return c.calculateMapSize(v)
	case []interface{}:
		return c.calculateSliceSize(v)
	default:
		return 0 // 未知类型
	}
}

// calculateMapSize 计算map大小
func (c *MemoryCache) calculateMapSize(m map[string]interface{}) int {
	size := 0
	for key, value := range m {
		size += len(key)               // 键的大小
		size += c.calculateSize(value) // 值的大小
		size += 16                     // map条目的开销
	}
	return size
}

// calculateSliceSize 计算slice大小
func (c *MemoryCache) calculateSliceSize(s []interface{}) int {
	size := 0
	for _, item := range s {
		size += c.calculateSize(item)
		size += 8 // slice元素的开销
	}
	return size
}

// estimateSize 估算对象大小
func (c *MemoryCache) estimateSize(obj interface{}) int {
	// 使用反射获取类型信息
	v := reflect.ValueOf(obj)
	if !v.IsValid() {
		return 0
	}

	switch v.Kind() {
	case reflect.String:
		return v.Len()
	case reflect.Slice, reflect.Array:
		size := v.Len() * 8 // 每个元素的基本开销
		for i := 0; i < v.Len(); i++ {
			if i < 10 { // 限制递归深度
				size += c.estimateSize(v.Index(i).Interface())
			} else {
				size += 32 // 估算值
			}
		}
		return size
	case reflect.Map:
		size := v.Len() * 16 // 每个map条目的开销
		keys := v.MapKeys()
		for i, key := range keys {
			if i < 10 { // 限制递归深度
				size += c.estimateSize(key.Interface())
				size += c.estimateSize(v.MapIndex(key).Interface())
			} else {
				size += 64 // 估算值
			}
		}
		return size
	case reflect.Ptr:
		if v.IsNil() {
			return 8
		}
		return 8 + c.estimateSize(v.Elem().Interface())
	case reflect.Struct:
		size := 0
		for i := 0; i < v.NumField(); i++ {
			field := v.Field(i)
			if field.CanInterface() {
				size += c.estimateSize(field.Interface())
			} else {
				size += 8 // 估算不可访问字段的大小
			}
		}
		return size
	default:
		return 32 // 默认估算大小
	}
}

// truncateKey 截断键名用于日志
func (c *MemoryCache) truncateKey(key string) string {
	const maxLen = 50
	if len(key) <= maxLen {
		return key
	}
	return key[:maxLen] + "..."
}

// 缓存键生成器

// CacheKeyGenerator 缓存键生成器
type CacheKeyGenerator struct{}

// NewCacheKeyGenerator 创建缓存键生成器
func NewCacheKeyGenerator() *CacheKeyGenerator {
	return &CacheKeyGenerator{}
}

// GenerateQueryKey 生成查询缓存键（TinyGo兼容版本）
func (g *CacheKeyGenerator) GenerateQueryKey(query string, variables map[string]interface{}, operationName string) string {
	// 使用简单的字符串组合方式生成唯一键
	// 标准化查询（移除空白字符）
	normalizedQuery := strings.ReplaceAll(strings.ReplaceAll(query, " ", ""), "\n", "")

	// 简化哈希：使用字符串长度和简单校验和
	hashValue := len(normalizedQuery)
	for _, char := range normalizedQuery {
		hashValue = hashValue*31 + int(char)
	}

	// 添加变量信息
	if variables != nil {
		if varData, err := jsonutil.Marshal(variables); err == nil {
			for _, b := range varData {
				hashValue = hashValue*31 + int(b)
			}
		}
	}

	// 添加操作名
	if operationName != "" {
		for _, char := range operationName {
			hashValue = hashValue*31 + int(char)
		}
	}

	return fmt.Sprintf("query:%x", uint32(hashValue)) // 使用uint32避免溢出
}

// GeneratePlanKey 生成执行计划缓存键（TinyGo兼容版本）
func (g *CacheKeyGenerator) GeneratePlanKey(query string, services []string) string {
	// 使用简单的哈希算法
	hashValue := 0

	// 标准化查询
	normalizedQuery := strings.ReplaceAll(strings.ReplaceAll(query, " ", ""), "\n", "")
	for _, char := range normalizedQuery {
		hashValue = hashValue*31 + int(char)
	}

	// 添加服务信息
	for _, service := range services {
		for _, char := range service {
			hashValue = hashValue*31 + int(char)
		}
	}

	return fmt.Sprintf("plan:%x", uint32(hashValue))
}

// GenerateSchemaKey 生成模式缓存键
func (g *CacheKeyGenerator) GenerateSchemaKey(serviceName string, version string) string {
	if version != "" {
		return fmt.Sprintf("schema:%s:%s", serviceName, version)
	}
	return fmt.Sprintf("schema:%s", serviceName)
}
