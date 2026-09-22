package admind

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gitlab.com/eastriver/internkim/internal/buzzidentity"
)

func serviceRecordingRelayMembershipGrants(t *testing.T) (*Service, string) {
	t.Helper()
	service := serviceWhoseRecordIsDown(t)
	directory := t.TempDir()
	grantsPath := filepath.Join(directory, "granted")
	commandPath := filepath.Join(directory, "buzz-admin")
	script := "#!/bin/sh\nwhile [ $# -gt 0 ]; do\n  if [ \"$1\" = \"--pubkey\" ]; then printf '%s\\n' \"$2\" >> " + grantsPath + "; fi\n  shift\ndone\n"
	if errorValue := os.WriteFile(commandPath, []byte(script), 0o700); errorValue != nil {
		t.Fatalf("write fake buzz-admin: %v", errorValue)
	}
	service.Configuration.BuzzAdminCommandPath = commandPath
	service.Configuration.BuzzRelayURL = "ws://127.0.0.1:3000"
	service.Configuration.BuzzDatabaseURL = "postgres://buzz@127.0.0.1:1/buzz?sslmode=disable"
	return service, grantsPath
}

func TestAPersonTheDeviceCanNameOnBuzzIsLetOntoTheRelay(t *testing.T) {
	service, grantsPath := serviceRecordingRelayMembershipGrants(t)
	service.HTTPClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if isBlueclawPolicyGet(request) {
			return jsonResponse(http.StatusOK, `{"people":[{"personID":"p-1","emails":["sample@example.com"]}]}`, nil), nil
		}
		return nil, errors.New("the company directory is not answering")
	})}
	service.Configuration.BuzzAccountLinksPath = filepath.Join(t.TempDir(), "buzz-account-links.json")

	service.linkDeterministicBuzzPeople(context.Background())

	granted, errorValue := os.ReadFile(grantsPath)
	if errorValue != nil {
		t.Fatalf("a person the device just learned to name on buzz was not let onto the relay: %v", errorValue)
	}
	secretHex := service.buzzSecretForEmail(context.Background(), "sample@example.com")
	pubkey, errorValue := buzzPublicKey(secretHex)
	if errorValue != nil {
		t.Fatalf("person pubkey: %v", errorValue)
	}
	if !strings.Contains(string(granted), pubkey) {
		t.Fatalf("the key the account links file now carries must be a relay member, got %s", strings.TrimSpace(string(granted)))
	}
}

// The messenger migrates its own database while admind is making its first
// pass, so the first grant answers `relation "communities" does not exist`. The
// link is written either way, so a pass that only lets in whoever is new lets
// nobody in ever again, and the person is named and locked out.
func TestAGrantTheRelayCouldNotTakeIsMadeAgainOnTheNextPass(t *testing.T) {
	service, grantsPath := serviceRecordingRelayMembershipGrants(t)
	service.HTTPClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if isBlueclawPolicyGet(request) {
			return jsonResponse(http.StatusOK, `{"people":[{"personID":"p-1","emails":["sample@example.com"]}]}`, nil), nil
		}
		return nil, errors.New("the company directory is not answering")
	})}
	service.Configuration.BuzzAccountLinksPath = filepath.Join(t.TempDir(), "buzz-account-links.json")

	service.linkDeterministicBuzzPeople(context.Background())
	service.linkDeterministicBuzzPeople(context.Background())

	secretHex := service.buzzSecretForEmail(context.Background(), "sample@example.com")
	pubkey, errorValue := buzzPublicKey(secretHex)
	if errorValue != nil {
		t.Fatalf("person pubkey: %v", errorValue)
	}
	granted, errorValue := os.ReadFile(grantsPath)
	if errorValue != nil {
		t.Fatalf("nobody was let onto the relay at all: %v", errorValue)
	}
	if strings.Count(string(granted), pubkey) < 2 {
		t.Fatalf("a grant the relay could not take must be made again, got %s", strings.TrimSpace(string(granted)))
	}
}

func TestRelayMembershipIsGrantedBeforeAnyRoomExists(t *testing.T) {
	service, grantsPath := serviceRecordingRelayMembershipGrants(t)

	service.ensureMemberChannelMembership(context.Background())

	granted, errorValue := os.ReadFile(grantsPath)
	if errorValue != nil {
		t.Fatalf("a relay with no rooms let nobody in, so nothing could ever open one: %v", errorValue)
	}
	agentPubkey, errorValue := buzzPublicKey(buzzidentity.Secret(identityTestSeed, buzzidentity.AgentSubject))
	if errorValue != nil {
		t.Fatalf("agent pubkey: %v", errorValue)
	}
	if !strings.Contains(string(granted), agentPubkey) {
		t.Fatalf("the agent must be a relay member before it can open the first room, got %s", strings.TrimSpace(string(granted)))
	}
}

func TestDirectoryChangedSeatsMemberBeforeBuzzLogin(t *testing.T) {
	service, grantsPath := serviceRecordingRelayMembershipGrants(t)
	agentKeyPath := filepath.Join(t.TempDir(), "agent-key")
	if errorValue := os.WriteFile(agentKeyPath, []byte("agent-key"), 0o600); errorValue != nil {
		t.Fatalf("write agent key: %v", errorValue)
	}
	service.Configuration.CentralPlaneAppURL = "http://central.test"
	service.Configuration.CentralPlaneProjectURL = "http://supabase.test"
	service.Configuration.CentralPlanePublishableKey = "publishable-key"
	service.Configuration.CentralPlaneAgentKeyPath = agentKeyPath
	service.HTTPClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.URL.Path == "/api/agent/member" && request.Method == http.MethodGet {
			return jsonResponse(http.StatusOK, `{"members":[{"memberID":"member-1","email":"newcomer@example.com","name":"박예시","status":"invited"}]}`, nil), nil
		}
		if request.URL.Path == "/api/agent/messenger-credential" && request.Method == http.MethodPost {
			return jsonResponse(http.StatusOK, `{}`, nil), nil
		}
		if isBlueclawPolicyGet(request) {
			return jsonResponse(http.StatusOK, `{"people":[]}`, nil), nil
		}
		return jsonResponse(http.StatusOK, `{}`, nil), nil
	})}

	request := httptest.NewRequest(http.MethodPost, "/admin/api/directory/changed", nil)
	response := httptest.NewRecorder()
	service.handleDirectoryChanged(response, request)
	if response.Code != http.StatusAccepted {
		t.Fatalf("directory change answered %d: %s", response.Code, response.Body.String())
	}

	granted, errorValue := os.ReadFile(grantsPath)
	if errorValue != nil {
		t.Fatalf("directory change did not grant relay membership: %v", errorValue)
	}
	secretHex := service.buzzSecretForEmail(context.Background(), "newcomer@example.com")
	pubkey, errorValue := buzzPublicKey(secretHex)
	if errorValue != nil {
		t.Fatalf("newcomer pubkey: %v", errorValue)
	}
	if !containsLine(string(granted), pubkey) {
		t.Fatalf("directory change must seat a newcomer before first Buzz login, got %s", strings.TrimSpace(string(granted)))
	}
}

func containsLine(content string, expected string) bool {
	for _, line := range strings.Split(content, "\n") {
		if strings.TrimSpace(line) == expected {
			return true
		}
	}
	return false
}
