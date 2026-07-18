package capabilityd

import (
	"strings"
	"testing"

	"gitlab.com/eastriver/internkim/internal/capabilities"
)

func TestCapabilitySuccessResponseProjectsRegisteredResultContracts(t *testing.T) {
	calendarResult := `{"eventID":"event-1","title":"점검","description":"","location":"","startISO":"2026-07-24T14:00:00+09:00","endISO":"2026-07-24T15:00:00+09:00","timeZone":"Asia/Seoul","isAllDay":false,"color":"","people":[],"participants":[],"reminderLeadHours":1,"updatedAt":"2026-07-19T00:00:00Z"}`
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
		{toolName: "task.delete", status: "deleted", result: `{"taskID":"task-1","deleted":true}`, objectType: "task", effect: "deleted", identity: "task-1"},
		{toolName: "calendar.add", status: "created", result: calendarResult, objectType: "calendar", effect: "created", identity: "event-1"},
		{toolName: "calendar.update", status: "updated", result: calendarResult, objectType: "calendar", effect: "updated", identity: "event-1"},
		{toolName: "calendar.delete", status: "deleted", result: `{"eventID":"event-1","deleted":true}`, objectType: "calendar", effect: "deleted", identity: "event-1"},
		{toolName: "message.send", status: "sent", result: `{"messageIDs":["message-1"],"deliveryStatus":"sent"}`, objectType: "message", effect: "sent", identity: "message-1"},
		{toolName: "message.update", status: "updated", result: `{"messageID":"message-1","deliveryStatus":"updated","messageUpdated":true}`, objectType: "message", effect: "updated", identity: "message-1"},
		{toolName: "message.delete", status: "deleted", result: `{"messageIDs":["message-1"],"deliveryStatus":"deleted"}`, objectType: "message", effect: "deleted", identity: "message-1"},
		{toolName: "channel.update", status: "updated", result: `{"channelID":"channel-1","updated":true}`, objectType: "channel", effect: "updated", identity: "channel-1"},
	}
	for _, testCase := range testCases {
		t.Run(testCase.toolName, func(t *testing.T) {
			response, errorValue := capabilitySuccessResponse(testCase.toolName, testCase.status, []byte(testCase.result))
			if errorValue != nil {
				t.Fatal(errorValue)
			}
			if response.ToolName != testCase.toolName || response.Outcome != capabilities.ToolOutcomeSucceeded || response.Status != testCase.status || len(response.Effects) != 1 {
				t.Fatalf("response = %+v", response)
			}
			effect := response.Effects[0]
			if effect.ObjectType != testCase.objectType || effect.Effect != testCase.effect || effect.ID != testCase.identity {
				t.Fatalf("effect = %+v", effect)
			}
		})
	}
}

func TestCapabilitySuccessResponseFailsClosed(t *testing.T) {
	testCases := []struct {
		name       string
		toolName   string
		result     string
		errorMatch string
	}{
		{name: "missing descriptor", toolName: "task.unknown", result: `{"taskID":"task-1"}`, errorMatch: "descriptor is missing"},
		{name: "invalid result", toolName: "task.add", result: `{"status":"created"}`, errorMatch: "violates task.add contract"},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			_, errorValue := capabilitySuccessResponse(testCase.toolName, "created", []byte(testCase.result))
			if errorValue == nil || !strings.Contains(errorValue.Error(), testCase.errorMatch) {
				t.Fatalf("error = %v", errorValue)
			}
		})
	}
}
