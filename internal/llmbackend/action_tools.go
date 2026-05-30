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
	"required":    true,
	"type":        true,
}

const openRouterNativeToolMaxFunctionCount = 20

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
	ToolNames    []string
	IsDispatcher bool
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
	if len(toolSet.Tools) > openRouterNativeToolMaxFunctionCount {
		toolSet = compactNativeActionToolSet(toolSet)
	}
	return toolSet, true, nil
}

func compactNativeActionToolSet(toolSet nativeActionToolSet) nativeActionToolSet {
	controlTools := []nativeActionTool{}
	toolNames := []string{}
	for _, tool := range toolSet.Tools {
		if isNativeToolAction(tool.Action) {
			toolNames = append(toolNames, tool.ToolName)
			continue
		}
		controlTools = append(controlTools, tool)
	}
	if len(toolNames) == 0 {
		return toolSet
	}
	compactToolSet := nativeActionToolSet{ToolByName: map[string]nativeActionTool{}}
	for _, tool := range controlTools {
		compactToolSet = addNativeActionTool(compactToolSet, tool)
	}
	compactToolSet = addNativeActionTool(compactToolSet, nativeContinueDispatcherTool(toolNames))
	return compactToolSet
}

func addNativeActionTool(toolSet nativeActionToolSet, tool nativeActionTool) nativeActionToolSet {
	toolSet.Tools = append(toolSet.Tools, tool)
	toolSet.ToolByName[tool.FunctionName] = tool
	return toolSet
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
		return nativeActionTool{
			FunctionName: nativeActionFunctionName(action, toolName),
			Description:  "Call " + toolName,
			Action:       action,
			ToolName:     toolName,
			Parameters:   toolActionParameters(variant),
		}, true, nil
	}
	return nativeActionTool{
		FunctionName: nativeControlFunctionName(action),
		Description:  nativeControlDescription(action),
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
	if tool.IsDispatcher {
		return dispatchContinueActionPayload(tool, argumentDocument)
	}
	return fixedContinueActionPayload(tool, argumentDocument)
}

func fixedContinueActionPayload(tool nativeActionTool, argumentDocument json.RawMessage) (map[string]any, error) {
	var payload map[string]any
	if errorValue := json.Unmarshal(argumentDocument, &payload); errorValue != nil {
		return nil, errorValue
	}
	toolInput := payload["toolInput"]
	if toolInput == nil {
		toolInput = map[string]any{}
	}
	payload["action"] = tool.Action
	payload["toolName"] = tool.ToolName
	payload["toolInput"] = toolInput
	return payload, nil
}

func dispatchContinueActionPayload(tool nativeActionTool, argumentDocument json.RawMessage) (map[string]any, error) {
	var payload map[string]any
	if errorValue := json.Unmarshal(argumentDocument, &payload); errorValue != nil {
		return nil, errorValue
	}
	toolName, _ := payload["toolName"].(string)
	toolName = strings.TrimSpace(toolName)
	if !stringSliceContains(tool.ToolNames, toolName) {
		return nil, errors.New("native continue dispatcher referenced unknown tool: " + toolName)
	}
	toolInput, errorValue := dispatchedToolInput(payload)
	if errorValue != nil {
		return nil, errorValue
	}
	payload["action"] = tool.Action
	payload["toolName"] = toolName
	payload["toolInput"] = toolInput
	delete(payload, "toolInputJSON")
	return payload, nil
}

func dispatchedToolInput(payload map[string]any) (any, error) {
	if toolInput, isFound := payload["toolInput"]; isFound {
		return toolInput, nil
	}
	toolInputJSONString, _ := payload["toolInputJSON"].(string)
	toolInputJSONString = strings.TrimSpace(toolInputJSONString)
	if toolInputJSONString == "" {
		return map[string]any{}, nil
	}
	var toolInput any
	if errorValue := json.Unmarshal([]byte(toolInputJSONString), &toolInput); errorValue != nil {
		return nil, errors.New("native continue dispatcher toolInputJSON was not valid JSON: " + errorValue.Error())
	}
	return toolInput, nil
}

func stringSliceContains(values []string, expected string) bool {
	for _, value := range values {
		if strings.TrimSpace(value) == expected {
			return true
		}
	}
	return false
}

func toolActionParameters(variant actionSchemaVariant) json.RawMessage {
	properties := map[string]json.RawMessage{
		"toolInput": variant.Properties["toolInput"],
	}
	for _, fieldName := range []string{"message", "reason", "goalStatus", "goalSatisfied", "remainingWork", "executionStateUpdate"} {
		if propertySchema, isFound := variant.Properties[fieldName]; isFound {
			properties[fieldName] = propertySchema
		}
	}
	content, errorValue := json.Marshal(map[string]any{
		"type":                 "object",
		"properties":           properties,
		"required":             []string{"toolInput", "executionStateUpdate"},
		"additionalProperties": false,
	})
	if errorValue != nil {
		return json.RawMessage(`{"type":"object","properties":{}}`)
	}
	document, errorValue := ProjectToolParametersForProvider(content, ToolParameterProjectionOpenRouterNativeTool)
	if errorValue != nil {
		return json.RawMessage(`{"type":"object","properties":{}}`)
	}
	return document
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

func nativeContinueDispatcherTool(toolNames []string) nativeActionTool {
	return nativeActionTool{
		FunctionName: "continue",
		Description:  "Continue work by calling one available tool. toolInputJSON must be a JSON string matching the selected tool input schema.",
		Action:       "continue",
		Parameters:   nativeContinueDispatcherParameters(toolNames),
		ToolNames:    toolNames,
		IsDispatcher: true,
	}
}

func nativeContinueDispatcherParameters(toolNames []string) json.RawMessage {
	content, errorValue := json.Marshal(map[string]any{
		"type": "object",
		"properties": map[string]any{
			"toolName": map[string]any{
				"type": "string",
				"enum": toolNames,
			},
			"toolInputJSON": map[string]any{
				"type":        "string",
				"description": "JSON-encoded input for the selected tool. Use {} when the tool takes no input.",
			},
			"message":              map[string]any{"type": "string"},
			"reason":               map[string]any{"type": "string"},
			"goalStatus":           map[string]any{"type": "string", "enum": []string{"in_progress"}},
			"goalSatisfied":        map[string]any{"type": "boolean"},
			"remainingWork":        map[string]any{"type": "string"},
			"executionStateUpdate": nativeDispatcherExecutionStateSchema(),
		},
		"required":             []string{"toolName", "toolInputJSON", "executionStateUpdate"},
		"additionalProperties": false,
	})
	if errorValue != nil {
		return json.RawMessage(`{"type":"object","properties":{}}`)
	}
	document, errorValue := ProjectToolParametersForProvider(content, ToolParameterProjectionOpenRouterNativeTool)
	if errorValue != nil {
		return json.RawMessage(`{"type":"object","properties":{}}`)
	}
	return document
}

func nativeDispatcherExecutionStateSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"goal":           map[string]any{"type": "string"},
			"workspace":      map[string]any{"type": "string"},
			"knownFacts":     map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
			"triedAndFailed": map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
			"currentBlocker": map[string]any{"type": "string"},
			"nextPlan":       map[string]any{"type": "string"},
		},
	}
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
		if dialect == ToolParameterProjectionOpenRouterNativeTool && fieldName == "enum" {
			if projectedEnum, isFound := projectOpenRouterNativeToolEnum(fieldValue); isFound {
				projectedDocument[fieldName] = projectedEnum
			}
			continue
		}
		projectedDocument[fieldName] = projectToolParametersValue(fieldValue, dialect)
	}
	return normalizeProjectedToolParameters(projectedDocument)
}

func projectOpenRouterNativeToolEnum(value any) ([]string, bool) {
	values, isArray := value.([]any)
	if !isArray {
		return nil, false
	}
	enumValues := make([]string, 0, len(values))
	for _, value := range values {
		enumValue, isString := value.(string)
		if !isString || enumValue == "" {
			return nil, false
		}
		enumValues = append(enumValues, enumValue)
	}
	return enumValues, len(enumValues) > 0
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
		properties := nativeToolSchemaProperties(document["properties"])
		if properties == nil {
			document["properties"] = map[string]any{}
		}
		document = normalizeProjectedToolRequired(document)
	}
	return document
}

func normalizeProjectedToolRequired(document map[string]any) map[string]any {
	properties := nativeToolSchemaProperties(document["properties"])
	required, isFound := document["required"].([]any)
	if !isFound {
		return document
	}
	filteredRequired := make([]string, 0, len(required))
	for _, fieldName := range required {
		fieldNameString, isString := fieldName.(string)
		if !isString {
			continue
		}
		if _, isProperty := properties[fieldNameString]; isProperty {
			filteredRequired = append(filteredRequired, fieldNameString)
		}
	}
	if len(filteredRequired) == 0 {
		delete(document, "required")
		return document
	}
	document["required"] = filteredRequired
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

func nativeControlDescription(action string) string {
	switch strings.TrimSpace(action) {
	case "finish":
		return "Finish the task only when the user goal is satisfied and completion evidence is available."
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
