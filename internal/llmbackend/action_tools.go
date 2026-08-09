package llmbackend

import (
	"encoding/json"
	"errors"
	"regexp"
	"sort"
	"strings"
)

var nativeFunctionNamePattern = regexp.MustCompile(`[^A-Za-z0-9_-]+`)

var nativeBlueclawPlanningFields = []nativeBlueclawPlanningField{
	{ActionFieldName: "message", ArgumentFieldName: "blueclawMessage"},
	{ActionFieldName: "reason", ArgumentFieldName: "blueclawReason"},
	{ActionFieldName: "goalStatus", ArgumentFieldName: "blueclawGoalStatus"},
	{ActionFieldName: "goalSatisfied", ArgumentFieldName: "blueclawGoalSatisfied"},
	{ActionFieldName: "remainingWork", ArgumentFieldName: "blueclawRemainingWork"},
	{ActionFieldName: "executionStateUpdate", ArgumentFieldName: "blueclawExecutionStateUpdate"},
	{ActionFieldName: "requestTools", ArgumentFieldName: "blueclawRequestTools"},
	{ActionFieldName: "requestSkills", ArgumentFieldName: "blueclawRequestSkills"},
}

type nativeBlueclawPlanningField struct {
	ActionFieldName   string
	ArgumentFieldName string
}

type nativeActionToolSet struct {
	Tools            []nativeActionTool
	ToolByName       map[string]nativeActionTool
	NativeSchemaLint NativeSchemaLintResult
}

type nativeActionTool struct {
	FunctionName string
	Description  string
	Action       string
	ToolName     string
	Parameters   json.RawMessage
}

type actionSchemaDocument struct {
	OneOf []actionSchemaVariant `json:"oneOf"`
}

type actionSchemaVariant struct {
	Description string                     `json:"description"`
	Properties  map[string]json.RawMessage `json:"properties"`
	Required    []string                   `json:"required"`
}

func nativeActionToolsForSchema(schema StructuredOutputSchema) (nativeActionToolSet, bool, error) {
	if strings.TrimSpace(schema.Name) != "bluecollar_agent_turn_action" {
		return nativeActionToolSet{}, false, nil
	}
	var document actionSchemaDocument
	if errorValue := json.Unmarshal(schema.Document, &document); errorValue != nil {
		return nativeActionToolSet{}, true, errorValue
	}
	toolSet := nativeActionToolSet{ToolByName: map[string]nativeActionTool{}}
	for _, variant := range document.OneOf {
		tool, isFound, errorValue := nativeActionToolForVariant(variant)
		if errorValue != nil {
			return nativeActionToolSet{}, true, errorValue
		}
		if !isFound {
			continue
		}
		if _, isDuplicate := toolSet.ToolByName[tool.FunctionName]; isDuplicate {
			return nativeActionToolSet{}, true, errors.New("agent action schema maps multiple actions to native function: " + tool.FunctionName)
		}
		toolSet.Tools = append(toolSet.Tools, tool)
		toolSet.ToolByName[tool.FunctionName] = tool
	}
	if len(toolSet.Tools) == 0 {
		return nativeActionToolSet{}, true, errors.New("agent action schema did not include callable variants")
	}
	return normalizeNativeActionToolSet(toolSet), true, nil
}

func normalizeNativeActionToolSet(toolSet nativeActionToolSet) nativeActionToolSet {
	normalizedTools, lintResult := NormalizeNativeActionToolSchemas(toolSet.Tools)
	return nativeActionToolSet{
		Tools:            normalizedTools,
		ToolByName:       nativeActionToolByName(normalizedTools),
		NativeSchemaLint: lintResult,
	}
}

func nativeActionToolByName(tools []nativeActionTool) map[string]nativeActionTool {
	result := map[string]nativeActionTool{}
	for _, tool := range tools {
		result[tool.FunctionName] = tool
	}
	return result
}

func nativeActionToolForVariant(variant actionSchemaVariant) (nativeActionTool, bool, error) {
	action, isFound := enumStringValue(variant.Properties["action"])
	if !isFound {
		return nativeActionTool{}, false, nil
	}
	if isNativeToolAction(action) {
		toolName, isFound := enumStringValue(variant.Properties["toolName"])
		if !isFound {
			return nativeActionTool{}, false, errors.New(action + " action schema is missing toolName enum")
		}
		parameters, errorValue := toolActionParameters(variant)
		if errorValue != nil {
			return nativeActionTool{}, false, errorValue
		}
		return nativeActionTool{
			FunctionName: nativeActionFunctionName(action, toolName),
			Description:  nativeToolDescription(variant.Description, toolName),
			Action:       action,
			ToolName:     toolName,
			Parameters:   parameters,
		}, true, nil
	}
	parameters, errorValue := controlActionParameters(variant)
	if errorValue != nil {
		return nativeActionTool{}, false, errorValue
	}
	return nativeActionTool{
		FunctionName: nativeControlFunctionName(action),
		Description:  nativeControlDescription(action),
		Action:       action,
		Parameters:   parameters,
	}, true, nil
}

func nativeActionJSON(toolSet nativeActionToolSet, functionName string, arguments string) (string, error) {
	tool, isFound := nativeActionToolForProviderName(toolSet, functionName)
	if !isFound {
		return "", errors.New("native tool call referenced unknown function: " + functionName)
	}
	argumentDocument := json.RawMessage(strings.TrimSpace(arguments))
	if len(argumentDocument) == 0 {
		argumentDocument = json.RawMessage(`{}`)
	}
	if isNativeToolAction(tool.Action) {
		payload, errorValue := nativeContinueActionPayload(tool, argumentDocument)
		if errorValue != nil {
			return "", errorValue
		}
		content, errorValue := json.Marshal(payload)
		return string(content), errorValue
	}
	var payload map[string]any
	if errorValue := json.Unmarshal(argumentDocument, &payload); errorValue != nil {
		return "", errorValue
	}
	payload["action"] = tool.Action
	content, errorValue := json.Marshal(payload)
	return string(content), errorValue
}

func nativeActionToolForProviderName(toolSet nativeActionToolSet, functionName string) (nativeActionTool, bool) {
	normalizedFunctionName := strings.TrimSpace(functionName)
	if tool, isFound := toolSet.ToolByName[normalizedFunctionName]; isFound {
		return tool, true
	}
	for _, tool := range toolSet.Tools {
		if nativeActionToolMatchesProviderName(tool, normalizedFunctionName) {
			return tool, true
		}
	}
	return nativeActionTool{}, false
}

func nativeActionToolMatchesProviderName(tool nativeActionTool, functionName string) bool {
	if functionName == "" {
		return false
	}
	if functionName == tool.FunctionName {
		return true
	}
	if !isNativeToolAction(tool.Action) {
		return functionName == tool.Action || functionName == nativeSafeFunctionName(tool.Action)
	}
	return functionName == tool.ToolName || functionName == nativeSafeFunctionName(tool.ToolName)
}

func isNativeToolAction(action string) bool {
	return action == "continue"
}

func nativeContinueActionPayload(tool nativeActionTool, argumentDocument json.RawMessage) (map[string]any, error) {
	var flatArguments map[string]json.RawMessage
	if errorValue := json.Unmarshal(argumentDocument, &flatArguments); errorValue != nil {
		return nil, errorValue
	}
	toolInput, planningFields := reconstructActionFromFlatArguments(flatArguments, nativeBlueclawPlanningArgumentNames())
	payload := map[string]any{}
	for fieldName, fieldValue := range planningFields {
		payload[fieldName] = fieldValue
	}
	payload["action"] = tool.Action
	payload["toolName"] = tool.ToolName
	payload["toolInput"] = toolInput
	return payload, nil
}

func flattenToolInputSchema(toolInputSchema json.RawMessage) (map[string]json.RawMessage, []string) {
	var document struct {
		Properties map[string]json.RawMessage `json:"properties"`
		Required   []string                   `json:"required"`
	}
	if errorValue := json.Unmarshal(toolInputSchema, &document); errorValue != nil {
		return map[string]json.RawMessage{}, nil
	}
	properties := map[string]json.RawMessage{}
	for propertyName, propertySchema := range document.Properties {
		properties[propertyName] = propertySchema
	}
	return properties, append([]string{}, document.Required...)
}

func reconstructActionFromFlatArguments(flatArguments map[string]json.RawMessage, knownBlueclawFields []string) (map[string]any, map[string]any) {
	blueclawFieldNames := stringSetForNativeSchema(knownBlueclawFields)
	toolInput := map[string]any{}
	planningFields := map[string]any{}
	for fieldName, fieldValue := range flatArguments {
		decodedValue := valueFromNativeArgument(fieldValue)
		if blueclawFieldNames[fieldName] {
			planningFields[actionFieldNameForBlueclawArgument(fieldName)] = decodedValue
			continue
		}
		toolInput[fieldName] = decodedValue
	}
	return toolInput, planningFields
}

func valueFromNativeArgument(fieldValue json.RawMessage) any {
	if len(fieldValue) == 0 {
		return nil
	}
	var decodedValue any
	if errorValue := json.Unmarshal(fieldValue, &decodedValue); errorValue != nil {
		return nil
	}
	return decodedValue
}

func actionFieldNameForBlueclawArgument(fieldName string) string {
	for _, planningField := range nativeBlueclawPlanningFields {
		if planningField.ArgumentFieldName == fieldName {
			return planningField.ActionFieldName
		}
	}
	return fieldName
}

func nativeBlueclawPlanningArgumentNames() []string {
	fieldNames := make([]string, 0, len(nativeBlueclawPlanningFields))
	for _, planningField := range nativeBlueclawPlanningFields {
		fieldNames = append(fieldNames, planningField.ArgumentFieldName)
	}
	return fieldNames
}

func stringSliceContains(values []string, expected string) bool {
	for _, value := range values {
		if strings.TrimSpace(value) == expected {
			return true
		}
	}
	return false
}

var nativeContinueKeptPlanningActionFieldNames = map[string]bool{
	"message":       true,
	"requestTools":  true,
	"requestSkills": true,
}

var nativeOmittedPlanningActionFieldNames = map[string]bool{
	"executionStateUpdate": true,
}

func toolActionParameters(variant actionSchemaVariant) (json.RawMessage, error) {
	properties, required := flattenToolInputSchema(variant.Properties["toolInput"])
	for _, planningField := range nativeBlueclawPlanningFields {
		if !nativeContinueKeptPlanningActionFieldNames[planningField.ActionFieldName] {
			continue
		}
		if propertySchema, isFound := variant.Properties[planningField.ActionFieldName]; isFound {
			properties[planningField.ArgumentFieldName] = propertySchema
		}
	}
	return nativeStrictObjectParameters(properties, required)
}

func controlActionParameters(variant actionSchemaVariant) (json.RawMessage, error) {
	properties := map[string]json.RawMessage{}
	for propertyName, propertySchema := range variant.Properties {
		if propertyName == "action" || nativeOmittedPlanningActionFieldNames[propertyName] {
			continue
		}
		properties[propertyName] = propertySchema
	}
	required := []string{}
	for _, fieldName := range variant.Required {
		if strings.TrimSpace(fieldName) == "action" || nativeOmittedPlanningActionFieldNames[fieldName] {
			continue
		}
		required = append(required, fieldName)
	}
	return nativeStrictObjectParameters(properties, required)
}

func nativeStrictObjectParameters(properties map[string]json.RawMessage, required []string) (json.RawMessage, error) {
	document := map[string]any{
		"type":       "object",
		"properties": nativeRawSchemaProperties(properties),
		"required":   required,
	}
	content, errorValue := json.Marshal(nativeStrictSchemaValue(document))
	if errorValue != nil {
		return nil, errorValue
	}
	return content, nil
}

func nativeRawSchemaProperties(properties map[string]json.RawMessage) map[string]any {
	result := map[string]any{}
	for propertyName, propertySchema := range properties {
		var document any
		if errorValue := json.Unmarshal(propertySchema, &document); errorValue != nil {
			result[propertyName] = map[string]any{"type": "object", "properties": map[string]any{}}
			continue
		}
		result[propertyName] = document
	}
	return result
}

func nativeStrictSchemaValue(value any) any {
	document, isDocument := value.(map[string]any)
	if isDocument {
		clone := map[string]any{}
		requiredFieldNames := stringSetForNativeSchema(stringSliceFromNativeSchema(document["required"]))
		for fieldName, fieldValue := range document {
			if fieldName == "required" {
				continue
			}
			if fieldName == "type" {
				clone[fieldName] = nativeStrictSchemaType(fieldValue)
				continue
			}
			if fieldName == "properties" {
				clone[fieldName] = nativeStrictSchemaProperties(fieldValue, requiredFieldNames)
				continue
			}
			clone[fieldName] = nativeStrictSchemaValue(fieldValue)
		}
		if clone["type"] == "object" {
			properties := mapFromNativeSchema(clone["properties"])
			clone["properties"] = properties
			clone["required"] = sortedNativeSchemaPropertyNames(properties)
		}
		return clone
	}
	values, isValues := value.([]any)
	if isValues {
		clone := make([]any, 0, len(values))
		for _, item := range values {
			clone = append(clone, nativeStrictSchemaValue(item))
		}
		return clone
	}
	return value
}

func nativeStrictSchemaProperties(value any, requiredFieldNames map[string]bool) map[string]any {
	properties := mapFromNativeSchema(value)
	clone := map[string]any{}
	for propertyName, propertySchema := range properties {
		normalizedSchema := nativeStrictSchemaValue(propertySchema)
		if !requiredFieldNames[propertyName] {
			normalizedSchema = nullableNativeSchemaValue(normalizedSchema)
		}
		clone[propertyName] = normalizedSchema
	}
	return clone
}

func nativeStrictSchemaType(value any) any {
	if value == "integer" {
		return "number"
	}
	values, isValues := value.([]any)
	if !isValues {
		return value
	}
	clone := make([]any, 0, len(values))
	for _, item := range values {
		if item == "integer" {
			clone = append(clone, "number")
			continue
		}
		clone = append(clone, item)
	}
	return clone
}

func nullableNativeSchemaValue(value any) any {
	document, isDocument := value.(map[string]any)
	if !isDocument {
		return value
	}
	clone := map[string]any{}
	for fieldName, fieldValue := range document {
		clone[fieldName] = fieldValue
	}
	clone["type"] = nullableNativeSchemaType(clone["type"])
	if enumValues, ok := clone["enum"].([]any); ok && !nativeEnumContainsNull(enumValues) {
		clone["enum"] = append(enumValues, nil)
	}
	return clone
}

func nullableNativeSchemaType(value any) any {
	values, isValues := value.([]any)
	if isValues {
		for _, item := range values {
			if item == nil || item == "null" {
				return values
			}
		}
		return append(values, "null")
	}
	if strings.TrimSpace(stringFromNativeSchema(value)) == "" {
		return value
	}
	return []any{value, "null"}
}

func nativeEnumContainsNull(values []any) bool {
	for _, value := range values {
		if value == nil {
			return true
		}
	}
	return false
}

func sortedNativeSchemaPropertyNames(properties map[string]any) []string {
	names := make([]string, 0, len(properties))
	for name := range properties {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func stringSetForNativeSchema(values []string) map[string]bool {
	result := map[string]bool{}
	for _, value := range values {
		result[value] = true
	}
	return result
}

func stringSliceFromNativeSchema(value any) []string {
	stringValues, isStringValues := value.([]string)
	if isStringValues {
		return append([]string{}, stringValues...)
	}
	values, isValues := value.([]any)
	if !isValues {
		return nil
	}
	result := []string{}
	for _, item := range values {
		stringValue := strings.TrimSpace(stringFromNativeSchema(item))
		if stringValue != "" {
			result = append(result, stringValue)
		}
	}
	return result
}

func mapFromNativeSchema(value any) map[string]any {
	document, isDocument := value.(map[string]any)
	if isDocument {
		return document
	}
	return map[string]any{}
}

func stringFromNativeSchema(value any) string {
	stringValue, _ := value.(string)
	return stringValue
}

func enumStringValue(schema json.RawMessage) (string, bool) {
	var document struct {
		Enum []string `json:"enum"`
	}
	if json.Unmarshal(schema, &document) != nil || len(document.Enum) == 0 {
		return "", false
	}
	value := strings.TrimSpace(document.Enum[0])
	return value, value != ""
}

func nativeActionFunctionName(action string, toolName string) string {
	return nativeSafeFunctionName(action) + "__" + nativeSafeFunctionName(toolName)
}

func nativeControlFunctionName(action string) string {
	switch strings.TrimSpace(action) {
	case "finish":
		return "finish"
	}
	return nativeSafeFunctionName(action)
}

func nativeToolDescription(description string, toolName string) string {
	if trimmed := strings.TrimSpace(description); trimmed != "" {
		return trimmed
	}
	return "Call " + toolName
}

func nativeControlDescription(action string) string {
	switch strings.TrimSpace(action) {
	case "finish":
		return "Finish the task only when the user goal is satisfied and completion evidence is available."
	case "tool.request":
		return "Load tools or skills into your callable set by exact name. The action schema exposes only a subset of your tools each turn; when the tool you need is listed in the tool catalog but absent from this turn's schema, call this with its toolNames (and skillNames) and it becomes directly callable next turn. Never run a tool name as a shell command instead."
	}
	return "Return agent action " + action
}

func nativeSafeFunctionName(value string) string {
	normalized := nativeFunctionNamePattern.ReplaceAllString(strings.TrimSpace(value), "_")
	normalized = strings.Trim(normalized, "_")
	if normalized == "" {
		return "action"
	}
	return normalized
}
