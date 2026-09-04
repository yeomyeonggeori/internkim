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

// lock_task_requester reads the mirror before the row exists, so a mirror
// written afterwards is a mirror it never saw.
func TestATaskCarriedOffADeviceSaysSoOnTheWriteThatMakesIt(t *testing.T) {
	stub := newCentralPlaneStub(t)

	if _, errorValue := stub.client().SaveTask(context.Background(), Task{
		DeviceTaskID:    "device-task-1",
		ActorPlatform:   "buzz",
		ActorExternalID: "owner-account",
		Title:           "Carried over",
		Status:          "requested",
	}); errorValue != nil {
		t.Fatal(errorValue)
	}

	mirrors, written := stub.savedArguments["target_mirrors"].([]any)
	if !written || len(mirrors) != 1 {
		t.Fatalf("target_mirrors = %+v", stub.savedArguments["target_mirrors"])
	}
	mirror, named := mirrors[0].(map[string]any)
	if !named || mirror["source"] != DeviceMirrorSource || mirror["externalID"] != "device-task-1" {
		t.Fatalf("mirror = %+v", mirrors[0])
	}
}

func TestATaskAskedForHereCarriesNoDeviceMirror(t *testing.T) {
	stub := newCentralPlaneStub(t)

	if _, errorValue := stub.client().SaveTask(context.Background(), Task{
		ActorPlatform:   "buzz",
		ActorExternalID: "owner-account",
		Title:           "Asked here",
		Status:          "requested",
	}); errorValue != nil {
		t.Fatal(errorValue)
	}

	if stub.savedArguments["target_mirrors"] != nil {
		t.Fatalf("a request made here must still record who asked, got %+v", stub.savedArguments["target_mirrors"])
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
