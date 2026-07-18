package capabilityprotocol

import (
	"encoding/json"
	"os"
	"testing"
)

func TestBlueclawProtocolCapabilityFixturesMatchGoContracts(t *testing.T) {
	var descriptor Descriptor
	readProtocolFixture(t, "capability-descriptor", &descriptor)
	if descriptor.Name != "calendar.add" || len(descriptor.InputSchema) == 0 {
		t.Fatalf("unexpected capability descriptor fixture: %#v", descriptor)
	}

	var request ToolInvokeRequest
	readProtocolFixture(t, "tool-invoke-request", &request)
	if request.ToolName != descriptor.Name || request.IdempotencyKey == "" {
		t.Fatalf("unexpected tool invoke fixture: %#v", request)
	}
	if request.Actor.PersonID != request.Context.RequesterPersonID {
		t.Fatalf("tool invoke fixture actor does not match requester: %#v", request)
	}

	var response ToolInvokeResponse
	readProtocolFixture(t, "tool-invoke-response", &response)
	if response.ToolName != descriptor.Name || response.Outcome != ToolOutcomeSucceeded || len(response.Effects) != 1 || response.Effects[0].ID != "event-1" {
		t.Fatalf("unexpected tool invoke response fixture: %#v", response)
	}
}

func readProtocolFixture(t *testing.T, fixtureName string, destination any) {
	t.Helper()
	documentBytes, errorValue := os.ReadFile("../../.dependency/blueclaw/protocol/fixtures/valid.json")
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	var fixtures map[string][]json.RawMessage
	if errorValue := json.Unmarshal(documentBytes, &fixtures); errorValue != nil {
		t.Fatal(errorValue)
	}
	if len(fixtures[fixtureName]) != 1 {
		t.Fatalf("expected one %s fixture", fixtureName)
	}
	if errorValue := json.Unmarshal(fixtures[fixtureName][0], destination); errorValue != nil {
		t.Fatal(errorValue)
	}
}
