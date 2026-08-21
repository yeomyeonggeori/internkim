package admind

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func recordedTestMapping(t *testing.T, service *Service, externalID string) {
	t.Helper()
	database, errorValue := service.openBridgeMapDatabase(context.Background())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	defer database.Close()
	mapping := bridgeMessageMapping{
		BuzzEventID:       "event-" + externalID,
		Platform:          "mattermost",
		ExternalID:        externalID,
		ExternalChannelID: "channel-1",
	}
	if errorValue := service.recordBridgeMessage(context.Background(), database, mapping); errorValue != nil {
		t.Fatal(errorValue)
	}
}

func mappingIsStillThere(t *testing.T, service *Service, externalID string) bool {
	t.Helper()
	database, errorValue := service.openBridgeMapDatabase(context.Background())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	defer database.Close()
	_, found, errorValue := service.bridgeMessageByExternal(context.Background(), database, "mattermost", externalID)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	return found
}

func forgetRequest(t *testing.T, service *Service, query string) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequest(http.MethodDelete, "http://127.0.0.1:18080/bridge/api/message?"+query, nil)
	request.RemoteAddr = "127.0.0.1:1234"
	response := httptest.NewRecorder()
	service.handleBridgeMap(response, request)
	return response
}

// The mirror retries an edit forever when the mapping names a Buzz event the
// relay no longer has, so it needs a way to say the row is dead.
func TestAMappingCanBeForgottenOnceTheRelayHasLostItsEvent(t *testing.T) {
	service := newBridgeMapTestService(t)
	recordedTestMapping(t, service, "post-1")
	recordedTestMapping(t, service, "post-2")

	response := forgetRequest(t, service, "platform=mattermost&externalId=post-1")

	if response.Code != http.StatusOK {
		t.Fatalf("forgetting answered %d: %s", response.Code, response.Body.String())
	}
	if mappingIsStillThere(t, service, "post-1") {
		t.Error("the dead mapping survived, so the mirror will retry it again")
	}
	if !mappingIsStillThere(t, service, "post-2") {
		t.Error("forgetting one mapping took another with it")
	}
}

func TestForgettingWithoutAPostToNameIsRefused(t *testing.T) {
	service := newBridgeMapTestService(t)
	recordedTestMapping(t, service, "post-1")

	response := forgetRequest(t, service, "platform=mattermost")

	if response.Code != http.StatusBadRequest {
		t.Errorf("answered %d; a delete with no externalId would take the whole platform", response.Code)
	}
	if !mappingIsStillThere(t, service, "post-1") {
		t.Error("the refused delete removed a mapping anyway")
	}
}
