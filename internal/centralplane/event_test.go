package centralplane

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestACalendarEventCarriesItsPeopleAsAddresses(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path == "/api/agent/session" {
			writeJSON(writer, map[string]any{"memberID": "member-1", "accessToken": "token-1", "expiresAt": 4102444800})
			return
		}
		writeJSON(writer, []map[string]any{{
			"id":           "task-1",
			"title":        "포틀랜드 출장",
			"note":         "",
			"location":     map[string]any{"name": "Portland"},
			"starts_at":    "2026-08-24T00:00:00+00:00",
			"ends_at":      "2026-08-28T00:00:00+00:00",
			"is_whole_day": true,
			"updated_at":   "2026-08-24T10:00:00+00:00",
			"task_participant": []map[string]any{
				{"member": map[string]any{"email": "kimyesi@example.com"}},
			},
		}})
	}))
	defer server.Close()
	client := New(Settings{AppURL: server.URL, AgentAPIKey: "agent-key", ProjectURL: server.URL, PublishableKey: "publishable-key"})

	event, found, errorValue := client.EventByID(context.Background(), "buzz", "owner-account", "task-1")
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if !found {
		t.Fatal("expected the one event the company holds")
	}
	if event.Title != "포틀랜드 출장" || event.Location != "Portland" || !event.IsWholeDay {
		t.Fatalf("event = %+v", event)
	}
	if len(event.ParticipantMails) != 1 || event.ParticipantMails[0] != "kimyesi@example.com" {
		t.Fatalf("a device turns addresses into its own identifiers, got %+v", event.ParticipantMails)
	}
}

func TestSavingAnEventNamesTheProcedureTheCompanyGuards(t *testing.T) {
	procedure := ""
	var arguments map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path == "/api/agent/session" {
			writeJSON(writer, map[string]any{"memberID": "member-1", "accessToken": "token-1", "expiresAt": 4102444800})
			return
		}
		if strings.HasPrefix(request.URL.Path, "/rest/v1/rpc/") {
			procedure = strings.TrimPrefix(request.URL.Path, "/rest/v1/rpc/")
			_ = json.NewDecoder(request.Body).Decode(&arguments)
			writeJSON(writer, "task-9")
			return
		}
		writeJSON(writer, []any{})
	}))
	defer server.Close()
	client := New(Settings{AppURL: server.URL, AgentAPIKey: "agent-key", ProjectURL: server.URL, PublishableKey: "publishable-key"})

	savedID, errorValue := client.SaveEvent(context.Background(), "email", "kimyesi@example.com", Event{
		Title:      "포틀랜드 출장",
		Location:   "Portland",
		StartsAt:   "2026-08-24T00:00:00Z",
		EndsAt:     "2026-08-28T00:00:00Z",
		IsWholeDay: true,
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if savedID != "task-9" {
		t.Fatalf("savedID = %q", savedID)
	}
	if procedure != "task_save" {
		t.Fatalf("the company guards its calendar behind that procedure, got %q", procedure)
	}
	if arguments["target_task_id"] != nil {
		t.Fatalf("a new event names no task, got %v", arguments["target_task_id"])
	}
	location, isObject := arguments["target_location"].(map[string]any)
	if !isObject || location["name"] != "Portland" {
		t.Fatalf("a place is written as the object the column holds, got %v", arguments["target_location"])
	}
}

func TestAnEventWithNoPlaceWritesNoPlace(t *testing.T) {
	if nullableLocation("   ") != nil {
		t.Fatal("an empty place is no place, not an object with an empty name")
	}
}

func TestAWriteCarriesTheVersionItRead(t *testing.T) {
	var arguments map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path == "/api/agent/session" {
			writeJSON(writer, map[string]any{"memberID": "member-1", "accessToken": "token-1", "expiresAt": 4102444800})
			return
		}
		if strings.HasPrefix(request.URL.Path, "/rest/v1/rpc/") {
			_ = json.NewDecoder(request.Body).Decode(&arguments)
			writeJSON(writer, "task-9")
			return
		}
		writeJSON(writer, []any{})
	}))
	defer server.Close()
	client := New(Settings{AppURL: server.URL, AgentAPIKey: "agent-key", ProjectURL: server.URL, PublishableKey: "publishable-key"})

	if _, errorValue := client.SaveEvent(context.Background(), "email", "kimyesi@example.com", Event{
		CentralID:         "task-9",
		Title:             "포틀랜드 출장",
		StartsAt:          "2026-08-24T00:00:00Z",
		EndsAt:            "2026-08-28T00:00:00Z",
		ExpectedUpdatedAt: "2026-08-24T10:00:00Z",
	}); errorValue != nil {
		t.Fatal(errorValue)
	}
	if arguments["target_expected_updated_at"] != "2026-08-24T10:00:00Z" {
		t.Fatalf("the company refuses a write whose version is gone, so it has to be told, got %v", arguments["target_expected_updated_at"])
	}

	arguments = nil
	if _, errorValue := client.SaveEvent(context.Background(), "email", "kimyesi@example.com", Event{
		Title:    "새 일정",
		StartsAt: "2026-08-24T00:00:00Z",
		EndsAt:   "2026-08-28T00:00:00Z",
	}); errorValue != nil {
		t.Fatal(errorValue)
	}
	if arguments["target_expected_updated_at"] != nil {
		t.Fatalf("a new event replaces no version, got %v", arguments["target_expected_updated_at"])
	}
}

func TestANewEventNamesNoTaskAndAKnownOneNamesItsOwn(t *testing.T) {
	var arguments map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path == "/api/agent/session" {
			writeJSON(writer, map[string]any{"memberID": "member-1", "accessToken": "token-1", "expiresAt": 4102444800})
			return
		}
		if strings.HasPrefix(request.URL.Path, "/rest/v1/rpc/") {
			_ = json.NewDecoder(request.Body).Decode(&arguments)
			writeJSON(writer, "task-9")
			return
		}
		writeJSON(writer, []any{})
	}))
	defer server.Close()
	client := New(Settings{AppURL: server.URL, AgentAPIKey: "agent-key", ProjectURL: server.URL, PublishableKey: "publishable-key"})

	if _, errorValue := client.SaveEvent(context.Background(), "email", "kimyesi@example.com", Event{
		Title: "새 일정", StartsAt: "2026-09-30T01:00:00Z", EndsAt: "2026-09-30T02:00:00Z",
	}); errorValue != nil {
		t.Fatal(errorValue)
	}
	if arguments["target_task_id"] != nil {
		t.Fatalf("an event the company does not hold names no task of theirs, got %v", arguments["target_task_id"])
	}

	if _, errorValue := client.SaveEvent(context.Background(), "email", "kimyesi@example.com", Event{
		CentralID: "b5f0832c-5193-4f1c-88fd-9213f257229f", Title: "고친 일정",
		StartsAt: "2026-09-30T01:00:00Z", EndsAt: "2026-09-30T02:00:00Z",
	}); errorValue != nil {
		t.Fatal(errorValue)
	}
	if arguments["target_task_id"] != "b5f0832c-5193-4f1c-88fd-9213f257229f" {
		t.Fatalf("an event the company holds names it, got %v", arguments["target_task_id"])
	}
}
