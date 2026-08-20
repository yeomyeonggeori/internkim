package centralplane

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

func TestATaskAlreadyCarriedOverIsFoundByItsDeviceIdentifier(t *testing.T) {
	stub := newCentralPlaneStub(t)
	stub.tasksByDeviceID["ee260bdacb3d"] = "150a49cd-6b5e-43e4-8731-2e3da76694a3"

	found, errorValue := stub.client().TaskCarrying(context.Background(), "mattermost", "owner-account", "ee260bdacb3d")

	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if found != "150a49cd-6b5e-43e4-8731-2e3da76694a3" {
		t.Fatalf("a first edit would otherwise make a second copy, got %q", found)
	}
}

func TestATaskNobodyHasCarriedOverIsNotAFailure(t *testing.T) {
	stub := newCentralPlaneStub(t)

	found, errorValue := stub.client().TaskCarrying(context.Background(), "mattermost", "owner-account", "never-carried")

	if errorValue != nil {
		t.Fatalf("nothing there yet is an ordinary answer: %v", errorValue)
	}
	if found != "" {
		t.Fatalf("found = %q", found)
	}
}

func TestATaskTheCentralPlaneMadeRecordsWhereItCameFrom(t *testing.T) {
	stub := newCentralPlaneStub(t)

	if errorValue := stub.client().MarkCarriedFrom(context.Background(), "mattermost", "owner-account",
		"central-task-1", "device-task-1"); errorValue != nil {
		t.Fatal(errorValue)
	}

	carried := stub.patched["central-task-1"]
	if !strings.Contains(carried, DeviceMirrorSource) || !strings.Contains(carried, "device-task-1") {
		t.Fatalf("without this the link dies with the device's own record, got %q", carried)
	}
}

func mirrorStubRoutes(stub *centralPlaneStub, writer http.ResponseWriter, request *http.Request) bool {
	switch {
	case request.Method == http.MethodGet && request.URL.Path == "/rest/v1/task":
		var asked struct {
			Mirrors []struct {
				ExternalID string `json:"externalID"`
			} `json:"mirrors"`
		}
		_ = json.Unmarshal([]byte(strings.TrimPrefix(request.URL.Query().Get("calendar"), "cs.")), &asked)
		if len(asked.Mirrors) == 0 {
			writeJSON(writer, []any{})
			return true
		}
		centralID, carried := stub.tasksByDeviceID[asked.Mirrors[0].ExternalID]
		if !carried {
			writeJSON(writer, []any{})
			return true
		}
		writeJSON(writer, []map[string]string{{"id": centralID}})
		return true
	case request.Method == http.MethodPatch && request.URL.Path == "/rest/v1/task":
		body := make([]byte, request.ContentLength)
		_, _ = request.Body.Read(body)
		stub.patched[strings.TrimPrefix(request.URL.Query().Get("id"), "eq.")] = string(body)
		writer.WriteHeader(http.StatusNoContent)
		return true
	}
	return false
}
