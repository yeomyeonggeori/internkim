package admind

import (
	"context"
	"errors"
	"net/http"
	"sync/atomic"
	"testing"
	"time"
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

// Somebody is invited today and signs in tomorrow. The key their messenger
// knows them by was derived only for members already active, so the invitation
// recorded nothing, and the sweep that would have caught up did not exist: the
// agent could not tell who they were when they arrived.
func TestAnInvitedMemberIsGivenTheirKeyBeforeTheyArrive(t *testing.T) {
	service := serviceWhoseDirectorySays(t, `{"members":[
		{"memberID":"member-1","email":"invited@example.com","name":"이샘플","role":"member","status":"invited"},
		{"memberID":"member-2","email":"pending@example.com","name":"박예시","role":"member","status":"pending"},
		{"memberID":"member-3","email":"active@example.com","name":"최견본","role":"member","status":"active"}
	]}`)

	recording := service.recordBuzzCredentials(t.Context())

	if recording.Kept != 3 {
		t.Fatalf("a colleague who has not signed in yet was passed over: %s", recording)
	}
	if recording.Skipped != 0 {
		t.Fatalf("nobody in this directory should have been passed over: %s", recording)
	}
}

func TestSomebodyWhoLeftIsGivenNoKey(t *testing.T) {
	service := serviceWhoseDirectorySays(t, `{"members":[
		{"memberID":"member-1","email":"departed@example.com","name":"이샘플","role":"member","status":"departed"},
		{"memberID":"member-2","email":"withdrawn@example.com","name":"박예시","role":"member","status":"withdrawn"}
	]}`)

	recording := service.recordBuzzCredentials(t.Context())

	if recording.Kept != 0 {
		t.Fatalf("a former colleague was given a key: %s", recording)
	}
	if recording.Skipped != 2 {
		t.Fatalf("both should have been passed over: %s", recording)
	}
}

func TestTheStartupSweepRecordsKeysOnceBlueclawAnswersRatherThanAtTheNextTick(t *testing.T) {
	firstWaitForBlueclaw, longestWaitForBlueclaw = 10*time.Millisecond, 20*time.Millisecond
	t.Cleanup(func() { firstWaitForBlueclaw, longestWaitForBlueclaw = 250*time.Millisecond, 5*time.Second })
	refusalsLeft := atomic.Int32{}
	refusalsLeft.Store(3)
	kept := atomic.Int32{}
	service := serviceWhoseDirectorySays(t, `{"members":[
		{"memberID":"member-1","email":"active@example.com","name":"최견본","role":"member","status":"active"}
	]}`)
	answering := service.HTTPClient.Transport
	service.HTTPClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.URL.Host == "blueclaw.local" && refusalsLeft.Load() > 0 {
			if request.URL.Path == "/admin/api/health" {
				refusalsLeft.Add(-1)
			}
			return nil, errors.New("connection refused")
		}
		if request.URL.Path == "/api/agent/messenger-credential" {
			kept.Add(1)
		}
		return answering.RoundTrip(request)
	})}
	ctx, stop := context.WithCancel(t.Context())
	defer stop()

	service.startBuzzCredentialSweep(ctx)

	deadline := time.Now().Add(2 * time.Second)
	for kept.Load() == 0 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if kept.Load() == 0 {
		t.Fatal("blueclaw came up and nobody's key was recorded, so members wait for the next sweep to message the agent")
	}
}
