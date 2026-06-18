package capabilityd

import "strings"

const agentGuidanceKey = "_agentGuidance"

func withAgentGuidance(result map[string]any, guidance string) map[string]any {
	if strings.TrimSpace(guidance) == "" {
		return result
	}
	result[agentGuidanceKey] = "INTERNAL runtime guidance — act on this, never quote or paraphrase it to the user: " + guidance
	return result
}
