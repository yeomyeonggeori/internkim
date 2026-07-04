package llmbackend

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

type NativeSchemaLintResult struct {
	NormalizationsApplied []string
	RemainingViolations   []string
	ToolCount             int
	MaxNestingDepth       int
	TotalPropertyCount    int
	LargestEnumSize       int
}

var nativeToolSchemaAllowedKeywordByName = map[string]bool{
	"description": true,
	"enum":        true,
	"items":       true,
	"properties":  true,
	"required":    true,
	"type":        true,
}

func NormalizeNativeActionToolSchemas(tools []nativeActionTool) ([]nativeActionTool, NativeSchemaLintResult) {
	result := NativeSchemaLintResult{ToolCount: len(tools)}
	normalizedTools := make([]nativeActionTool, 0, len(tools))
	for _, tool := range tools {
		normalizedSchema, schemaResult := NormalizeNativeSchema(tool.Parameters)
		normalizedTool := tool
		normalizedTool.Parameters = normalizedSchema
		normalizedTools = append(normalizedTools, normalizedTool)
		result = mergeNativeSchemaLintResult(result, prefixNativeSchemaLintResult(tool.FunctionName, schemaResult))
	}
	result.ToolCount = len(tools)
	return normalizedTools, result
}

func NormalizeNativeSchema(schema json.RawMessage) (json.RawMessage, NativeSchemaLintResult) {
	var value any
	if errorValue := json.Unmarshal(schema, &value); errorValue != nil {
		return schema, NativeSchemaLintResult{
			RemainingViolations: []string{"schema is invalid JSON: " + errorValue.Error()},
		}
	}
	state := nativeSchemaLintState{}
	normalizedValue, _ := state.normalizeValue(value, "$", 1)
	content, errorValue := json.Marshal(normalizedValue)
	if errorValue != nil {
		state.result.RemainingViolations = append(state.result.RemainingViolations, "normalized schema is invalid JSON: "+errorValue.Error())
		return schema, state.result
	}
	return content, state.result
}

func NativeSchemaLintDiagnostics(result NativeSchemaLintResult) string {
	fields := []string{
		fmt.Sprintf("toolCount=%d", result.ToolCount),
		fmt.Sprintf("maxDepth=%d", result.MaxNestingDepth),
		fmt.Sprintf("totalProperties=%d", result.TotalPropertyCount),
		fmt.Sprintf("largestEnum=%d", result.LargestEnumSize),
		"lintFindings=" + nativeSchemaLintFindings(result),
	}
	return strings.Join(fields, " ")
}

func nativeSchemaLintFindings(result NativeSchemaLintResult) string {
	findings := append([]string{}, result.NormalizationsApplied...)
	findings = append(findings, result.RemainingViolations...)
	if len(findings) == 0 {
		return "none"
	}
	return strings.Join(sanitizeNativeSchemaLintFindings(findings), "|")
}

func sanitizeNativeSchemaLintFindings(findings []string) []string {
	result := make([]string, 0, len(findings))
	for _, finding := range findings {
		normalizedFinding := strings.TrimSpace(finding)
		normalizedFinding = strings.ReplaceAll(normalizedFinding, " ", "_")
		normalizedFinding = strings.ReplaceAll(normalizedFinding, "\n", "_")
		normalizedFinding = strings.ReplaceAll(normalizedFinding, "\t", "_")
		if normalizedFinding != "" {
			result = append(result, normalizedFinding)
		}
	}
	sort.Strings(result)
	return result
}

func mergeNativeSchemaLintResult(left NativeSchemaLintResult, right NativeSchemaLintResult) NativeSchemaLintResult {
	left.NormalizationsApplied = append(left.NormalizationsApplied, right.NormalizationsApplied...)
	left.RemainingViolations = append(left.RemainingViolations, right.RemainingViolations...)
	if right.ToolCount > left.ToolCount {
		left.ToolCount = right.ToolCount
	}
	if right.MaxNestingDepth > left.MaxNestingDepth {
		left.MaxNestingDepth = right.MaxNestingDepth
	}
	left.TotalPropertyCount += right.TotalPropertyCount
	if right.LargestEnumSize > left.LargestEnumSize {
		left.LargestEnumSize = right.LargestEnumSize
	}
	return left
}

func prefixNativeSchemaLintResult(prefix string, result NativeSchemaLintResult) NativeSchemaLintResult {
	if strings.TrimSpace(prefix) == "" {
		return result
	}
	result.NormalizationsApplied = prefixNativeSchemaLintFindings(prefix, result.NormalizationsApplied)
	result.RemainingViolations = prefixNativeSchemaLintFindings(prefix, result.RemainingViolations)
	return result
}

func prefixNativeSchemaLintFindings(prefix string, findings []string) []string {
	result := make([]string, 0, len(findings))
	for _, finding := range findings {
		result = append(result, prefix+": "+finding)
	}
	return result
}

type nativeSchemaLintState struct {
	result NativeSchemaLintResult
}

func (state *nativeSchemaLintState) normalizeValue(value any, path string, depth int) (any, bool) {
	state.recordDepth(depth)
	document, isDocument := value.(map[string]any)
	if isDocument {
		return state.normalizeDocument(document, path, depth)
	}
	values, isValues := value.([]any)
	if isValues {
		return state.normalizeArray(values, path, depth), false
	}
	return value, false
}

func (state *nativeSchemaLintState) normalizeArray(values []any, path string, depth int) []any {
	result := make([]any, 0, len(values))
	for index, item := range values {
		normalizedItem, _ := state.normalizeValue(item, fmt.Sprintf("%s[%d]", path, index), depth+1)
		result = append(result, normalizedItem)
	}
	return result
}

func (state *nativeSchemaLintState) normalizeDocument(document map[string]any, path string, depth int) (map[string]any, bool) {
	result := map[string]any{}
	requiredFields := stringSliceFromNativeSchema(document["required"])
	nullableProperties := map[string]bool{}
	keys := sortedNativeSchemaKeys(document)
	hasNullableType := false
	for _, key := range keys {
		value := document[key]
		if !nativeToolSchemaAllowedKeywordByName[key] {
			state.result.NormalizationsApplied = append(state.result.NormalizationsApplied, path+": stripped unsupported keyword "+key)
			continue
		}
		if key == "type" {
			normalizedType, isNullableType := state.normalizeType(value, path)
			result[key] = normalizedType
			hasNullableType = isNullableType
			continue
		}
		if key == "properties" {
			result[key] = state.normalizeProperties(value, path, depth, requiredFields, nullableProperties)
			continue
		}
		if key == "items" {
			normalizedValue, _ := state.normalizeValue(value, path+".items", depth+1)
			result[key] = normalizedValue
			continue
		}
		if key == "enum" {
			result[key] = state.normalizeEnum(value, path, false)
			continue
		}
		result[key] = value
	}
	state.normalizeRequired(result, path, requiredFields, nullableProperties)
	if hasNullableType {
		state.normalizeNullableEnum(result, path)
	}
	state.removeNonStringEnum(result, path)
	state.validateDocument(result, path)
	return result, hasNullableType
}

func (state *nativeSchemaLintState) removeNonStringEnum(document map[string]any, path string) {
	typeName, isString := document["type"].(string)
	if !isString || typeName == "string" {
		return
	}
	if _, hasEnum := document["enum"]; !hasEnum {
		return
	}
	delete(document, "enum")
	state.result.NormalizationsApplied = append(state.result.NormalizationsApplied, path+": removed enum from "+typeName+" type")
}

func (state *nativeSchemaLintState) normalizeProperties(value any, path string, depth int, requiredFields []string, nullableProperties map[string]bool) map[string]any {
	properties := mapFromNativeSchema(value)
	state.result.TotalPropertyCount += len(properties)
	result := map[string]any{}
	for _, propertyName := range sortedNativeSchemaKeys(properties) {
		normalizedValue, isNullable := state.normalizeValue(properties[propertyName], path+".properties."+propertyName, depth+1)
		result[propertyName] = normalizedValue
		if isNullable && stringSliceContains(requiredFields, propertyName) {
			nullableProperties[propertyName] = true
		}
	}
	return result
}

func (state *nativeSchemaLintState) normalizeRequired(document map[string]any, path string, requiredFields []string, nullableProperties map[string]bool) {
	if _, isFound := document["required"]; !isFound && len(requiredFields) == 0 {
		return
	}
	result := make([]string, 0, len(requiredFields))
	for _, fieldName := range requiredFields {
		if nullableProperties[fieldName] {
			state.result.NormalizationsApplied = append(state.result.NormalizationsApplied, path+": removed nullable field "+fieldName+" from required")
			continue
		}
		result = append(result, fieldName)
	}
	if len(result) > 0 || document["type"] == "object" {
		document["required"] = result
	}
}

func (state *nativeSchemaLintState) normalizeType(value any, path string) (any, bool) {
	if value == "integer" {
		state.result.NormalizationsApplied = append(state.result.NormalizationsApplied, path+": converted integer type to number")
		return "number", false
	}
	values, isValues := value.([]any)
	if !isValues {
		return value, false
	}
	types := []string{}
	hasNull := false
	for _, item := range values {
		typeName := stringFromNativeSchema(item)
		if item == nil || typeName == "null" {
			hasNull = true
			continue
		}
		if typeName == "integer" {
			typeName = "number"
			state.result.NormalizationsApplied = append(state.result.NormalizationsApplied, path+": converted integer type to number")
		}
		if strings.TrimSpace(typeName) != "" {
			types = append(types, typeName)
		}
	}
	types = sortedUniqueNativeSchemaStrings(types)
	if len(types) == 1 {
		if hasNull {
			state.result.NormalizationsApplied = append(state.result.NormalizationsApplied, path+": converted nullable type union to "+types[0])
		} else {
			state.result.NormalizationsApplied = append(state.result.NormalizationsApplied, path+": converted single-item type array to "+types[0])
		}
		return types[0], hasNull
	}
	state.result.RemainingViolations = append(state.result.RemainingViolations, path+": type array union is not provider portable")
	return value, hasNull
}

func (state *nativeSchemaLintState) normalizeEnum(value any, path string, removeNull bool) any {
	values, isValues := value.([]any)
	if !isValues {
		return value
	}
	result := make([]any, 0, len(values))
	for _, item := range values {
		if removeNull && item == nil {
			state.result.NormalizationsApplied = append(state.result.NormalizationsApplied, path+": removed null enum value")
			continue
		}
		result = append(result, item)
	}
	if len(result) > state.result.LargestEnumSize {
		state.result.LargestEnumSize = len(result)
	}
	return result
}

func (state *nativeSchemaLintState) normalizeNullableEnum(document map[string]any, path string) {
	enumValue, isFound := document["enum"]
	if !isFound {
		return
	}
	document["enum"] = state.normalizeEnum(enumValue, path, true)
}

func (state *nativeSchemaLintState) validateDocument(document map[string]any, path string) {
	if document["type"] == "integer" {
		state.result.RemainingViolations = append(state.result.RemainingViolations, path+": integer type remains after normalization")
	}
	if document["type"] == "object" {
		properties, isProperties := document["properties"].(map[string]any)
		if !isProperties {
			state.result.RemainingViolations = append(state.result.RemainingViolations, path+": object schema does not define properties")
			return
		}
		state.validateRequiredFields(document, properties, path)
	}
	if document["type"] == "array" {
		if _, isFound := document["items"]; !isFound {
			state.result.RemainingViolations = append(state.result.RemainingViolations, path+": array schema does not define items")
		}
	}
}

func (state *nativeSchemaLintState) validateRequiredFields(document map[string]any, properties map[string]any, path string) {
	required, isRequired := document["required"].([]string)
	if !isRequired {
		requiredValues, isRequiredValues := document["required"].([]any)
		if !isRequiredValues {
			return
		}
		for _, fieldName := range requiredValues {
			fieldNameString, isString := fieldName.(string)
			if !isString {
				state.result.RemainingViolations = append(state.result.RemainingViolations, path+": required field is not a string")
				continue
			}
			if _, isFound := properties[fieldNameString]; !isFound {
				state.result.RemainingViolations = append(state.result.RemainingViolations, path+": required field "+fieldNameString+" is not defined in properties")
			}
		}
		return
	}
	for _, fieldName := range required {
		if _, isFound := properties[fieldName]; !isFound {
			state.result.RemainingViolations = append(state.result.RemainingViolations, path+": required field "+fieldName+" is not defined in properties")
		}
	}
}

func (state *nativeSchemaLintState) recordDepth(depth int) {
	if depth > state.result.MaxNestingDepth {
		state.result.MaxNestingDepth = depth
	}
}

func sortedNativeSchemaKeys(document map[string]any) []string {
	keys := make([]string, 0, len(document))
	for key := range document {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func sortedUniqueNativeSchemaStrings(values []string) []string {
	seen := map[string]bool{}
	result := []string{}
	for _, value := range values {
		if seen[value] {
			continue
		}
		seen[value] = true
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}
