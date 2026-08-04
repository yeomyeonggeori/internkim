package companion

import (
	"gitlab.com/eastriver/internkim/internal/capabilities"
)

// The person who asked for the work approves it in chat, and the agent remembers
// that approval for the rest of the task. The companion runs no second gate of its
// own: a job that reaches it has already been approved by the person who asked.

type JobEnvelope struct {
	JobID         string                     `json:"jobID"`
	ParentJobID   string                     `json:"parentJobID,omitempty"`
	GrantID       string                     `json:"grantID,omitempty"`
	ToolName      string                     `json:"toolName"`
	PrivacyClass  string                     `json:"privacyClass"`
	ResourceScope capabilities.ResourceScope `json:"resourceScope,omitempty"`
	Depth         int                        `json:"depth"`
}

type DenialError struct {
	Denial capabilities.DenialResult
}

func (errorValue DenialError) Error() string {
	return "companion job denied"
}
