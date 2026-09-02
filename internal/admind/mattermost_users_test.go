package admind

import (
	"testing"
)

func TestMattermostCircleChannelDefinitionsFromPolicy(t *testing.T) {
	circleChannels := mattermostCircleChannelDefinitionsFromPolicy(map[string]any{
		"circleSync": map[string]any{
			"mattermostPrivateChannels": []any{
				map[string]any{"circleID": "HR", "channelName": "Circle-HR"},
			},
		},
	})

	if len(circleChannels) != 1 {
		t.Fatalf("expected one circle channel, got %+v", circleChannels)
	}
	if circleChannels[0].CircleID != "hr" || circleChannels[0].ChannelName != "circle-hr" {
		t.Fatalf("expected normalized circle channel, got %+v", circleChannels)
	}
}

func containsMattermostTestString(values []string, expectedValue string) bool {
	for _, value := range values {
		if value == expectedValue {
			return true
		}
	}
	return false
}
