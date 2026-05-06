package capabilities

type AttentionTriageRequest struct {
	JobID             string        `json:"jobID"`
	ParentJobID       string        `json:"parentJobID,omitempty"`
	ToolName          string        `json:"toolName"`
	Status            string        `json:"status"`
	RequesterPersonID string        `json:"requesterPersonID,omitempty"`
	RequesterEmail    string        `json:"requesterEmail,omitempty"`
	PrivacyClass      string        `json:"privacyClass"`
	ResourceScope     ResourceScope `json:"resourceScope,omitempty"`
	WatchAttemptCount int           `json:"watchAttemptCount"`
	LastAttentionAt   string        `json:"lastAttentionAt,omitempty"`
	CreatedAt         string        `json:"createdAt,omitempty"`
	UpdatedAt         string        `json:"updatedAt,omitempty"`
	ExpiresAt         string        `json:"expiresAt,omitempty"`
	Error             string        `json:"error,omitempty"`
	DenialCode        string        `json:"denialCode,omitempty"`
}

type AttentionTriageDecision struct {
	ShouldEscalate   bool     `json:"shouldEscalate"`
	Importance       string   `json:"importance"`
	Confidence       float64  `json:"confidence"`
	ReasonCodes      []string `json:"reasonCodes"`
	SummaryForRemote string   `json:"summaryForRemote"`
	PrivacyClass     string   `json:"privacyClass"`
}

type RemoteAttentionRequest struct {
	JobID             string                  `json:"jobID"`
	ToolName          string                  `json:"toolName"`
	RequesterPersonID string                  `json:"requesterPersonID,omitempty"`
	RequesterEmail    string                  `json:"requesterEmail,omitempty"`
	ConversationID    string                  `json:"conversationID,omitempty"`
	Platform          string                  `json:"platform,omitempty"`
	PrivacyClass      string                  `json:"privacyClass"`
	ResourceScope     ResourceScope           `json:"resourceScope,omitempty"`
	LocalDecision     AttentionTriageDecision `json:"localDecision"`
	DeduplicationKey  string                  `json:"deduplicationKey"`
}

type RemoteAttentionResponse struct {
	Status  string `json:"status"`
	Message string `json:"message,omitempty"`
	Reason  string `json:"reason,omitempty"`
}
