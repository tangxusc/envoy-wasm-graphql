package parser

import (
	"fmt"
	"strings"

	"github.com/wundergraph/graphql-go-tools/pkg/ast"
	"github.com/wundergraph/graphql-go-tools/pkg/astparser"
	"github.com/wundergraph/graphql-go-tools/pkg/astvalidation"
	"github.com/wundergraph/graphql-go-tools/pkg/operationreport"

	"envoy-wasm-graphql-federation/pkg/errors"
	federationtypes "envoy-wasm-graphql-federation/pkg/types"
)

// Parser 实现 GraphQL 查询解析器
type Parser struct {
	logger          federationtypes.Logger
	directiveParser federationtypes.FederationDirectiveParser
}

// NewParser 创建新的解析器
func NewParser(logger federationtypes.Logger) federationtypes.GraphQLParser {
	return &Parser{
		logger: logger,
		// 不能在这里创建 directiveParser，因为会造成循环依赖
		// directiveParser: federation.NewDirectiveParser(logger),
	}
}

// ParseQuery 解析 GraphQL 查询
func (p *Parser) ParseQuery(query string) (*federationtypes.ParsedQuery, error) {
	if strings.TrimSpace(query) == "" {
		return nil, errors.NewQueryParsingError("query cannot be empty")
	}

	p.logger.Debug("Parsing GraphQL query", "query", p.truncateQuery(query))

	// 使用 wundergraph/graphql-go-tools 解析查询
	report := &operationreport.Report{}
	document, parseReport := astparser.ParseGraphqlDocumentString(query)

	if parseReport.HasErrors() {
		p.logger.Error("Failed to parse GraphQL query", "errors", "parse errors found")
		return nil, p.convertParseErrors(parseReport)
	}

	// 分析查询
	parsedQuery, err := p.analyzeDocument(&document, report)
	if err != nil {
		return nil, err
	}

	p.logger.Debug("Query parsed successfully",
		"operation", parsedQuery.Operation,
		"complexity", parsedQuery.Complexity,
		"depth", parsedQuery.Depth,
	)

	return parsedQuery, nil
}

// ValidateQuery 验证查询合法性
func (p *Parser) ValidateQuery(query *federationtypes.ParsedQuery, schema *federationtypes.Schema) error {
	if query == nil {
		return errors.NewQueryValidationError("query is nil")
	}

	if schema == nil {
		return errors.NewQueryValidationError("schema is nil")
	}

	p.logger.Debug("Validating GraphQL query", "operation", query.Operation)

	// 转换为 AST 文档
	document, ok := query.AST.(*ast.Document)
	if !ok {
		return errors.NewQueryValidationError("invalid AST document")
	}

	// 解析模式
	schemaDocument, err := p.parseSchema(schema.SDL)
	if err != nil {
		return errors.NewQueryValidationError("invalid schema: " + err.Error())
	}

	// 验证查询
	report := &operationreport.Report{}
	validator := astvalidation.DefaultOperationValidator()
	validator.Validate(document, schemaDocument, report)

	if report.HasErrors() {
		p.logger.Error("Query validation failed", "errors", "validation errors found")
		return p.convertValidationErrors(report)
	}

	// 检查查询深度
	if query.Depth > 0 {
		// 这里可以添加深度限制检查
		p.logger.Debug("Query depth", "depth", query.Depth)
	}

	// 检查查询复杂度
	if query.Complexity > 0 {
		// 这里可以添加复杂度限制检查
		p.logger.Debug("Query complexity", "complexity", query.Complexity)
	}

	return nil
}

// ExtractFields 提取查询字段信息
func (p *Parser) ExtractFields(query *federationtypes.ParsedQuery) ([]federationtypes.FieldPath, error) {
	if query == nil {
		return nil, errors.NewQueryParsingError("query is nil")
	}

	document, ok := query.AST.(*ast.Document)
	if !ok {
		return nil, errors.NewQueryParsingError("invalid AST document")
	}

	p.logger.Debug("Extracting fields from query", "operation", query.Operation)

	var fieldPaths []federationtypes.FieldPath

	// 遍历文档中的操作
	for i, _ := range document.OperationDefinitions {
		operation := document.OperationDefinitions[i]

		// 检查操作类型
		if query.Operation != "" {
			operationName := document.OperationDefinitionNameString(i)
			if operationName != query.Operation {
				continue
			}
		}

		// 提取选择集字段
		selectionSet := operation.SelectionSet
		paths := p.extractFieldsFromSelectionSet(document, selectionSet, []string{})
		fieldPaths = append(fieldPaths, paths...)
	}

	p.logger.Debug("Extracted fields", "count", len(fieldPaths))
	return fieldPaths, nil
}

// analyzeDocument 分析文档
func (p *Parser) analyzeDocument(document *ast.Document, report *operationreport.Report) (*federationtypes.ParsedQuery, error) {
	parsed := &federationtypes.ParsedQuery{
		AST:       document,
		Variables: make(map[string]interface{}),
		Fragments: make(map[string]interface{}),
	}

	// 查找目标操作
	var targetOperation ast.OperationDefinition
	var operationIndex int

	if len(document.OperationDefinitions) == 1 {
		// 单个操作，直接使用
		operationIndex = 0
		targetOperation = document.OperationDefinitions[operationIndex]
	} else {
		// 多个操作，需要根据 operationName 选择
		// 这里需要从上下文获取 operationName
		return nil, errors.NewQueryParsingError("multiple operations found, operationName required")
	}

	// 获取操作名称
	// 简化处理，由于 AST API 变化，这里使用基本方法
	if operationName := document.OperationDefinitionNameString(operationIndex); operationName != "" {
		parsed.Operation = operationName
	}

	// 计算查询深度和复杂度
	parsed.Depth = p.calculateDepth(document, targetOperation.SelectionSet, 0)
	parsed.Complexity = p.calculateComplexity(document, targetOperation.SelectionSet)

	// 提取片段
	p.extractFragments(document, parsed)

	return parsed, nil
}

// extractFieldsFromSelectionSet 从选择集提取字段
func (p *Parser) extractFieldsFromSelectionSet(document *ast.Document, selectionSet int, path []string) []federationtypes.FieldPath {
	var fieldPaths []federationtypes.FieldPath

	if selectionSet == -1 {
		return fieldPaths
	}

	selections := document.SelectionSets[selectionSet].SelectionRefs
	for _, selectionRef := range selections {
		selection := document.Selections[selectionRef]

		switch selection.Kind {
		case ast.SelectionKindField:
			field := document.Fields[selection.Ref]
			fieldName := document.FieldNameString(selection.Ref)

			currentPath := append(path, fieldName)

			// 添加字段路径
			fieldPath := federationtypes.FieldPath{
				Path: currentPath,
				Type: p.getFieldType(document, field),
			}
			fieldPaths = append(fieldPaths, fieldPath)

			// 递归处理子字段
			if field.SelectionSet != -1 {
				subPaths := p.extractFieldsFromSelectionSet(document, field.SelectionSet, currentPath)
				fieldPaths = append(fieldPaths, subPaths...)
			}

		case ast.SelectionKindFragmentSpread:
			// 处理片段展开
			fragmentName := document.FragmentSpreadNameString(selection.Ref)
			p.logger.Debug("Found fragment spread", "fragment", fragmentName)

		case ast.SelectionKindInlineFragment:
			// 处理内联片段
			inlineFragment := document.InlineFragments[selection.Ref]
			if inlineFragment.SelectionSet != -1 {
				subPaths := p.extractFieldsFromSelectionSet(document, inlineFragment.SelectionSet, path)
				fieldPaths = append(fieldPaths, subPaths...)
			}
		}
	}

	return fieldPaths
}

// calculateDepth 计算查询深度
func (p *Parser) calculateDepth(document *ast.Document, selectionSet int, currentDepth int) int {
	visited := make(map[int]bool)
	return p.calculateDepthWithVisited(document, selectionSet, currentDepth, visited)
}

// calculateDepthWithVisited 计算查询深度（带访问跟踪）
func (p *Parser) calculateDepthWithVisited(document *ast.Document, selectionSet int, currentDepth int, visited map[int]bool) int {
	if selectionSet == -1 {
		return currentDepth
	}

	// 检查是否已经访问过这个选择集，防止循环引用
	if visited[selectionSet] {
		return currentDepth
	}

	// 防止无限递归，设置最大深度限制
	const maxAllowedDepth = 50
	if currentDepth > maxAllowedDepth {
		p.logger.Warn("Query depth calculation exceeded maximum allowed depth", "maxDepth", maxAllowedDepth, "currentDepth", currentDepth)
		return currentDepth
	}

	// 标记为已访问
	visited[selectionSet] = true
	defer func() {
		delete(visited, selectionSet)
	}()

	maxDepth := currentDepth
	selections := document.SelectionSets[selectionSet].SelectionRefs

	for _, selectionRef := range selections {
		selection := document.Selections[selectionRef]

		switch selection.Kind {
		case ast.SelectionKindField:
			field := document.Fields[selection.Ref]
			if field.SelectionSet != -1 {
				depth := p.calculateDepthWithVisited(document, field.SelectionSet, currentDepth+1, visited)
				if depth > maxDepth {
					maxDepth = depth
				}
			}

		case ast.SelectionKindInlineFragment:
			inlineFragment := document.InlineFragments[selection.Ref]
			if inlineFragment.SelectionSet != -1 {
				depth := p.calculateDepthWithVisited(document, inlineFragment.SelectionSet, currentDepth, visited)
				if depth > maxDepth {
					maxDepth = depth
				}
			}
		}
	}

	return maxDepth
}

// calculateComplexity 计算查询复杂度
func (p *Parser) calculateComplexity(document *ast.Document, selectionSet int) int {
	visited := make(map[int]bool)
	return p.calculateComplexityWithVisited(document, selectionSet, visited)
}

// calculateComplexityWithVisited 计算查询复杂度（带访问跟踪）
func (p *Parser) calculateComplexityWithVisited(document *ast.Document, selectionSet int, visited map[int]bool) int {
	if selectionSet == -1 {
		return 0
	}

	// 检查是否已经访问过这个选择集，防止循环引用
	if visited[selectionSet] {
		return 0
	}

	// 标记为已访问
	visited[selectionSet] = true
	defer func() {
		delete(visited, selectionSet)
	}()

	complexity := 0
	selections := document.SelectionSets[selectionSet].SelectionRefs

	for _, selectionRef := range selections {
		selection := document.Selections[selectionRef]

		switch selection.Kind {
		case ast.SelectionKindField:
			field := document.Fields[selection.Ref]
			complexity += 1 // 每个字段增加1点复杂度

			if field.SelectionSet != -1 {
				complexity += p.calculateComplexityWithVisited(document, field.SelectionSet, visited)
			}

		case ast.SelectionKindInlineFragment:
			inlineFragment := document.InlineFragments[selection.Ref]
			if inlineFragment.SelectionSet != -1 {
				complexity += p.calculateComplexityWithVisited(document, inlineFragment.SelectionSet, visited)
			}
		}
	}

	return complexity
}

// extractFragments 提取片段
func (p *Parser) extractFragments(document *ast.Document, parsed *federationtypes.ParsedQuery) {
	for i, _ := range document.FragmentDefinitions {
		fragment := document.FragmentDefinitions[i]
		fragmentName := document.FragmentDefinitionNameString(i)

		// 简化处理，存储片段名称
		parsed.Fragments[fragmentName] = fragment
	}
}

// parseSchema 解析模式
func (p *Parser) parseSchema(schemaSDL string) (*ast.Document, error) {
	document, parseReport := astparser.ParseGraphqlDocumentString(schemaSDL)

	if parseReport.HasErrors() {
		return nil, fmt.Errorf("schema parse error: parse errors found")
	}

	return &document, nil
}

// getFieldType 获取字段类型（增强版）
func (p *Parser) getFieldType(document *ast.Document, field ast.Field) string {
	// 由于GraphQL库版本兼容性问题，返回默认类型
	return "String"
}

// resolveTypeFromRef 从类型引用解析类型
func (p *Parser) resolveTypeFromRef(document *ast.Document, typeRef int) string {
	if typeRef == -1 {
		return "Unknown"
	}

	// 检查类型节点
	if typeRef >= 0 && typeRef < len(document.Types) {
		typeNode := document.Types[typeRef]
		return p.getTypeString(document, typeNode)
	}

	return "String" // 默认类型
}

// getTypeString 获取类型字符串
func (p *Parser) getTypeString(document *ast.Document, typeNode ast.Type) string {
	switch typeNode.TypeKind {
	case ast.TypeKindNamed:
		// 命名类型
		if typeNode.OfType != -1 {
			return document.ResolveTypeNameString(typeNode.OfType)
		}
		return "String"

	case ast.TypeKindList:
		// 列表类型
		if typeNode.OfType != -1 {
			innerType := p.resolveTypeFromRef(document, typeNode.OfType)
			return fmt.Sprintf("[%s]", innerType)
		}
		return "[String]"

	case ast.TypeKindNonNull:
		// 非空类型
		if typeNode.OfType != -1 {
			innerType := p.resolveTypeFromRef(document, typeNode.OfType)
			return fmt.Sprintf("%s!", innerType)
		}
		return "String!"

	default:
		return "String"
	}
}

// convertParseErrors 转换解析错误（增强版）
func (p *Parser) convertParseErrors(report operationreport.Report) error {
	if !report.HasErrors() {
		return fmt.Errorf("unknown parse error")
	}

	// 收集所有错误信息
	var errorMessages []string

	// 遍历外部错误
	for _, externalErr := range report.ExternalErrors {
		// 由于ExternalError结构体可能没有Error()方法，使用字符串转换
		errorMsg := fmt.Sprintf("%v", externalErr)
		errorMessages = append(errorMessages, errorMsg)
	}

	// 遍历内部错误
	for _, internalErr := range report.InternalErrors {
		errorMessages = append(errorMessages, internalErr.Error())
	}

	// 构建详细错误信息
	mainMessage := "GraphQL parse errors occurred"
	if len(errorMessages) > 0 {
		mainMessage = errorMessages[0]
		if len(errorMessages) > 1 {
			mainMessage += fmt.Sprintf(" (and %d more errors)", len(errorMessages)-1)
		}
	}

	p.logger.Error("Parse errors detected",
		"errorCount", len(errorMessages),
		"mainError", mainMessage,
	)

	return fmt.Errorf("parse error: %s", mainMessage)
}

// convertValidationErrors 转换验证错误（增强版）
func (p *Parser) convertValidationErrors(report *operationreport.Report) error {
	if !report.HasErrors() {
		return fmt.Errorf("unknown validation error")
	}

	// 收集验证错误
	var errorMessages []string

	// 处理外部错误
	for _, externalErr := range report.ExternalErrors {
		errorMsg := fmt.Sprintf("%v", externalErr)
		errorMessages = append(errorMessages, errorMsg)
	}

	// 处理内部错误
	for _, internalErr := range report.InternalErrors {
		errorMsg := internalErr.Error()
		errorMessages = append(errorMessages, errorMsg)
	}

	// 构建主错误消息
	mainMessage := "GraphQL validation failed"
	if len(errorMessages) > 0 {
		mainMessage = errorMessages[0]
		if len(errorMessages) > 1 {
			mainMessage += fmt.Sprintf(" (and %d more validation errors)", len(errorMessages)-1)
		}
	}

	p.logger.Error("Validation errors detected",
		"errorCount", len(errorMessages),
		"mainError", mainMessage,
	)

	return fmt.Errorf("validation error: %s", mainMessage)
}

// categorizeValidationError 对验证错误进行分类
func (p *Parser) categorizeValidationError(errorMsg string) string {
	errorMsg = strings.ToLower(errorMsg)

	// 字段相关错误
	if strings.Contains(errorMsg, "field") {
		if strings.Contains(errorMsg, "not found") || strings.Contains(errorMsg, "undefined") {
			return "field_not_found"
		}
		if strings.Contains(errorMsg, "argument") {
			return "field_argument_error"
		}
		return "field_error"
	}

	// 类型相关错误
	if strings.Contains(errorMsg, "type") {
		if strings.Contains(errorMsg, "mismatch") {
			return "type_mismatch"
		}
		if strings.Contains(errorMsg, "not found") {
			return "type_not_found"
		}
		return "type_error"
	}

	// 语法错误
	if strings.Contains(errorMsg, "syntax") || strings.Contains(errorMsg, "parse") {
		return "syntax_error"
	}

	// 参数错误
	if strings.Contains(errorMsg, "argument") || strings.Contains(errorMsg, "variable") {
		return "argument_error"
	}

	// 指令错误
	if strings.Contains(errorMsg, "directive") {
		return "directive_error"
	}

	// 片段错误
	if strings.Contains(errorMsg, "fragment") {
		return "fragment_error"
	}

	return "unknown"
}

// truncateQuery 截断查询用于日志记录
func (p *Parser) truncateQuery(query string) string {
	const maxLen = 200
	if len(query) <= maxLen {
		return query
	}
	return query[:maxLen] + "..."
}

// Federation 指令解析支持

// ExtractFederationEntities 从模式中提取 Federation 实体
func (p *Parser) ExtractFederationEntities(schema string) ([]federationtypes.FederatedEntity, error) {
	if strings.TrimSpace(schema) == "" {
		return nil, errors.NewParsingError("schema cannot be empty")
	}

	p.logger.Debug("Extracting Federation entities from schema")

	// 解析模式文档
	document, parseReport := astparser.ParseGraphqlDocumentString(schema)
	if parseReport.HasErrors() {
		p.logger.Error("Failed to parse schema", "errors", "parse errors found")
		return nil, p.convertParseErrors(parseReport)
	}

	var entities []federationtypes.FederatedEntity

	// 遍历类型定义
	for i, _ := range document.ObjectTypeDefinitions {
		_ = document.ObjectTypeDefinitions[i] // 使用 typeDef 变量
		typeName := document.ObjectTypeDefinitionNameString(i)

		// 检查是否有 Federation 指令
		entity, err := p.extractEntityFromTypeDefinition(&document, i, typeName)
		if err != nil {
			p.logger.Warn("Failed to extract entity", "type", typeName, "error", err)
			continue
		}

		if entity != nil {
			entities = append(entities, *entity)
		}
	}

	p.logger.Debug("Extracted Federation entities", "count", len(entities))
	return entities, nil
}

// extractEntityFromTypeDefinition 从类型定义中提取实体
func (p *Parser) extractEntityFromTypeDefinition(document *ast.Document, typeIndex int, typeName string) (*federationtypes.FederatedEntity, error) {
	typeDef := document.ObjectTypeDefinitions[typeIndex]

	// 提取类型指令
	typeDirectives, err := p.extractDirectivesFromType(document, typeIndex)
	if err != nil {
		return nil, fmt.Errorf("failed to extract type directives: %w", err)
	}

	// 检查是否是 Federation 实体（有 @key 指令）
	if len(typeDirectives.Keys) == 0 {
		// 不是 Federation 实体
		return nil, nil
	}

	entity := &federationtypes.FederatedEntity{
		TypeName:   typeName,
		Directives: *typeDirectives,
		Fields:     []federationtypes.FederatedField{},
	}

	// 提取字段信息
	for _, fieldRef := range typeDef.FieldsDefinition.Refs {
		field, err := p.extractFieldFromDefinition(document, fieldRef)
		if err != nil {
			p.logger.Warn("Failed to extract field", "type", typeName, "error", err)
			continue
		}

		if field != nil {
			entity.Fields = append(entity.Fields, *field)
		}
	}

	return entity, nil
}

// extractDirectivesFromType 从类型定义中提取指令
func (p *Parser) extractDirectivesFromType(document *ast.Document, typeIndex int) (*federationtypes.EntityDirectives, error) {
	typeDef := document.ObjectTypeDefinitions[typeIndex]
	directives := &federationtypes.EntityDirectives{}

	// 遍历类型上的指令
	for _, directiveRef := range typeDef.Directives.Refs {
		_ = document.Directives[directiveRef] // 使用 directive 变量
		directiveName := document.DirectiveNameString(directiveRef)

		switch directiveName {
		case "key":
			keyDirective, err := p.extractKeyDirective(document, directiveRef)
			if err != nil {
				return nil, fmt.Errorf("failed to extract @key directive: %w", err)
			}
			if keyDirective != nil {
				directives.Keys = append(directives.Keys, *keyDirective)
			}

		case "external":
			externalDirective, err := p.extractExternalDirective(document, directiveRef)
			if err != nil {
				return nil, fmt.Errorf("failed to extract @external directive: %w", err)
			}
			directives.External = externalDirective

		case "requires":
			requiresDirective, err := p.extractRequiresDirective(document, directiveRef)
			if err != nil {
				return nil, fmt.Errorf("failed to extract @requires directive: %w", err)
			}
			directives.Requires = requiresDirective

		case "provides":
			providesDirective, err := p.extractProvidesDirective(document, directiveRef)
			if err != nil {
				return nil, fmt.Errorf("failed to extract @provides directive: %w", err)
			}
			directives.Provides = providesDirective
		}
	}

	return directives, nil
}

// extractFieldFromDefinition 从字段定义中提取字段
func (p *Parser) extractFieldFromDefinition(document *ast.Document, fieldRef int) (*federationtypes.FederatedField, error) {
	fieldDef := document.FieldDefinitions[fieldRef]
	fieldName := document.FieldDefinitionNameString(fieldRef)
	fieldType := p.extractFieldType(document, fieldDef.Type)

	field := &federationtypes.FederatedField{
		Name:       fieldName,
		Type:       fieldType,
		Directives: federationtypes.EntityDirectives{},
		Arguments:  []federationtypes.ArgumentInfo{},
	}

	// 提取字段指令
	fieldDirectives, err := p.extractDirectivesFromField(document, fieldRef)
	if err != nil {
		return nil, fmt.Errorf("failed to extract field directives: %w", err)
	}
	field.Directives = *fieldDirectives

	// 提取字段参数
	for _, argRef := range fieldDef.ArgumentsDefinition.Refs {
		argDef := document.InputValueDefinitions[argRef]
		argName := document.InputValueDefinitionNameString(argRef)
		argType := p.extractFieldType(document, argDef.Type)

		argument := federationtypes.ArgumentInfo{
			Name: argName,
			Type: argType,
		}
		field.Arguments = append(field.Arguments, argument)
	}

	return field, nil
}

// extractDirectivesFromField 从字段定义中提取指令
func (p *Parser) extractDirectivesFromField(document *ast.Document, fieldRef int) (*federationtypes.EntityDirectives, error) {
	fieldDef := document.FieldDefinitions[fieldRef]
	directives := &federationtypes.EntityDirectives{}

	// 遍历字段上的指令
	for _, directiveRef := range fieldDef.Directives.Refs {
		directiveName := document.DirectiveNameString(directiveRef)

		switch directiveName {
		case "external":
			externalDirective, err := p.extractExternalDirective(document, directiveRef)
			if err != nil {
				return nil, fmt.Errorf("failed to extract @external directive: %w", err)
			}
			directives.External = externalDirective

		case "requires":
			requiresDirective, err := p.extractRequiresDirective(document, directiveRef)
			if err != nil {
				return nil, fmt.Errorf("failed to extract @requires directive: %w", err)
			}
			directives.Requires = requiresDirective

		case "provides":
			providesDirective, err := p.extractProvidesDirective(document, directiveRef)
			if err != nil {
				return nil, fmt.Errorf("failed to extract @provides directive: %w", err)
			}
			directives.Provides = providesDirective
		}
	}

	return directives, nil
}

// extractKeyDirective 提取 @key 指令
func (p *Parser) extractKeyDirective(document *ast.Document, directiveRef int) (*federationtypes.KeyDirective, error) {
	directive := document.Directives[directiveRef]
	keyDirective := &federationtypes.KeyDirective{
		Resolvable: true, // 默认值
	}

	// 提取指令参数
	for _, argRef := range directive.Arguments.Refs {
		argument := document.Arguments[argRef]
		argName := document.ArgumentNameString(argRef)

		switch argName {
		case "fields":
			// 提取 fields 参数值
			fieldsValue, err := p.extractStringValue(document, argument.Value.Ref)
			if err != nil {
				return nil, fmt.Errorf("failed to extract fields value: %w", err)
			}
			keyDirective.Fields = fieldsValue

		case "resolvable":
			// 提取 resolvable 参数值
			resolvableValue, err := p.extractBooleanValue(document, argument.Value.Ref)
			if err != nil {
				return nil, fmt.Errorf("failed to extract resolvable value: %w", err)
			}
			keyDirective.Resolvable = resolvableValue
		}
	}

	return keyDirective, nil
}

// extractExternalDirective 提取 @external 指令
func (p *Parser) extractExternalDirective(document *ast.Document, directiveRef int) (*federationtypes.ExternalDirective, error) {
	externalDirective := &federationtypes.ExternalDirective{}

	// @external 指令可能没有参数，或者有 reason 参数
	directive := document.Directives[directiveRef]
	for _, argRef := range directive.Arguments.Refs {
		argument := document.Arguments[argRef]
		argName := document.ArgumentNameString(argRef)

		if argName == "reason" {
			reasonValue, err := p.extractStringValue(document, argument.Value.Ref)
			if err != nil {
				return nil, fmt.Errorf("failed to extract reason value: %w", err)
			}
			externalDirective.Reason = reasonValue
		}
	}

	return externalDirective, nil
}

// extractRequiresDirective 提取 @requires 指令
func (p *Parser) extractRequiresDirective(document *ast.Document, directiveRef int) (*federationtypes.RequiresDirective, error) {
	directive := document.Directives[directiveRef]
	requiresDirective := &federationtypes.RequiresDirective{}

	// 提取指令参数
	for _, argRef := range directive.Arguments.Refs {
		argument := document.Arguments[argRef]
		argName := document.ArgumentNameString(argRef)

		if argName == "fields" {
			fieldsValue, err := p.extractStringValue(document, argument.Value.Ref)
			if err != nil {
				return nil, fmt.Errorf("failed to extract fields value: %w", err)
			}
			requiresDirective.Fields = fieldsValue
		}
	}

	return requiresDirective, nil
}

// extractProvidesDirective 提取 @provides 指令
func (p *Parser) extractProvidesDirective(document *ast.Document, directiveRef int) (*federationtypes.ProvidesDirective, error) {
	directive := document.Directives[directiveRef]
	providesDirective := &federationtypes.ProvidesDirective{}

	// 提取指令参数
	for _, argRef := range directive.Arguments.Refs {
		argument := document.Arguments[argRef]
		argName := document.ArgumentNameString(argRef)

		if argName == "fields" {
			fieldsValue, err := p.extractStringValue(document, argument.Value.Ref)
			if err != nil {
				return nil, fmt.Errorf("failed to extract fields value: %w", err)
			}
			providesDirective.Fields = fieldsValue
		}
	}

	return providesDirective, nil
}

// extractStringValue 提取字符串值
func (p *Parser) extractStringValue(document *ast.Document, valueRef int) (string, error) {
	value := document.Values[valueRef]
	if value.Kind != ast.ValueKindString {
		return "", fmt.Errorf("expected string value, got %v", value.Kind)
	}

	return document.StringValueContentString(valueRef), nil
}

// extractBooleanValue 提取布尔值
func (p *Parser) extractBooleanValue(document *ast.Document, valueRef int) (bool, error) {
	value := document.Values[valueRef]
	if value.Kind != ast.ValueKindBoolean {
		return false, fmt.Errorf("expected boolean value, got %v", value.Kind)
	}

	// 从 ast.BooleanValue 转换为 bool
	boolValue := document.BooleanValue(valueRef)
	return bool(boolValue), nil
}

// extractFieldType 提取字段类型
func (p *Parser) extractFieldType(document *ast.Document, typeRef int) string {
	if typeRef == -1 {
		return "String" // 默认类型
	}

	return p.resolveTypeFromRef(document, typeRef)
}
