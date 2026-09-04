package capabilityd

import (
	"strings"
	"testing"

	"gitlab.com/eastriver/internkim/internal/capabilities"
	"gitlab.com/eastriver/internkim/pkg/capabilityprotocol"
)

func TestCapabilitySuccessResponseProjectsRegisteredResultContracts(t *testing.T) {
	calendarResult := `{"eventID":"event-1","title":"점검","note":"","location":"","startsAt":"2026-07-24T14:00:00+09:00","endsAt":"2026-07-24T15:00:00+09:00","isWholeDay":false,"participants":[],"notifyMinutesBefore":60,"updatedAt":"2026-07-19T00:00:00Z"}`
	testCases := []struct {
		toolName   string
		status     string
		result     string
		objectType string
		effect     string
		identity   string
	}{
		{toolName: "task_add", status: "created", result: `{"taskID":"task-1"}`, objectType: "task", effect: "created", identity: "task-1"},
		{toolName: "task_update", status: "updated", result: `{"taskID":"task-1"}`, objectType: "task", effect: "updated", identity: "task-1"},
		{toolName: "task_delete", status: "deleted", result: `{"taskID":"task-1","deleted":true}`, objectType: "task", effect: "deleted", identity: "task-1"},
		{toolName: "event_add", status: "created", result: calendarResult, objectType: "calendar", effect: "created", identity: "event-1"},
		{toolName: "event_update", status: "updated", result: calendarResult, objectType: "calendar", effect: "updated", identity: "event-1"},
		{toolName: "event_delete", status: "deleted", result: `{"eventID":"event-1","deleted":true}`, objectType: "calendar", effect: "deleted", identity: "event-1"},
		{toolName: "message_send", status: "sent", result: `{"messageIDs":["message-1"],"deliveryStatus":"sent"}`, objectType: "message", effect: "sent", identity: "message-1"},
		{toolName: "message_update", status: "updated", result: `{"messageID":"message-1","deliveryStatus":"updated","messageUpdated":true}`, objectType: "message", effect: "updated", identity: "message-1"},
		{toolName: "message_delete", status: "deleted", result: `{"messageIDs":["message-1"],"deliveryStatus":"deleted"}`, objectType: "message", effect: "deleted", identity: "message-1"},
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

func TestCapabilitySuccessResponseFromCarriesOriginAndValidates(t *testing.T) {
	result := `{"status":"ok","path":"/workspace/shared/note.md","format":"markdown","content":"# note","warnings":[],"truncated":false}`
	response, errorValue := capabilitySuccessResponseFrom("document_read", "ok", []byte(result), capabilityResponseOrigin{
		Provider:        "internkim-test",
		SelectedBackend: capabilities.LLMBackendRemote,
		Content:         "listed",
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if response.Provider != "internkim-test" || response.SelectedBackend != capabilities.LLMBackendRemote || response.Content != "listed" {
		t.Fatalf("response = %+v", response)
	}
	if response.Outcome != capabilities.ToolOutcomeSucceeded || len(response.Effects) != 0 {
		t.Fatalf("response = %+v", response)
	}
}

func TestCapabilitySuccessResponseFailsClosed(t *testing.T) {
	testCases := []struct {
		name     string
		toolName string
		result   string
		names    []string
	}{
		{name: "missing descriptor", toolName: "task.unknown", result: `{"taskID":"task-1"}`, names: []string{"descriptor is missing", "task.unknown"}},
		{name: "missing field", toolName: "message_send", result: `{"deliveryStatus":"sent"}`, names: []string{"message_send", "result.messageIDs is required and is missing"}},
		{name: "invalid list result", toolName: "site_list", result: `{"sites":null}`, names: []string{"site_list", "result.sites must be an array, and it is null", "the same request with different arguments will not change it"}},
		{name: "invalid list member", toolName: "site_list", result: `{"sites":[{"siteID":"","slug":"brochure","title":"Brochure","status":"draft"}]}`, names: []string{"site_list", `result.sites[0].siteID must be at least 1 character long, and it is ""`}},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			_, errorValue := capabilitySuccessResponse(testCase.toolName, "created", []byte(testCase.result))
			if errorValue == nil {
				t.Fatal("expected the contract to refuse the answer")
			}
			for _, expected := range testCase.names {
				if !strings.Contains(errorValue.Error(), expected) {
					t.Fatalf("expected the refusal to name %q, got %v", expected, errorValue)
				}
			}
		})
	}
}

// answeredBy decides who holds a contract, and a record tool's answer is
// written on the plane against the schema the plane authored. This machine's
// copy of that schema arrives only with an OTA release, so refusing here would
// report a break to the one side that cannot fix it and would take every
// record tool down until the next release. The shape below is the one that
// fails closed for a tool this machine answers, in the case above.
func TestARecordToolsAnswerIsCarriedRatherThanRefused(t *testing.T) {
	response, errorValue := capabilitySuccessResponse("task_list", "ok", []byte(`{"tasks":null,"count":0,"scope":"self"}`))
	if errorValue != nil {
		t.Fatalf("the plane authors this contract, so this side carries the answer: %v", errorValue)
	}
	if response.Outcome != capabilities.ToolOutcomeSucceeded || response.ToolName != "task_list" {
		t.Fatalf("response = %+v", response)
	}
}

// Reading an answer by name is what capabilityd does with it, and that never
// consulted the schema, so the effects a record tool records are unchanged by
// no longer validating its answer here.
func TestARecordToolsEffectsSurviveTheContractGoingUnchecked(t *testing.T) {
	response, errorValue := capabilitySuccessResponse("task_add", "created", []byte(`{"taskID":"task-1","status":"planned","unknownToThisRelease":true}`))
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if len(response.Effects) != 1 || response.Effects[0].ID != "task-1" {
		t.Fatalf("effects = %+v", response.Effects)
	}
}

// The split is the whole point, so it is asserted over the catalog rather than
// on the four tools a fixture happens to name. Every tool this machine answers
// is held to its contract here; every tool the plane answers is carried, and
// the plane holds its own answer through noteWhereTheAnswerLeftItsContract.
func TestWhoHoldsAResultContractFollowsWhoAnswersIt(t *testing.T) {
	held, carried := 0, 0
	for _, descriptor := range capabilityToolDescriptorsByCanonicalName {
		if descriptor.ResultContract == nil {
			continue
		}
		refused := holdResultToContract(descriptor, []byte(`{"__nothing_any_contract_promises":true}`)) != nil
		if descriptor.AnsweredBy == capabilityprotocol.AnsweredByRecord {
			if refused {
				t.Fatalf("%s is answered on the plane, so this side must carry its answer", descriptor.CanonicalName)
			}
			carried++
			continue
		}
		if !refused {
			t.Fatalf("%s is answered on this machine, so this side must hold it to its contract", descriptor.CanonicalName)
		}
		held++
	}
	if held == 0 || carried == 0 {
		t.Fatalf("the catalog no longer has both sides: %d held, %d carried", held, carried)
	}
}
