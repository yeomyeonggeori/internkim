package capabilityd

import (
	"strings"

	"gitlab.com/eastriver/internkim/internal/capabilities"
)

type taskDefinitionsForTool struct {
	Categories     []string                    `json:"categories"`
	CategoryColors map[string]string           `json:"categoryColors"`
	Types          []string                    `json:"types"`
	TypeColors     map[string]string           `json:"typeColors"`
	Sizes          []taskSizeDefinitionForTool `json:"sizes"`
}

type taskSizeDefinitionForTool struct {
	Name string `json:"name"`
}

func resolveTaskAddLabels(input taskAddInput, definitions taskDefinitionsForTool) (taskAddInput, *taskLabelFailure) {
	if input.Business != "" {
		resolvedCategory, failure := resolveTaskLabel(input.Business, "business", definitions.Categories)
		if failure != nil {
			return taskAddInput{}, failure
		}
		input.Business = resolvedCategory
	}
	if input.Type != "" {
		resolvedType, failure := resolveTaskLabel(input.Type, "type", definitions.Types)
		if failure != nil {
			return taskAddInput{}, failure
		}
		input.Type = resolvedType
	}
	return input, nil
}

func resolveTaskLabels(input taskUpdateInput, definitions taskDefinitionsForTool) (taskUpdateInput, *taskLabelFailure) {
	if input.Business != nil {
		resolvedCategory, failure := resolveTaskLabel(*input.Business, "business", definitions.Categories)
		if failure != nil {
			return taskUpdateInput{}, failure
		}
		input.Business = &resolvedCategory
	}
	if input.Type != nil {
		resolvedType, failure := resolveTaskLabel(*input.Type, "type", definitions.Types)
		if failure != nil {
			return taskUpdateInput{}, failure
		}
		input.Type = &resolvedType
	}
	return input, nil
}

func resolveTaskLabel(value string, fieldName string, registeredLabels []string) (string, *taskLabelFailure) {
	trimmedValue := strings.TrimSpace(value)
	if trimmedValue == "" || len(registeredLabels) == 0 {
		return trimmedValue, nil
	}
	matches := matchingTaskLabels(trimmedValue, registeredLabels)
	if len(matches) == 1 {
		return matches[0], nil
	}
	return "", unregisteredTaskLabelFailure(trimmedValue, fieldName, matches, registeredLabels)
}

func matchingTaskLabels(value string, registeredLabels []string) []string {
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

func unregisteredTaskLabelFailure(value string, fieldName string, matches []string, registeredLabels []string) *taskLabelFailure {
	if len(matches) > 1 {
		return &taskLabelFailure{
			ErrorCode:     "task_label_ambiguous",
			FailureStage:  "label_resolution",
			Message:       fieldName + " " + value + " matches more than one registered label; retry with one of them exactly",
			Field:         fieldName,
			Candidates:    matches,
			RecoveryHints: retryWithARegisteredLabelHint(fieldName),
			Retryable:     true,
			SafeRetry:     true,
		}
	}
	return &taskLabelFailure{
		ErrorCode:     "task_label_not_registered",
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

type registeredTaskLabelsForTool struct {
	Businesses []string `json:"businesses"`
	Types      []string `json:"types"`
	Sizes      []string `json:"sizes"`
	Statuses   []string `json:"statuses"`
}

func registeredTaskLabels(definitions taskDefinitionsForTool) registeredTaskLabelsForTool {
	return registeredTaskLabelsForTool{
		Businesses: nonNilStrings(definitions.Categories),
		Types:      nonNilStrings(definitions.Types),
		Sizes:      taskSizeNames(definitions.Sizes),
		Statuses:   taskUpdateStatuses(),
	}
}

func taskSizeNames(sizes []taskSizeDefinitionForTool) []string {
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
