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

	found, errorValue := stub.client().TaskCarrying(context.Background(), "buzz", "owner-account", "ee260bdacb3d")

	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if found != "150a49cd-6b5e-43e4-8731-2e3da76694a3" {
		t.Fatalf("a first edit would otherwise make a second copy, got %q", found)
	}
}

func TestATaskNobodyHasCarriedOverIsNotAFailure(t *testing.T) {
	stub := newCentralPlaneStub(t)

	found, errorValue := stub.client().TaskCarrying(context.Background(), "buzz", "owner-account", "never-carried")

	if errorValue != nil {
		t.Fatalf("nothing there yet is an ordinary answer: %v", errorValue)
	}
	if found != "" {
		t.Fatalf("found = %q", found)
	}
}

func mirrorStubRoutes(stub *centralPlaneStub, writer http.ResponseWriter, request *http.Request) bool {
	if request.Method != http.MethodGet || request.URL.Path != "/rest/v1/task" {
		return false
	}
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
}
