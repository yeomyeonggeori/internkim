package capabilities

import (
	"encoding/json"
	"testing"
)

func TestToolInvokeRequestRoundTrip(t *testing.T) {
	request := ToolInvokeRequest{
		ToolName:             "browser.open",
		Input:                json.RawMessage(`{"url":"https://example.com"}`),
		ExecutionMode:        ExecutionModeCompanion,
		RequiresUserPresence: true,
		PrivacyClass:         "user_browser",
		SessionID:            "session-1",
		TimeoutSecond:        30,
	}

	document, errorValue := json.Marshal(request)
	if errorValue != nil {
		t.Fatal(errorValue)
	}

	var decodedRequest ToolInvokeRequest
	if errorValue := json.Unmarshal(document, &decodedRequest); errorValue != nil {
		t.Fatal(errorValue)
	}

	if decodedRequest.ToolName != request.ToolName {
		t.Fatalf("expected tool name to round trip, got %q", decodedRequest.ToolName)
	}
	if decodedRequest.SessionID != request.SessionID {
		t.Fatalf("expected session id to round trip, got %q", decodedRequest.SessionID)
	}
	if string(decodedRequest.Input) != string(request.Input) {
		t.Fatalf("expected input to round trip, got %s", decodedRequest.Input)
	}
}

func TestCompanionToolNamesComeFromDescriptors(t *testing.T) {
	descriptors := CompanionToolDescriptors()
	toolNames := CompanionToolNames()

	if len(descriptors) != len(toolNames) {
		t.Fatalf("expected descriptor and tool name counts to match")
	}
	for index, descriptor := range descriptors {
		if toolNames[index] != descriptor.Name {
			t.Fatalf("expected tool name %q, got %q", descriptor.Name, toolNames[index])
		}
	}
}
