package capabilityd

import (
	"encoding/json"
	"errors"
	"fmt"

	"gitlab.com/eastriver/internkim/internal/capabilities"
	capabilityschema "gitlab.com/eastriver/internkim/pkg/capabilityprotocol/jsonschema"
)

type capabilityResponseOrigin struct {
	Provider        string
	SelectedBackend string
	Content         string
}

func capabilitySuccessResponse(toolName string, status string, result json.RawMessage) (capabilities.ToolInvokeResponse, error) {
	return capabilitySuccessResponseFrom(toolName, status, result, capabilityResponseOrigin{Provider: "internkim", SelectedBackend: "device"})
}

func capabilitySuccessResponseFrom(toolName string, status string, result json.RawMessage, origin capabilityResponseOrigin) (capabilities.ToolInvokeResponse, error) {
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
		Provider:        origin.Provider,
		SelectedBackend: origin.SelectedBackend,
		ToolName:        descriptor.CanonicalName,
		Outcome:         capabilities.ToolOutcomeSucceeded,
		Effects:         effects,
		Status:          status,
		Content:         origin.Content,
		Result:          result,
	}, nil
}

func capabilityToolHasResultContract(toolName string) bool {
	descriptor, isRegistered := capabilityToolDescriptorFor(toolName)
	return isRegistered && descriptor.ResultContract != nil
}
