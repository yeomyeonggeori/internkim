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
	server          *httptest.Server
	membersByEmail  map[string]string
	tasksByDeviceID map[string]string
	savedArguments  map[string]any
	savedAs         string
	deleted         []string
	removedRows     []map[string]any
}

// A delete answers with the rows it removed, so a stub that removes nothing has
// to say so rather than answering empty by accident.
func newCentralPlaneStub(t *testing.T) *centralPlaneStub {
	t.Helper()
	stub := &centralPlaneStub{
		membersByEmail:  map[string]string{},
		tasksByDeviceID: map[string]string{},
		removedRows:     []map[string]any{{"id": "removed"}},
	}
	stub.server = httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if mirrorStubRoutes(stub, writer, request) {
			return
		}
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
		case request.URL.Path == "/rest/v1/rpc/task_save":
			_ = json.NewDecoder(request.Body).Decode(&stub.savedArguments)
			stub.savedAs = request.Header.Get("Authorization")
			writeJSON(writer, "central-task-1")
		case request.Method == http.MethodDelete && request.URL.Path == "/rest/v1/task":
			stub.deleted = append(stub.deleted, request.URL.Query().Get("id"))
			writeJSON(writer, stub.removedRows)
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
		HostCredential: func() string { return "agent-key" },
		ProjectURL:     stub.server.URL,
		PublishableKey: "publishable-key",
	})
}

func writeJSON(writer http.ResponseWriter, value any) {
	writer.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(writer).Encode(value)
}

func TestRemovingATaskNamesItToTheCentralPlane(t *testing.T) {
	stub := newCentralPlaneStub(t)

	if errorValue := stub.client().DeleteTask(context.Background(), "buzz", "owner-account", "central-task-1"); errorValue != nil {
		t.Fatal(errorValue)
	}

	if len(stub.deleted) != 1 || !strings.Contains(stub.deleted[0], "central-task-1") {
		t.Fatalf("deleted = %+v", stub.deleted)
	}
}

// Row level security narrows a delete rather than refusing it, so a caller who
// may not remove the row is answered as though the row went.
func TestATaskNobodyWasAllowedToRemoveIsNotReportedAsRemoved(t *testing.T) {
	stub := newCentralPlaneStub(t)
	stub.removedRows = []map[string]any{}

	errorValue := stub.client().DeleteTask(context.Background(), "buzz", "owner-account", "central-task-1")

	if errorValue == nil {
		t.Fatal("the company removed nothing and said so, and that was taken for success")
	}
}

func TestATaskTheCentralPlaneNeverTookIsNotRemoved(t *testing.T) {
	stub := newCentralPlaneStub(t)

	if errorValue := stub.client().DeleteTask(context.Background(), "buzz", "owner-account", ""); errorValue != nil {
		t.Fatal(errorValue)
	}

	if len(stub.deleted) != 0 {
		t.Fatalf("there is nothing there to remove, yet it asked: %+v", stub.deleted)
	}
}
