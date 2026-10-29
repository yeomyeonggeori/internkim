package capabilityd

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/yeomyeonggeori/internkim/internal/capabilities"
)

func searchChannelByName(t *testing.T, service Service, channelName string) capabilities.ToolInvokeResponse {
	t.Helper()
	input, _ := json.Marshal(map[string]any{"scope": "channel", "channelName": channelName, "queries": []string{"공지"}})
	response, errorValue := service.invokePlatformMessageSearch(context.Background(), capabilities.ToolInvokeRequest{
		ToolName: "message_search",
		Input:    input,
		Context:  capabilities.ToolInvokeContext{Platform: "buzz", RequesterPlatformUserID: "requester-pubkey"},
	})
	if errorValue != nil {
		t.Fatalf("search failed: %v", errorValue)
	}
	return response
}

func TestASearchedChannelNameNothingMatchesOffersTheNearestAndNoneOfThese(t *testing.T) {
	service, _ := channelPostService(t)

	response := searchChannelByName(t, service, "devopz")

	assertPlatformDMStructuredFailure(t, response, "error", "interaction_required", "target_resolution", true, true)
	if !strings.Contains(string(response.Content), "#devops (channel-devops)") || !strings.Contains(string(response.Content), "none of these") {
		t.Fatalf("a near miss names the nearest channels the way message_send does, got %s", response.Content)
	}
}

func TestASearchedChannelNameSeveralChannelsAnswerToAsksWhichByID(t *testing.T) {
	service, _ := channelPostService(t)

	response := searchChannelByName(t, service, "ops")

	assertPlatformDMStructuredFailure(t, response, "error", "interaction_required", "target_resolution", true, true)
	if !strings.Contains(string(response.Content), "channel-ops-a") || !strings.Contains(string(response.Content), "channel-ops-b") {
		t.Fatalf("an ambiguous name names each channel by its ID, got %s", response.Content)
	}
}
