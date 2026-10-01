package capabilityd

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strings"

	"github.com/yeomyeonggeori/internkim/internal/capabilities"
	"github.com/yeomyeonggeori/internkim/pkg/capabilityprotocol"
	capabilityschema "github.com/yeomyeonggeori/internkim/pkg/capabilityprotocol/jsonschema"
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
	if errorValue := holdResultToContract(descriptor, result); errorValue != nil {
		return capabilities.ToolInvokeResponse{}, errorValue
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

// A contract is held by the side that can be fixed when it is broken, and
// answeredBy says which side that is. What this machine answers is written and
// checked in the same release, so a break here is a break in code that is
// already installed: refusing is how the writer hears about it.
//
// A record tool is answered on the plane, which deploys on its own schedule,
// and this copy of the contract arrives only with an OTA release. Refusing the
// plane's answer against a copy this machine has no way to refresh spends a
// fleet-wide outage to report a fault nobody here can fix: every record tool
// answering correctly and being refused anyway until the next release (#1486).
// The plane holds its own answer to the contract it authored, where the break
// is reported to the side that authored it, and this side reads the answer by
// name the way ProjectResourceEffects already does.
func holdResultToContract(descriptor capabilities.Descriptor, result json.RawMessage) error {
	if descriptor.AnsweredBy == capabilityprotocol.AnsweredByRecord {
		return nil
	}
	check, errorValue := capabilityschema.ValidateResult(descriptor.ResultContract.Schema, result)
	if errorValue != nil {
		return fmt.Errorf("capability tool %s: %w", descriptor.CanonicalName, errorValue)
	}
	if len(check.UnknownFields) > 0 {
		log.Printf("capability.result_carried_unknown_fields: tool=%s fields=%s", descriptor.CanonicalName, strings.Join(check.UnknownFields, " "))
	}
	return nil
}
