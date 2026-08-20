package centralplane

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

// The device mints its own identifier and it rides across in the same place an
// event's identities ride, so a task already carried over can be found again
// without the device having been told what it was called. The relay writes the
// same source name in host/relay/flow-task-as-task.ts.
const DeviceMirrorSource = "internkim-device"

// TaskCarrying answers which task already carries that device identifier, or an
// empty string when none does. A task nobody has carried over yet is an ordinary
// answer, not a failure.
func (client *Client) TaskCarrying(ctx context.Context, platform string, externalID string, deviceTaskID string) (string, error) {
	if strings.TrimSpace(deviceTaskID) == "" {
		return "", nil
	}
	session, errorValue := client.sessionFor(ctx, platform, externalID)
	if errorValue != nil {
		return "", errorValue
	}
	carried, errorValue := json.Marshal(map[string]any{
		"mirrors": []map[string]string{{"source": DeviceMirrorSource, "externalID": deviceTaskID}},
	})
	if errorValue != nil {
		return "", errorValue
	}
	query := url.Values{}
	query.Set("select", "id")
	query.Set("calendar", "cs."+string(carried))
	query.Set("limit", "1")

	request, errorValue := http.NewRequestWithContext(ctx, http.MethodGet,
		strings.TrimSuffix(client.settings.ProjectURL, "/")+"/rest/v1/task?"+query.Encode(), nil)
	if errorValue != nil {
		return "", errorValue
	}
	client.signAsMember(request, session)

	response, errorValue := client.httpClient.Do(request)
	if errorValue != nil {
		return "", errorValue
	}
	defer response.Body.Close()
	if response.StatusCode >= 300 {
		return "", fmt.Errorf("central plane refused a search for device task %s: %s", deviceTaskID, response.Status)
	}
	var found []struct {
		ID string `json:"id"`
	}
	if errorValue := json.NewDecoder(response.Body).Decode(&found); errorValue != nil {
		return "", errorValue
	}
	if len(found) == 0 {
		return "", nil
	}
	return found[0].ID, nil
}

// MarkCarriedFrom writes the device identifier onto a task the central plane made,
// so the link survives the device losing what it remembered.
func (client *Client) MarkCarriedFrom(ctx context.Context, platform string, externalID string, centralID string, deviceTaskID string) error {
	if strings.TrimSpace(centralID) == "" || strings.TrimSpace(deviceTaskID) == "" {
		return nil
	}
	session, errorValue := client.sessionFor(ctx, platform, externalID)
	if errorValue != nil {
		return errorValue
	}
	payload, errorValue := json.Marshal(map[string]any{
		"calendar": map[string]any{
			"mirrors": []map[string]string{{"source": DeviceMirrorSource, "externalID": deviceTaskID}},
		},
	})
	if errorValue != nil {
		return errorValue
	}
	request, errorValue := http.NewRequestWithContext(ctx, http.MethodPatch,
		strings.TrimSuffix(client.settings.ProjectURL, "/")+"/rest/v1/task?id=eq."+url.QueryEscape(centralID),
		bytes.NewReader(payload))
	if errorValue != nil {
		return errorValue
	}
	client.signAsMember(request, session)
	request.Header.Set("Content-Type", "application/json")

	response, errorValue := client.httpClient.Do(request)
	if errorValue != nil {
		return errorValue
	}
	defer response.Body.Close()
	if response.StatusCode >= 300 {
		return fmt.Errorf("central plane refused to record task %s as carried from %s: %s", centralID, deviceTaskID, response.Status)
	}
	return nil
}
