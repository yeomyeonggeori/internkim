package capabilityd

import (
	"strings"

	"gitlab.com/eastriver/internkim/internal/capabilities"
)

type flowDefinitionsForTool struct {
	Categories     []string                    `json:"categories"`
	CategoryColors map[string]string           `json:"categoryColors"`
	Types          []string                    `json:"types"`
	TypeColors     map[string]string           `json:"typeColors"`
	Sizes          []flowSizeDefinitionForTool `json:"sizes"`
}

type flowSizeDefinitionForTool struct {
	Name string `json:"name"`
}

func resolveFlowTaskLabels(input flowTaskUpdateInput, definitions flowDefinitionsForTool) (flowTaskUpdateInput, *flowTaskLabelFailure) {
	if input.Category != nil {
		resolvedCategory, failure := resolveFlowLabel(*input.Category, "category", definitions.Categories)
		if failure != nil {
			return flowTaskUpdateInput{}, failure
		}
		input.Category = &resolvedCategory
	}
	if input.Type != nil {
		resolvedType, failure := resolveFlowLabel(*input.Type, "type", definitions.Types)
		if failure != nil {
			return flowTaskUpdateInput{}, failure
		}
		input.Type = &resolvedType
	}
	return input, nil
}

func resolveFlowLabel(value string, fieldName string, registeredLabels []string) (string, *flowTaskLabelFailure) {
	trimmedValue := strings.TrimSpace(value)
	if trimmedValue == "" || len(registeredLabels) == 0 {
		return trimmedValue, nil
	}
	matches := matchingFlowLabels(trimmedValue, registeredLabels)
	if len(matches) == 1 {
		return matches[0], nil
	}
	return "", unregisteredFlowLabelFailure(trimmedValue, fieldName, matches, registeredLabels)
}

func matchingFlowLabels(value string, registeredLabels []string) []string {
	normalizedValue := strings.ToLower(value)
	for _, label := range registeredLabels {
		if strings.ToLower(strings.TrimSpace(label)) == normalizedValue {
			return []string{label}
		}
	}
	matches := []string{}
	for _, label := range registeredLabels {
		if strings.Contains(strings.ToLower(strings.TrimSpace(label)), normalizedValue) {
			matches = append(matches, label)
		}
	}
	return matches
}

func unregisteredFlowLabelFailure(value string, fieldName string, matches []string, registeredLabels []string) *flowTaskLabelFailure {
	if len(matches) > 1 {
		return &flowTaskLabelFailure{
			ErrorCode:     "flow_label_ambiguous",
			FailureStage:  "label_resolution",
			Message:       fieldName + " " + value + " matches more than one registered label; retry with one of them exactly",
			Field:         fieldName,
			Candidates:    matches,
			RecoveryHints: retryWithARegisteredLabelHint(fieldName),
			Retryable:     true,
			SafeRetry:     true,
		}
	}
	return &flowTaskLabelFailure{
		ErrorCode:     "flow_label_not_registered",
		FailureStage:  "label_resolution",
		Message:       fieldName + " " + value + " is not registered in this workspace; retry with one of the registered labels or leave the field unchanged",
		Field:         fieldName,
		Candidates:    registeredLabels,
		RecoveryHints: retryWithARegisteredLabelHint(fieldName),
		Retryable:     true,
		SafeRetry:     true,
	}
}

func retryWithARegisteredLabelHint(fieldName string) []capabilities.RecoveryHint {
	return []capabilities.RecoveryHint{{
		Action:    "retry_with_a_registered_label",
		ToolNames: []string{"task_update"},
		Reason:    "only labels this workspace registers are accepted for " + fieldName,
	}}
}

type registeredFlowLabelsForTool struct {
	Businesses []string `json:"businesses"`
	Types      []string `json:"types"`
	Sizes      []string `json:"sizes"`
	Statuses   []string `json:"statuses"`
}

func registeredFlowLabels(definitions flowDefinitionsForTool) registeredFlowLabelsForTool {
	return registeredFlowLabelsForTool{
		Businesses: nonNilStrings(definitions.Categories),
		Types:      nonNilStrings(definitions.Types),
		Sizes:      flowSizeNames(definitions.Sizes),
		Statuses:   flowTaskUpdateStatuses(),
	}
}

func flowSizeNames(sizes []flowSizeDefinitionForTool) []string {
	names := make([]string, 0, len(sizes))
	for _, size := range sizes {
		names = append(names, size.Name)
	}
	return names
}

func nonNilStrings(values []string) []string {
	if values == nil {
		return []string{}
	}
	return values
}
