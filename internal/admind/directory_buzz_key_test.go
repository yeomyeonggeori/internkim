package admind

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	nostr "github.com/nbd-wtf/go-nostr"

	"github.com/yeomyeonggeori/internkim/internal/buzzidentity"
)

func buzzKeyTestService(t *testing.T, memberEmail string) *Service {
	t.Helper()
	stateDirectory := t.TempDir()
	seedPath := filepath.Join(stateDirectory, "buzz-key-seed")
	writeFile(t, seedPath, "test-seed")
	service := NewService(Configuration{
		StateDirectory:  stateDirectory,
		BuzzKeySeedPath: seedPath,
	})
	service.HTTPClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if isBlueclawPolicyGet(request) {
			return jsonResponse(http.StatusOK, `{"people":[]}`, nil), nil
		}
		if !isCompanyDirectoryRequest(request) {
			return jsonResponse(http.StatusNotFound, `{}`, nil), nil
		}
		email := strings.ToLower(strings.TrimSpace(request.URL.Query().Get("email")))
		if email != memberEmail {
			return jsonResponse(http.StatusOK, `{"members":[]}`, nil), nil
		}
		return jsonResponse(http.StatusOK, `{"member":`+memberJSONForTest(email)+`}`, nil), nil
	})}
	seatPeopleInACompanyDirectoryForTest(t, service)
	return service
}

func postBuzzKey(service *Service, email string) *httptest.ResponseRecorder {
	body, _ := json.Marshal(map[string]string{"email": email})
	request := httptest.NewRequest(http.MethodPost, "/admin/api/directory/buzz-key", bytes.NewReader(body))
	recorder := httptest.NewRecorder()
	service.handleDirectoryBuzzKey(recorder, request)
	return recorder
}

func TestDirectoryBuzzKeyAnswersAMembersCurrentKey(t *testing.T) {
	service := buzzKeyTestService(t, "sample@example.com")
	recorder := postBuzzKey(service, "Sample@Example.com")
	if recorder.Code != http.StatusOK {
		t.Fatalf("answered %d: %s", recorder.Code, recorder.Body.String())
	}
	var response directoryBuzzKeyResponse
	if errorValue := json.Unmarshal(recorder.Body.Bytes(), &response); errorValue != nil {
		t.Fatalf("response decode failed: %v", errorValue)
	}
	expectedPubkey, _ := nostr.GetPublicKey(buzzidentity.Secret("test-seed", "sample@example.com"))
	if response.PubkeyHex != expectedPubkey {
		t.Fatalf("answered %q, derived %q", response.PubkeyHex, expectedPubkey)
	}
}

func TestDirectoryBuzzKeyRefusesAStranger(t *testing.T) {
	service := buzzKeyTestService(t, "sample@example.com")
	recorder := postBuzzKey(service, "stranger@example.com")
	if recorder.Code != http.StatusNotFound {
		t.Fatalf("a stranger answered %d", recorder.Code)
	}
	if strings.Contains(recorder.Body.String(), "pubkeyHex") {
		t.Fatal("a stranger must not receive a key")
	}
}
