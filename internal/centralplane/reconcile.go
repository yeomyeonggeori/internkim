package centralplane

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

type ReconciledEvent struct {
	ExternalID string    `json:"externalID"`
	Kind       string    `json:"kind"`
	Location   string    `json:"location,omitempty"`
	OccurredAt time.Time `json:"occurredAt"`
}

type ReconcileWindow struct {
	Platform string
	WorkMode string
	From     time.Time
	To       time.Time
	Events   []ReconciledEvent
}

type ReconcileResult struct {
	Added   int      `json:"added"`
	Removed int      `json:"removed"`
	Refused []string `json:"refused"`
}

func (client *Client) ReconcileAttendance(ctx context.Context, window ReconcileWindow) (ReconcileResult, error) {
	if window.Events == nil {
		window.Events = []ReconciledEvent{}
	}
	payload, errorValue := json.Marshal(map[string]any{
		"platform": window.Platform,
		"workMode": window.WorkMode,
		"from":     window.From.UTC().Format(time.RFC3339),
		"to":       window.To.UTC().Format(time.RFC3339),
		"events":   window.Events,
	})
	if errorValue != nil {
		return ReconcileResult{}, errorValue
	}

	request, errorValue := http.NewRequestWithContext(ctx, http.MethodPost,
		strings.TrimSuffix(client.settings.AppURL, "/")+"/api/agent/attendance-reconcile", bytes.NewReader(payload))
	if errorValue != nil {
		return ReconcileResult{}, errorValue
	}
	request.Header.Set("Authorization", "Bearer "+client.settings.AgentAPIKey)
	request.Header.Set("Content-Type", "application/json")

	response, errorValue := client.httpClient.Do(request)
	if errorValue != nil {
		return ReconcileResult{}, errorValue
	}
	defer response.Body.Close()
	if response.StatusCode >= 300 {
		return ReconcileResult{}, fmt.Errorf("central plane refused the reconciliation: %s", response.Status)
	}

	var result ReconcileResult
	if errorValue := json.NewDecoder(response.Body).Decode(&result); errorValue != nil {
		return ReconcileResult{}, errorValue
	}
	return result, nil
}
