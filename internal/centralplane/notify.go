package centralplane

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

// The plane holds the push subscriptions and the per-member category
// preferences, so a device says who to reach and what happened and lets the
// plane decide whether anything is sent.
type Notification struct {
	Platform    string
	ExternalIDs []string
	Category    string
	Title       string
	Body        string
	OpenPath    string
	Tag         string
}

// Told counts the members whose settings allowed the category through; reached
// counts the devices a push service accepted. Both can be zero on a success.
type NotifyResult struct {
	Told      int `json:"told"`
	Reached   int `json:"reached"`
	Pruned    int `json:"pruned"`
	Addressed int `json:"addressed"`
}

func (client *Client) Notify(ctx context.Context, notification Notification) (NotifyResult, error) {
	payload, errorValue := json.Marshal(map[string]any{
		"platform":    notification.Platform,
		"externalIDs": notification.ExternalIDs,
		"category":    notification.Category,
		"title":       notification.Title,
		"body":        notification.Body,
		"openPath":    notification.OpenPath,
		"tag":         notification.Tag,
	})
	if errorValue != nil {
		return NotifyResult{}, errorValue
	}

	request, errorValue := http.NewRequestWithContext(ctx, http.MethodPost,
		strings.TrimSuffix(client.settings.AppURL, "/")+"/api/agent/notify", bytes.NewReader(payload))
	if errorValue != nil {
		return NotifyResult{}, errorValue
	}
	request.Header.Set("Authorization", "Bearer "+client.settings.AgentAPIKey)
	request.Header.Set("Content-Type", "application/json")

	response, errorValue := client.httpClient.Do(request)
	if errorValue != nil {
		return NotifyResult{}, errorValue
	}
	defer response.Body.Close()
	if response.StatusCode >= 300 {
		return NotifyResult{}, fmt.Errorf("central plane refused the notification: %s", response.Status)
	}

	var result NotifyResult
	if errorValue := json.NewDecoder(response.Body).Decode(&result); errorValue != nil {
		return NotifyResult{}, errorValue
	}
	return result, nil
}
