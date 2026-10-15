package centralplane

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

const circleChannelPath = "/api/agent/circle-channel"

// CircleChannel is a circle an administrator gave a channel of its own, or one
// whose channel the host opened and has not yet taken down. ChannelName is what
// the company asked for, empty once it asked for none; ChannelID is what the
// host opened, empty until it has.
type CircleChannel struct {
	CircleID     string   `json:"circleID"`
	ChannelName  string   `json:"channelName"`
	ChannelID    string   `json:"channelID"`
	MemberEmails []string `json:"memberEmails"`
}

func (channel CircleChannel) IsWanted() bool {
	return strings.TrimSpace(channel.ChannelName) != ""
}

func (client *Client) CircleChannels(ctx context.Context) ([]CircleChannel, error) {
	var answer struct {
		Circles []CircleChannel `json:"circles"`
	}
	if errorValue := client.readAsAgent(ctx, circleChannelPath, &answer); errorValue != nil {
		return nil, errorValue
	}
	return answer.Circles, nil
}

// RecordCircleChannel tells the record which channel the host opened for a
// circle; an empty channelID records that it has none any more.
func (client *Client) RecordCircleChannel(ctx context.Context, circleID string, channelID string) error {
	if client == nil || !client.settings.Configured() {
		return fmt.Errorf("central plane is not configured")
	}
	payload, errorValue := json.Marshal(map[string]any{"circleID": circleID, "channelID": nullableString(channelID)})
	if errorValue != nil {
		return errorValue
	}
	request, errorValue := http.NewRequestWithContext(ctx, http.MethodPut,
		strings.TrimSuffix(client.settings.AppURL, "/")+circleChannelPath, bytes.NewReader(payload))
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
		return fmt.Errorf("the record refused the channel of circle %s: %s", circleID, response.Status)
	}
	return nil
}
