package admind

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	companionruntime "gitlab.com/eastriver/internkim/internal/companion"
)

func companionMemberTestService(t *testing.T) http.Handler {
	t.Helper()
	service := NewService(Configuration{StateDirectory: t.TempDir(), AdminEmailPath: writeTestFile(t, "admin@example.com")})
	return service.router()
}

func pairCompanionOwnedBy(t *testing.T, handler http.Handler, ownerEmail string, ownerPersonID string) string {
	t.Helper()
	pairingResponse := httptest.NewRecorder()
	pairingRequest := httptest.NewRequest(http.MethodPost, "/_internkim/companion/pairing-codes", strings.NewReader(`{"ownerEmail":"`+ownerEmail+`","ownerPersonID":"`+ownerPersonID+`"}`))
	pairingRequest.RemoteAddr = "127.0.0.1:1234"
	handler.ServeHTTP(pairingResponse, pairingRequest)
	if pairingResponse.Code != http.StatusOK {
		t.Fatalf("pairing code status = %d body = %s", pairingResponse.Code, pairingResponse.Body.String())
	}
	var pairingCode companionPairingCodeResponse
	if errorValue := json.Unmarshal(pairingResponse.Body.Bytes(), &pairingCode); errorValue != nil {
		t.Fatal(errorValue)
	}
	keyPair, errorValue := companionruntime.GenerateKeyPair()
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	pairResponse := httptest.NewRecorder()
	pairRequest := httptest.NewRequest(http.MethodPost, "/_internkim/companion/pair", strings.NewReader(`{
		"code":"`+pairingCode.Code+`",
		"displayName":"`+ownerEmail+`'s computer",
		"publicKey":"`+keyPair.PublicKey+`",
		"capabilities":[{"name":"computer_task","version":"1","privacyClass":"user_browser","estimatedLatency":"interactive","requiresUserPresence":true}]
	}`))
	handler.ServeHTTP(pairResponse, pairRequest)
	if pairResponse.Code != http.StatusOK {
		t.Fatalf("pair status = %d body = %s", pairResponse.Code, pairResponse.Body.String())
	}
	var paired companionPairResponse
	if errorValue := json.Unmarshal(pairResponse.Body.Bytes(), &paired); errorValue != nil {
		t.Fatal(errorValue)
	}
	return paired.CompanionID
}

func memberCompanions(t *testing.T, handler http.Handler) []CompanionStatus {
	t.Helper()
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/companion/api/mine", nil))
	if response.Code != http.StatusOK {
		t.Fatalf("mine status = %d body = %s", response.Code, response.Body.String())
	}
	var listed companionStatusResponse
	if errorValue := json.Unmarshal(response.Body.Bytes(), &listed); errorValue != nil {
		t.Fatal(errorValue)
	}
	return listed.Companions
}

func TestMemberSeesOnlyTheCompanionsPairedToThem(t *testing.T) {
	handler := companionMemberTestService(t)
	mine := pairCompanionOwnedBy(t, handler, "alice@example.com", "person-alice")
	pairCompanionOwnedBy(t, handler, "bob@example.com", "person-bob")
	stubPersonaActor(t, personaActor{email: "Alice@Example.com", personID: "person-alice"}, true)

	listed := memberCompanions(t, handler)
	if len(listed) != 1 || listed[0].CompanionID != mine {
		t.Fatalf("companions = %+v, expected only %s", listed, mine)
	}
}

func TestMemberPairingCodeIsAddressedToTheSignedInMember(t *testing.T) {
	handler := companionMemberTestService(t)
	stubPersonaActor(t, personaActor{email: "alice@example.com", personID: "person-alice"}, true)

	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/companion/api/pairing-codes", nil))
	if response.Code != http.StatusOK {
		t.Fatalf("pairing code status = %d body = %s", response.Code, response.Body.String())
	}
	var pairingCode companionPairingCodeResponse
	if errorValue := json.Unmarshal(response.Body.Bytes(), &pairingCode); errorValue != nil {
		t.Fatal(errorValue)
	}
	if pairingCode.InstallCommand != "curl -fsSL https://intern.kim/companion/install.sh | sh" || pairingCode.ServiceCommand != "internkim-companion service install" {
		t.Fatalf("pairing code = %+v", pairingCode)
	}
	if !strings.HasSuffix(pairingCode.PairCommand, "--code "+pairingCode.Code) {
		t.Fatalf("pair command = %q", pairingCode.PairCommand)
	}

	stubPersonaActor(t, personaActor{email: "alice@example.com"}, true)
	keyPair, errorValue := companionruntime.GenerateKeyPair()
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	pairResponse := httptest.NewRecorder()
	handler.ServeHTTP(pairResponse, httptest.NewRequest(http.MethodPost, "/_internkim/companion/pair", strings.NewReader(`{"code":"`+pairingCode.Code+`","displayName":"laptop","publicKey":"`+keyPair.PublicKey+`","capabilities":[]}`)))
	if pairResponse.Code != http.StatusOK {
		t.Fatalf("pair status = %d body = %s", pairResponse.Code, pairResponse.Body.String())
	}
	listed := memberCompanions(t, handler)
	if len(listed) != 1 || listed[0].OwnerEmail != "alice@example.com" || listed[0].OwnerPersonID != "person-alice" {
		t.Fatalf("companions = %+v", listed)
	}
}

func TestMemberDisconnectsOnlyTheirOwnCompanion(t *testing.T) {
	handler := companionMemberTestService(t)
	mine := pairCompanionOwnedBy(t, handler, "alice@example.com", "person-alice")
	theirs := pairCompanionOwnedBy(t, handler, "bob@example.com", "person-bob")
	stubPersonaActor(t, personaActor{email: "alice@example.com", personID: "person-alice"}, true)

	refused := httptest.NewRecorder()
	handler.ServeHTTP(refused, httptest.NewRequest(http.MethodPost, "/companion/api/mine/disconnect", strings.NewReader(`{"companionID":"`+theirs+`"}`)))
	if refused.Code != http.StatusNotFound {
		t.Fatalf("disconnecting another member's companion answered %d", refused.Code)
	}

	disconnected := httptest.NewRecorder()
	handler.ServeHTTP(disconnected, httptest.NewRequest(http.MethodPost, "/companion/api/mine/disconnect", strings.NewReader(`{"companionID":"`+mine+`"}`)))
	if disconnected.Code != http.StatusOK {
		t.Fatalf("disconnect status = %d body = %s", disconnected.Code, disconnected.Body.String())
	}
	if listed := memberCompanions(t, handler); len(listed) != 0 {
		t.Fatalf("companions after disconnect = %+v", listed)
	}
}

func TestMemberCompanionRoutesNeedASignedInEmail(t *testing.T) {
	handler := companionMemberTestService(t)
	stubPersonaActor(t, personaActor{personID: "person-anonymous"}, true)

	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/companion/api/pairing-codes", nil))
	if response.Code != http.StatusForbidden {
		t.Fatalf("status = %d body = %s", response.Code, response.Body.String())
	}
}
