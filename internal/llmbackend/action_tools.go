package llmbackend

import (
	"encoding/json"
	"errors"
	"regexp"
	"strings"
)

var nativeFunctionNamePattern = regexp.MustCompile(`[^A-Za-z0-9_-]+`)

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
	tool, isFound := toolSet.ToolByName[strings.TrimSpace(functionName)]
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
	return content
}

func objectSchemaDocument(schema json.RawMessage) json.RawMessage {
	if len(schema) == 0 {
		return json.RawMessage(`{"type":"object","properties":{},"additionalProperties":false}`)
	}
	return schema
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
