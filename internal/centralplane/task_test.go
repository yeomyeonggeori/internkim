package centralplane

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type centralPlaneStub struct {
	server         *httptest.Server
	membersByEmail map[string]string
	savedArguments map[string]any
	savedAs        string
	deleted        []string
}

func newCentralPlaneStub(t *testing.T) *centralPlaneStub {
	t.Helper()
	stub := &centralPlaneStub{membersByEmail: map[string]string{}}
	stub.server = httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch {
		case request.URL.Path == "/api/agent/session":
			var asked struct{ ExternalID string }
			_ = json.NewDecoder(request.Body).Decode(&asked)
			writeJSON(writer, map[string]any{
				"memberID":    "member-for-" + asked.ExternalID,
				"accessToken": "token-for-" + asked.ExternalID,
				"expiresAt":   4102444800,
			})
		case request.URL.Path == "/api/agent/member":
			memberID, known := stub.membersByEmail[request.URL.Query().Get("email")]
			if !known {
				writeJSON(writer, map[string]any{"member": nil})
				return
			}
			writeJSON(writer, map[string]any{"member": map[string]string{"memberID": memberID}})
		case request.URL.Path == "/rest/v1/rpc/save_flow_task":
			_ = json.NewDecoder(request.Body).Decode(&stub.savedArguments)
			stub.savedAs = request.Header.Get("Authorization")
			writeJSON(writer, "central-task-1")
		case request.Method == http.MethodDelete && request.URL.Path == "/rest/v1/task":
			stub.deleted = append(stub.deleted, request.URL.Query().Get("id"))
			writer.WriteHeader(http.StatusNoContent)
		default:
			http.Error(writer, "unexpected "+request.Method+" "+request.URL.Path, http.StatusTeapot)
		}
	}))
	t.Cleanup(stub.server.Close)
	return stub
}

func (stub *centralPlaneStub) client() *Client {
	return New(Settings{
		AppURL:         stub.server.URL,
		AgentAPIKey:    "agent-key",
		ProjectURL:     stub.server.URL,
		PublishableKey: "publishable-key",
	})
}

func writeJSON(writer http.ResponseWriter, value any) {
	writer.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(writer).Encode(value)
}

func TestATaskIsWrittenAsThePersonItBelongsTo(t *testing.T) {
	stub := newCentralPlaneStub(t)
	stub.membersByEmail["colleague@example.test"] = "member-colleague"

	savedID, errorValue := stub.client().SaveTask(context.Background(), Task{
		ActorPlatform:    "mattermost",
		ActorExternalID:  "owner-account",
		Title:            "Write the report",
		Status:           "todo",
		ParticipantMails: []string{"colleague@example.test"},
	})

	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if savedID != "central-task-1" {
		t.Fatalf("the central plane's own identifier has to come back, got %q", savedID)
	}
	if stub.savedAs != "Bearer token-for-owner-account" {
		t.Fatalf("save_flow_task is granted to authenticated alone, so it must be called as the member: %q", stub.savedAs)
	}
}

func TestTheOwnerIsAParticipantOfTheirOwnTask(t *testing.T) {
	stub := newCentralPlaneStub(t)
	stub.membersByEmail["colleague@example.test"] = "member-colleague"

	if _, errorValue := stub.client().SaveTask(context.Background(), Task{
		ActorPlatform:    "mattermost",
		ActorExternalID:  "owner-account",
		Title:            "Write the report",
		ParticipantMails: []string{"colleague@example.test"},
	}); errorValue != nil {
		t.Fatal(errorValue)
	}

	participants := stub.savedArguments["target_participant_ids"].([]any)
	if len(participants) != 2 || participants[0] != "member-for-owner-account" {
		t.Fatalf("the owner leads the participants, got %+v", participants)
	}
}

func TestSomebodyTheCompanyDoesNotKnowIsLeftOutRatherThanFailingTheWrite(t *testing.T) {
	stub := newCentralPlaneStub(t)

	if _, errorValue := stub.client().SaveTask(context.Background(), Task{
		ActorPlatform:    "mattermost",
		ActorExternalID:  "owner-account",
		Title:            "Write the report",
		ParticipantMails: []string{"nobody@example.test"},
	}); errorValue != nil {
		t.Fatalf("an uninvited participant is an ordinary answer, not a failure: %v", errorValue)
	}

	participants := stub.savedArguments["target_participant_ids"].([]any)
	if len(participants) != 1 {
		t.Fatalf("only the owner is a member here, got %+v", participants)
	}
}

func TestATaskTheCentralPlaneAlreadyHoldsIsUpdatedRatherThanRemade(t *testing.T) {
	stub := newCentralPlaneStub(t)

	if _, errorValue := stub.client().SaveTask(context.Background(), Task{
		CentralID:       "central-task-1",
		ActorPlatform:   "mattermost",
		ActorExternalID: "owner-account",
		Title:           "Write the report again",
	}); errorValue != nil {
		t.Fatal(errorValue)
	}

	if stub.savedArguments["target_task_id"] != "central-task-1" {
		t.Fatalf("without the identifier every drain would make a second copy, got %+v", stub.savedArguments["target_task_id"])
	}
}

func TestADateNobodySetIsSentAsNothing(t *testing.T) {
	stub := newCentralPlaneStub(t)

	if _, errorValue := stub.client().SaveTask(context.Background(), Task{
		ActorPlatform:   "mattermost",
		ActorExternalID: "owner-account",
		Title:           "Write the report",
	}); errorValue != nil {
		t.Fatal(errorValue)
	}

	if stub.savedArguments["target_starts_at"] != nil || stub.savedArguments["target_task_id"] != nil {
		t.Fatalf("an empty string is a value and would be stored as one, got %+v", stub.savedArguments)
	}
}

func TestRemovingATaskNamesItToTheCentralPlane(t *testing.T) {
	stub := newCentralPlaneStub(t)

	if errorValue := stub.client().DeleteTask(context.Background(), "mattermost", "owner-account", "central-task-1"); errorValue != nil {
		t.Fatal(errorValue)
	}

	if len(stub.deleted) != 1 || !strings.Contains(stub.deleted[0], "central-task-1") {
		t.Fatalf("deleted = %+v", stub.deleted)
	}
}

func TestATaskTheCentralPlaneNeverTookIsNotRemoved(t *testing.T) {
	stub := newCentralPlaneStub(t)

	if errorValue := stub.client().DeleteTask(context.Background(), "mattermost", "owner-account", ""); errorValue != nil {
		t.Fatal(errorValue)
	}

	if len(stub.deleted) != 0 {
		t.Fatalf("there is nothing there to remove, yet it asked: %+v", stub.deleted)
	}
}
