package admind

import (
	"encoding/json"
	"net/http"
	"slices"
	"testing"
)

func serviceWhoseDirectorySays(t *testing.T, members string) *Service {
	t.Helper()
	service := NewService(Configuration{
		CentralPlaneAppURL:         "https://company.example.test",
		CentralPlaneProjectURL:     "https://project.example.test",
		CentralPlanePublishableKey: "publishable",
		CentralPlaneAgentKeyPath:   writeTestFile(t, "agent-key"),
		BuzzKeySeedPath:            writeTestFile(t, "a-seed-for-this-test"),
		BlueclawBaseURL:            "http://blueclaw.local",
		StateDirectory:             t.TempDir(),
	})
	service.HTTPClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		switch {
		case request.URL.String() == "https://company.example.test/api/agent/member" && request.Method == http.MethodGet:
			return jsonResponse(http.StatusOK, members, nil), nil
		case request.URL.String() == "http://blueclaw.local/admin/api/policy":
			return jsonResponse(http.StatusOK, `{"people":[]}`, nil), nil
		default:
			return jsonResponse(http.StatusOK, `{}`, nil), nil
		}
	})}
	return service
}

func messengerKeysRecordedFor(t *testing.T, service *Service) []string {
	t.Helper()
	recorded := []string{}
	answering := service.HTTPClient.Transport
	service.HTTPClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.URL.Path == "/api/agent/messenger-credential" {
			var credential struct {
				MemberID string `json:"memberID"`
			}
			if errorValue := json.NewDecoder(request.Body).Decode(&credential); errorValue != nil {
				t.Fatalf("decode the recorded credential: %v", errorValue)
			}
			recorded = append(recorded, credential.MemberID)
		}
		return answering.RoundTrip(request)
	})}
	client := service.centralPlane()
	members, errorValue := client.Members(t.Context())
	if errorValue != nil {
		t.Fatalf("read the directory: %v", errorValue)
	}
	if errorValue := service.recordMessengerKeys(t.Context(), client, members); errorValue != nil {
		t.Fatalf("the messenger keys were not all recorded: %v", errorValue)
	}
	return recorded
}

// Somebody is invited today and signs in tomorrow. The key their messenger
// knows them by was derived only for members already active, so the invitation
// recorded nothing, and the agent could not tell who they were when they
// arrived.
func TestAnInvitedMemberIsGivenTheirKeyBeforeTheyArrive(t *testing.T) {
	service := serviceWhoseDirectorySays(t, `{"members":[
		{"memberID":"member-1","email":"invited@example.com","name":"이샘플","role":"member","status":"invited","isWorking":false,"canReadTheCompany":true},
		{"memberID":"member-2","email":"pending@example.com","name":"박예시","role":"member","status":"pending","isWorking":false,"canReadTheCompany":true},
		{"memberID":"member-3","email":"active@example.com","name":"최견본","role":"member","status":"active","isWorking":true,"canReadTheCompany":true}
	]}`)

	recorded := messengerKeysRecordedFor(t, service)

	if !slices.Equal(recorded, []string{"member-1", "member-2", "member-3"}) {
		t.Fatalf("a colleague who has not signed in yet was passed over: %v", recorded)
	}
}

func TestSomebodyWhoLeftIsGivenNoKey(t *testing.T) {
	service := serviceWhoseDirectorySays(t, `{"members":[
		{"memberID":"member-1","email":"departed@example.com","name":"이샘플","role":"member","status":"departed","isWorking":false,"canReadTheCompany":false},
		{"memberID":"member-2","email":"withdrawn@example.com","name":"박예시","role":"member","status":"withdrawn","isWorking":false,"canReadTheCompany":false}
	]}`)

	if recorded := messengerKeysRecordedFor(t, service); len(recorded) != 0 {
		t.Fatalf("a former colleague was given a key: %v", recorded)
	}
}
