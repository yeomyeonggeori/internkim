package capabilityd

import (
	"encoding/json"
	"errors"
	"fmt"

	"gitlab.com/eastriver/internkim/internal/capabilities"
	capabilityschema "gitlab.com/eastriver/internkim/pkg/capabilityprotocol/jsonschema"
)

func capabilitySuccessResponse(toolName string, status string, result json.RawMessage) (capabilities.ToolInvokeResponse, error) {
	descriptor, isRegistered := capabilityToolDescriptorFor(toolName)
	if !isRegistered {
		return capabilities.ToolInvokeResponse{}, errors.New("capability tool descriptor is missing: " + toolName)
	}
	if descriptor.ResultContract == nil {
		return capabilities.ToolInvokeResponse{}, errors.New("capability tool result contract is missing: " + toolName)
	}
	if errorValue := capabilityschema.Validate(descriptor.ResultContract.Schema, result); errorValue != nil {
		return capabilities.ToolInvokeResponse{}, fmt.Errorf("capability tool result violates %s contract: %w", toolName, errorValue)
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
