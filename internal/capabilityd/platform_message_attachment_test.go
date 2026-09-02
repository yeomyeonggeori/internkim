package capabilityd

import (
	"context"
	"testing"

	"gitlab.com/eastriver/internkim/internal/capabilities"
)

func TestPlatformMessageSendRefusesAFileNobodyCarried(t *testing.T) {
	service := Service{Configuration: Configuration{ChatdEndpoint: "http://127.0.0.1:18090", ChatdPlatform: "buzz"}}

	response, errorValue := service.invokePlatformMessageTool(context.Background(), capabilities.ToolInvokeRequest{
		ToolName: "message_send",
		Input: mustJSON(t, map[string]any{
			"targetType":  "channel",
			"channelID":   "channel-1",
			"message":     "hello",
			"attachments": []string{"/workspace/shared/missing.png"},
		}),
		Context: capabilities.ToolInvokeContext{
			RequesterEmail:          "member@example.com",
			RequesterPlatformUserID: "member-1",
			IsApprovalContinuation:  true,
		},
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if response.ErrorCode != "attachment_not_carried" {
		t.Fatalf("expected attachment_not_carried, got %+v", response)
	}
}
