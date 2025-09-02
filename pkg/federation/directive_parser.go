package federation

import (
	"fmt"
	"regexp"
	"strings"

	"envoy-wasm-graphql-federation/pkg/errors"
	federationtypes "envoy-wasm-graphql-federation/pkg/types"
)

// DirectiveParser 实现 Federation 指令解析器
type DirectiveParser struct {
	logger federationtypes.Logger
}

// NewDirectiveParser 创建新的指令解析器
func NewDirectiveParser(logger federationtypes.Logger) federationtypes.FederationDirectiveParser {
	return &DirectiveParser{
		logger: logger,
	}
}

// ParseDirectives 解析类型上的 Federation 指令
func (p *DirectiveParser) ParseDirectives(typeDef string) (*federationtypes.EntityDirectives, error) {
	if strings.TrimSpace(typeDef) == "" {
		return nil, errors.NewParsingError("type definition cannot be empty")
	}

	p.logger.Debug("Parsing Federation directives", "typeDef", p.truncateString(typeDef, 100))

	directives := &federationtypes.EntityDirectives{}

	// 解析 @key 指令
	keys, err := p.parseKeyDirectives(typeDef)
	if err != nil {
		return nil, fmt.Errorf("failed to parse @key directives: %w", err)
	}
	directives.Keys = keys

	// 解析 @external 指令
	external, err := p.parseExternalDirective(typeDef)
	if err != nil {
		return nil, fmt.Errorf("failed to parse @external directive: %w", err)
	}
	directives.External = external

	// 解析 @requires 指令
	requires, err := p.parseRequiresDirective(typeDef)
	if err != nil {
		return nil, fmt.Errorf("failed to parse @requires directive: %w", err)
	}
	directives.Requires = requires

	// 解析 @provides 指令
	provides, err := p.parseProvidesDirective(typeDef)
	if err != nil {
		return nil, fmt.Errorf("failed to parse @provides directive: %w", err)
	}
	directives.Provides = provides

	return directives, nil
}

// ParseKeyDirective 解析 @key 指令
func (p *DirectiveParser) ParseKeyDirective(directive string) (*federationtypes.KeyDirective, error) {
	if strings.TrimSpace(directive) == "" {
		return nil, errors.NewParsingError("directive cannot be empty")
	}

	// 正则表达式匹配 @key(fields: "field1 field2", resolvable: true)
	keyRegex := regexp.MustCompile(`@key\s*\(\s*fields\s*:\s*"([^"]+)"(?:\s*,\s*resolvable\s*:\s*(true|false))?\s*\)`)
	matches := keyRegex.FindStringSubmatch(directive)

	if len(matches) < 2 {
		return nil, fmt.Errorf("invalid @key directive format: %s", directive)
	}

	keyDirective := &federationtypes.KeyDirective{
		Fields:     strings.TrimSpace(matches[1]),
		Resolvable: true, // 默认值
	}

	// 解析 resolvable 参数
	if len(matches) > 2 && matches[2] != "" {
		keyDirective.Resolvable = matches[2] == "true"
	}

	// 验证字段选择集格式
	if err := p.validateFieldSelection(keyDirective.Fields); err != nil {
		return nil, fmt.Errorf("invalid key fields: %w", err)
	}

	return keyDirective, nil
}

// ParseExternalDirective 解析 @external 指令
func (p *DirectiveParser) ParseExternalDirective(directive string) (*federationtypes.ExternalDirective, error) {
	if strings.TrimSpace(directive) == "" {
		return nil, errors.NewParsingError("directive cannot be empty")
	}

	// 正则表达式匹配 @external 或 @external(reason: "explanation")
	externalRegex := regexp.MustCompile(`@external(?:\s*\(\s*reason\s*:\s*"([^"]+)"\s*\))?`)
	matches := externalRegex.FindStringSubmatch(directive)

	if len(matches) == 0 {
		return nil, fmt.Errorf("invalid @external directive format: %s", directive)
	}

	externalDirective := &federationtypes.ExternalDirective{}

	// 如果有 reason 参数
	if len(matches) > 1 && matches[1] != "" {
		externalDirective.Reason = matches[1]
	}

	return externalDirective, nil
}

// ParseRequiresDirective 解析 @requires 指令
func (p *DirectiveParser) ParseRequiresDirective(directive string) (*federationtypes.RequiresDirective, error) {
	if strings.TrimSpace(directive) == "" {
		return nil, errors.NewParsingError("directive cannot be empty")
	}

	// 正则表达式匹配 @requires(fields: "field1 field2")
	requiresRegex := regexp.MustCompile(`@requires\s*\(\s*fields\s*:\s*"([^"]+)"\s*\)`)
	matches := requiresRegex.FindStringSubmatch(directive)

	if len(matches) < 2 {
		return nil, fmt.Errorf("invalid @requires directive format: %s", directive)
	}

	requiresDirective := &federationtypes.RequiresDirective{
		Fields: strings.TrimSpace(matches[1]),
	}

	// 验证字段选择集格式
	if err := p.validateFieldSelection(requiresDirective.Fields); err != nil {
		return nil, fmt.Errorf("invalid requires fields: %w", err)
	}

	return requiresDirective, nil
}

// ParseProvidesDirective 解析 @provides 指令
func (p *DirectiveParser) ParseProvidesDirective(directive string) (*federationtypes.ProvidesDirective, error) {
	if strings.TrimSpace(directive) == "" {
		return nil, errors.NewParsingError("directive cannot be empty")
	}

	// 正则表达式匹配 @provides(fields: "field1 field2")
	providesRegex := regexp.MustCompile(`@provides\s*\(\s*fields\s*:\s*"([^"]+)"\s*\)`)
	matches := providesRegex.FindStringSubmatch(directive)

	if len(matches) < 2 {
		return nil, fmt.Errorf("invalid @provides directive format: %s", directive)
	}

	providesDirective := &federationtypes.ProvidesDirective{
		Fields: strings.TrimSpace(matches[1]),
	}

	// 验证字段选择集格式
	if err := p.validateFieldSelection(providesDirective.Fields); err != nil {
		return nil, fmt.Errorf("invalid provides fields: %w", err)
	}

	return providesDirective, nil
}

// ValidateDirectives 验证指令的有效性
func (p *DirectiveParser) ValidateDirectives(directives *federationtypes.EntityDirectives) error {
	if directives == nil {
		return errors.NewValidationError("directives cannot be nil")
	}

	// 验证 @key 指令
	if len(directives.Keys) == 0 {
		// 如果有其他指令但没有 @key，这可能是有效的（例如，扩展类型）
		p.logger.Debug("No @key directives found")
	}

	for i, key := range directives.Keys {
		if key.Fields == "" {
			return fmt.Errorf("@key directive %d has empty fields", i)
		}
		if err := p.validateFieldSelection(key.Fields); err != nil {
			return fmt.Errorf("@key directive %d has invalid fields: %w", i, err)
		}
	}

	// 验证 @requires 和 @external 的组合
	if directives.Requires != nil && directives.External == nil {
		return errors.NewValidationError("@requires directive requires the field to be marked as @external")
	}

	// 验证 @provides 指令
	if directives.Provides != nil {
		if err := p.validateFieldSelection(directives.Provides.Fields); err != nil {
			return fmt.Errorf("@provides directive has invalid fields: %w", err)
		}
	}

	return nil
}

// 私有辅助方法

// parseKeyDirectives 解析所有 @key 指令
func (p *DirectiveParser) parseKeyDirectives(typeDef string) ([]federationtypes.KeyDirective, error) {
	var keys []federationtypes.KeyDirective

	// 查找所有 @key 指令
	keyRegex := regexp.MustCompile(`@key\s*\([^)]+\)`)
	matches := keyRegex.FindAllString(typeDef, -1)

	for _, match := range matches {
		key, err := p.ParseKeyDirective(match)
		if err != nil {
			return nil, err
		}
		keys = append(keys, *key)
	}

	return keys, nil
}

// parseExternalDirective 解析 @external 指令
func (p *DirectiveParser) parseExternalDirective(typeDef string) (*federationtypes.ExternalDirective, error) {
	// 查找 @external 指令
	externalRegex := regexp.MustCompile(`@external(?:\s*\([^)]*\))?`)
	match := externalRegex.FindString(typeDef)

	if match == "" {
		return nil, nil // 没有找到 @external 指令
	}

	return p.ParseExternalDirective(match)
}

// parseRequiresDirective 解析 @requires 指令
func (p *DirectiveParser) parseRequiresDirective(typeDef string) (*federationtypes.RequiresDirective, error) {
	// 查找 @requires 指令
	requiresRegex := regexp.MustCompile(`@requires\s*\([^)]+\)`)
	match := requiresRegex.FindString(typeDef)

	if match == "" {
		return nil, nil // 没有找到 @requires 指令
	}

	return p.ParseRequiresDirective(match)
}

// parseProvidesDirective 解析 @provides 指令
func (p *DirectiveParser) parseProvidesDirective(typeDef string) (*federationtypes.ProvidesDirective, error) {
	// 查找 @provides 指令
	providesRegex := regexp.MustCompile(`@provides\s*\([^)]+\)`)
	match := providesRegex.FindString(typeDef)

	if match == "" {
		return nil, nil // 没有找到 @provides 指令
	}

	return p.ParseProvidesDirective(match)
}

// validateFieldSelection 验证字段选择集格式
func (p *DirectiveParser) validateFieldSelection(fields string) error {
	if strings.TrimSpace(fields) == "" {
		return errors.NewValidationError("field selection cannot be empty")
	}

	// 简单验证：检查是否包含有效的字段名
	// 这里可以根据需要实现更复杂的 GraphQL 字段选择集验证
	fieldRegex := regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]*(\s+[a-zA-Z_][a-zA-Z0-9_]*)*$`)
	if !fieldRegex.MatchString(strings.TrimSpace(fields)) {
		return fmt.Errorf("invalid field selection format: %s", fields)
	}

	return nil
}

// truncateString 截断字符串用于日志记录
func (p *DirectiveParser) truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
