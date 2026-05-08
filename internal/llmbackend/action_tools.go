package llmbackend

import (
	"encoding/json"
	"errors"
	"regexp"
	"strings"
)

var nativeFunctionNamePattern = regexp.MustCompile(`[^A-Za-z0-9_-]+`)

var openRouterNativeToolSchemaKeywordByName = map[string]bool{
	"description": true,
	"enum":        true,
	"items":       true,
	"properties":  true,
	"type":        true,
}

type nativeActionToolSet struct {
	Tools      []nativeActionTool
	ToolByName map[string]nativeActionTool
}

type nativeActionTool struct {
	FunctionName string
	Description  string
	Action       string
	ToolName     string
	Parameters   json.RawMessage
}

type ToolParameterProjectionDialect string

const ToolParameterProjectionOpenRouterNativeTool ToolParameterProjectionDialect = "openrouter_native_tool"

type actionSchemaDocument struct {
	OneOf []actionSchemaVariant `json:"oneOf"`
}

type actionSchemaVariant struct {
	Properties map[string]json.RawMessage `json:"properties"`
	Required   []string                   `json:"required"`
}

func nativeActionToolsForSchema(schema StructuredOutputSchema) (nativeActionToolSet, bool, error) {
	if strings.TrimSpace(schema.Name) != "blueclaw_agent_turn_action" {
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
	return toolSet, true, nil
}

func nativeActionToolForVariant(variant actionSchemaVariant) (nativeActionTool, bool, error) {
	action, isFound := enumStringValue(variant.Properties["action"])
	if !isFound {
		return nativeActionTool{}, false, nil
	}
	if action == "call_tool" {
		toolName, isFound := enumStringValue(variant.Properties["toolName"])
		if !isFound {
			return nativeActionTool{}, false, errors.New("call_tool action schema is missing toolName enum")
		}
		return nativeActionTool{
			FunctionName: nativeActionFunctionName(toolName),
			Description:  "Call " + toolName,
			Action:       action,
			ToolName:     toolName,
			Parameters:   objectSchemaDocument(variant.Properties["toolInput"]),
		}, true, nil
	}
	return nativeActionTool{
		FunctionName: nativeControlFunctionName(action),
		Description:  "Return agent action " + action,
		Action:       action,
		Parameters:   controlActionParameters(variant),
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
	if tool.Action == "call_tool" {
		content, errorValue := json.Marshal(map[string]any{
			"action":    "call_tool",
			"toolName":  tool.ToolName,
			"toolInput": argumentDocument,
		})
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
	if tool.Action != "call_tool" {
		return false
	}
	return functionName == tool.ToolName || functionName == nativeSafeFunctionName(tool.ToolName)
}

func controlActionParameters(variant actionSchemaVariant) json.RawMessage {
	properties := map[string]json.RawMessage{}
	for propertyName, propertySchema := range variant.Properties {
		if propertyName == "action" {
			continue
		}
		properties[propertyName] = propertySchema
	}
	required := []string{}
	for _, fieldName := range variant.Required {
		if strings.TrimSpace(fieldName) != "action" {
			required = append(required, fieldName)
		}
	}
	content, errorValue := json.Marshal(map[string]any{
		"type":                 "object",
		"properties":           properties,
		"required":             required,
		"additionalProperties": false,
	})
	if errorValue != nil {
		return json.RawMessage(`{"type":"object","properties":{},"additionalProperties":false}`)
	}
	document, errorValue := ProjectToolParametersForProvider(content, ToolParameterProjectionOpenRouterNativeTool)
	if errorValue != nil {
		return json.RawMessage(`{"type":"object","properties":{}}`)
	}
	return document
}

func objectSchemaDocument(schema json.RawMessage) json.RawMessage {
	if len(schema) == 0 {
		return json.RawMessage(`{"type":"object","properties":{},"additionalProperties":false}`)
	}
	document, errorValue := ProjectToolParametersForProvider(schema, ToolParameterProjectionOpenRouterNativeTool)
	if errorValue != nil {
		return json.RawMessage(`{"type":"object","properties":{},"additionalProperties":false}`)
	}
	return document
}

func ProjectToolParametersForProvider(schema json.RawMessage, dialect ToolParameterProjectionDialect) (json.RawMessage, error) {
	var document any
	if errorValue := json.Unmarshal(schema, &document); errorValue != nil {
		return nil, errorValue
	}
	projectedDocument := projectToolParametersValue(document, dialect)
	content, errorValue := json.Marshal(projectedDocument)
	if errorValue != nil {
		return nil, errorValue
	}
	return content, nil
}

func projectToolParametersValue(value any, dialect ToolParameterProjectionDialect) any {
	switch typedValue := value.(type) {
	case []any:
		return projectToolParametersArray(typedValue, dialect)
	case map[string]any:
		return projectToolParametersObject(typedValue, dialect)
	default:
		return value
	}
}

func projectToolParametersArray(values []any, dialect ToolParameterProjectionDialect) []any {
	projectedValues := make([]any, 0, len(values))
	for _, value := range values {
		projectedValues = append(projectedValues, projectToolParametersValue(value, dialect))
	}
	return projectedValues
}

func projectToolParametersObject(document map[string]any, dialect ToolParameterProjectionDialect) any {
	if replacement, isFound := firstNativeToolSchemaUnionValue(document); isFound {
		return projectToolParametersValue(replacement, dialect)
	}
	projectedDocument := map[string]any{}
	for fieldName, fieldValue := range document {
		if dialect == ToolParameterProjectionOpenRouterNativeTool && fieldName == "properties" {
			projectedDocument[fieldName] = projectToolParameterProperties(fieldValue, dialect)
			continue
		}
		if !toolParameterProjectionAllowsSchemaKeyword(fieldName, dialect) {
			continue
		}
		projectedDocument[fieldName] = projectToolParametersValue(fieldValue, dialect)
	}
	return normalizeProjectedToolParameters(projectedDocument)
}

func projectToolParameterProperties(value any, dialect ToolParameterProjectionDialect) map[string]any {
	properties, isObject := value.(map[string]any)
	if !isObject {
		return map[string]any{}
	}
	projectedProperties := map[string]any{}
	for propertyName, propertySchema := range properties {
		projectedProperties[propertyName] = projectToolParametersValue(propertySchema, dialect)
	}
	return projectedProperties
}

func normalizeProjectedToolParameters(document map[string]any) map[string]any {
	switch document["type"] {
	case "array":
		if _, isFound := document["items"]; !isFound {
			document["items"] = map[string]any{"type": "object", "properties": map[string]any{}}
		}
	case "object":
		if nativeToolSchemaProperties(document["properties"]) == nil {
			document["properties"] = map[string]any{}
		}
	}
	return document
}

func firstNativeToolSchemaUnionValue(document map[string]any) (any, bool) {
	for _, fieldName := range []string{"oneOf", "anyOf", "allOf"} {
		values, isArray := document[fieldName].([]any)
		if isArray && len(values) > 0 {
			return values[0], true
		}
	}
	return nil, false
}

func toolParameterProjectionAllowsSchemaKeyword(fieldName string, dialect ToolParameterProjectionDialect) bool {
	if dialect != ToolParameterProjectionOpenRouterNativeTool {
		return true
	}
	return openRouterNativeToolSchemaKeywordByName[fieldName]
}

func nativeToolSchemaProperties(value any) map[string]any {
	properties, _ := value.(map[string]any)
	return properties
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

func nativeActionFunctionName(toolName string) string {
	return "call_tool__" + nativeSafeFunctionName(toolName)
}

func nativeControlFunctionName(action string) string {
	return nativeSafeFunctionName(action)
}

func nativeSafeFunctionName(value string) string {
	normalized := nativeFunctionNamePattern.ReplaceAllString(strings.TrimSpace(value), "_")
	normalized = strings.Trim(normalized, "_")
	if normalized == "" {
		return "action"
	}
	return normalized
}
