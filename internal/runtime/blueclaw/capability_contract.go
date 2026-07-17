package blueclaw

import (
	"encoding/json"

	"gitlab.com/eastriver/internkim/internal/capabilities"
)

type CapabilityContract struct {
	Version                    int                       `json:"version"`
	ToolDescriptors            []capabilities.Descriptor `json:"toolDescriptors"`
	RoutingCandidates          []string                  `json:"routingCandidates"`
	PolicyResourceReplacements map[string]string         `json:"policyResourceReplacements"`
	PolicyResourceDefaults     []map[string]any          `json:"policyResourceDefaults"`
}

func CurrentCapabilityContract() CapabilityContract {
	return CapabilityContract{
		Version:                    2,
		ToolDescriptors:            capabilities.DefaultToolDescriptors(),
		RoutingCandidates:          capabilities.RoutingCandidates(),
		PolicyResourceReplacements: currentPolicyResourceReplacements(),
		PolicyResourceDefaults:     defaultResourceAccessPolicies(),
	}
}

func CapabilityContractDocument() (string, error) {
	document, errorValue := json.MarshalIndent(CurrentCapabilityContract(), "", "  ")
	if errorValue != nil {
		return "", errorValue
	}
	return string(document) + "\n", nil
}

func currentPolicyResourceReplacements() map[string]string {
	replacements := map[string]string{}
	for legacyToolName, currentToolName := range capabilities.LegacyToolNameReplacements() {
		replacements["tool:"+legacyToolName] = "tool:" + currentToolName
	}
	return replacements
}
