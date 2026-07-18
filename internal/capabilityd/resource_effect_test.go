package capabilityd

import (
	"strings"
	"testing"
)

func TestCapabilityMutationResponseProjectsRegisteredResultContracts(t *testing.T) {
	testCases := []struct {
		toolName   string
		status     string
		result     string
		objectType string
		effect     string
		identity   string
	}{
		{toolName: "task.add", status: "created", result: `{"taskID":"task-1"}`, objectType: "task", effect: "created", identity: "task-1"},
		{toolName: "task.update", status: "updated", result: `{"taskID":"task-1"}`, objectType: "task", effect: "updated", identity: "task-1"},
		{toolName: "task.delete", status: "deleted", result: `{"taskID":"task-1"}`, objectType: "task", effect: "deleted", identity: "task-1"},
		{toolName: "calendar.add", status: "created", result: `{"eventID":"event-1"}`, objectType: "calendar", effect: "created", identity: "event-1"},
		{toolName: "calendar.update", status: "updated", result: `{"eventID":"event-1"}`, objectType: "calendar", effect: "updated", identity: "event-1"},
		{toolName: "calendar.delete", status: "deleted", result: `{"eventID":"event-1"}`, objectType: "calendar", effect: "deleted", identity: "event-1"},
	}
	for _, testCase := range testCases {
		t.Run(testCase.toolName, func(t *testing.T) {
			response, errorValue := capabilityMutationResponse(testCase.toolName, testCase.status, []byte(testCase.result))
			if errorValue != nil {
				t.Fatal(errorValue)
			}
			if response.ToolName != testCase.toolName || response.Status != testCase.status || len(response.Effects) != 1 {
				t.Fatalf("response = %+v", response)
			}
			effect := response.Effects[0]
			if effect.ObjectType != testCase.objectType || effect.Effect != testCase.effect || effect.ID != testCase.identity {
				t.Fatalf("effect = %+v", effect)
			}
		})
	}
}

func TestCapabilityMutationResponseFailsClosed(t *testing.T) {
	testCases := []struct {
		name       string
		toolName   string
		result     string
		errorMatch string
	}{
		{name: "missing descriptor", toolName: "task.unknown", result: `{"taskID":"task-1"}`, errorMatch: "descriptor is missing"},
		{name: "missing identity", toolName: "task.add", result: `{"status":"created"}`, errorMatch: "identity is missing"},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			_, errorValue := capabilityMutationResponse(testCase.toolName, "created", []byte(testCase.result))
			if errorValue == nil || !strings.Contains(errorValue.Error(), testCase.errorMatch) {
				t.Fatalf("error = %v", errorValue)
			}
		})
	}
}
