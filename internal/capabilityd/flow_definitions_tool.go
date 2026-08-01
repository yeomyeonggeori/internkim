package capabilityd

import (
	"context"
	"encoding/json"

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

type flowDefinitionLabel struct {
	Value string `json:"value"`
	Color string `json:"color,omitempty"`
}

func (service Service) invokeFlowTaskDefinitions(ctx context.Context, request capabilities.ToolInvokeRequest) (capabilities.ToolInvokeResponse, error) {
	definitions, statuses, errorValue := service.fetchFlowDefinitions(ctx, request.Context.RequesterEmail)
	if errorValue != nil {
		return capabilities.ToolInvokeResponse{}, errorValue
	}
	result, errorValue := json.Marshal(map[string]any{
		"businesses": flowDefinitionLabels(definitions.Categories, definitions.CategoryColors),
		"types":      flowDefinitionLabels(definitions.Types, definitions.TypeColors),
		"sizes":      flowSizeNames(definitions.Sizes),
		"statuses":   nonNilStrings(statuses),
	})
	if errorValue != nil {
		return capabilities.ToolInvokeResponse{}, errorValue
	}
	return capabilitySuccessResponse(request.ToolName, "ok", result)
}

func (service Service) fetchFlowDefinitions(ctx context.Context, requesterEmail string) (flowDefinitionsForTool, []string, error) {
	body, errorValue := service.getFlow(ctx, "/flow/api/state", requesterEmail)
	if errorValue != nil {
		return flowDefinitionsForTool{}, nil, errorValue
	}
	var state struct {
		Definitions   flowDefinitionsForTool `json:"definitions"`
		StatusOptions []string               `json:"statusOptions"`
	}
	if errorValue := json.Unmarshal(body, &state); errorValue != nil {
		return flowDefinitionsForTool{}, nil, errorValue
	}
	return state.Definitions, state.StatusOptions, nil
}

func flowDefinitionLabels(values []string, colors map[string]string) []flowDefinitionLabel {
	labels := make([]flowDefinitionLabel, 0, len(values))
	for _, value := range values {
		labels = append(labels, flowDefinitionLabel{Value: value, Color: colors[value]})
	}
	return labels
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
