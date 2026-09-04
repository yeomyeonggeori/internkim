package centralplane

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

// The device mints its own identifier and it rides across in the same place an
// event's identities ride, so a task already carried over can be found again
// without the device having been told what it was called.
const DeviceMirrorSource = "internkim-device"

func deviceMirrors(deviceTaskID string) []map[string]string {
	if strings.TrimSpace(deviceTaskID) == "" {
		return nil
	}
	return []map[string]string{{"source": DeviceMirrorSource, "externalID": deviceTaskID}}
}

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
	carried, errorValue := json.Marshal(map[string]any{"mirrors": deviceMirrors(deviceTaskID)})
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
