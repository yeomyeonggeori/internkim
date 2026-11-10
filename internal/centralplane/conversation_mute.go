package centralplane

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

const conversationMutePath = "/api/agent/conversation-mute"

func (client *Client) MuteConversation(ctx context.Context, memberID string, conversationID string) error {
	if client == nil || !client.settings.Configured() {
		return fmt.Errorf("central plane is not configured")
	}
	payload, errorValue := json.Marshal(map[string]string{"memberID": memberID, "conversationID": conversationID})
	if errorValue != nil {
		return errorValue
	}
	request, errorValue := http.NewRequestWithContext(ctx, http.MethodPost,
		strings.TrimSuffix(client.settings.AppURL, "/")+conversationMutePath, bytes.NewReader(payload))
	if errorValue != nil {
		return errorValue
	}
	request.Header.Set("Authorization", "Bearer "+client.settings.HostCredential())
	request.Header.Set("Content-Type", "application/json")

	response, errorValue := client.httpClient.Do(request)
	if errorValue != nil {
		return errorValue
	}
	defer response.Body.Close()
	if response.StatusCode >= 300 {
		return fmt.Errorf("the record refused to mute conversation %s for member %s: %s", conversationID, memberID, response.Status)
	}
	return nil
}
