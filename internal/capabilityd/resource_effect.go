package capabilityd

import (
	"encoding/json"
	"errors"

	"gitlab.com/eastriver/internkim/internal/capabilities"
)

func capabilityMutationResponse(toolName string, status string, result json.RawMessage) (capabilities.ToolInvokeResponse, error) {
	descriptor, isRegistered := capabilityToolDescriptorFor(toolName)
	if !isRegistered {
		return capabilities.ToolInvokeResponse{}, errors.New("capability tool descriptor is missing: " + toolName)
	}
	effects, errorValue := capabilities.ProjectResourceEffects(descriptor.ResultContract, result)
	if errorValue != nil {
		return capabilities.ToolInvokeResponse{}, errorValue
	}
	return capabilities.ToolInvokeResponse{
		Provider:        "internkim",
		SelectedBackend: "device",
		ToolName:        descriptor.CanonicalName,
		Outcome:         capabilities.ToolOutcomeSucceeded,
		Effects:         effects,
		Status:          status,
		Result:          result,
	}, nil
}
