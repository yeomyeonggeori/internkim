package admind

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

func serviceWhoseDirectoryDroppedSomebody(t *testing.T) *Service {
	t.Helper()
	service := serviceWhoseDirectorySays(t, `{"members":[
		{"memberID":"member-1","email":"member@example.com","name":"이샘플","role":"member","status":"active"}
	]}`)
	service.Configuration.BlueclawPolicyDeliveryPath = filepath.Join(t.TempDir(), "policy.json")
	answering := service.HTTPClient.Transport
	service.HTTPClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.URL.String() == "http://blueclaw.local/admin/api/policy" {
			return jsonResponse(http.StatusOK, `{"people":[
				{"personID":"member-1","emails":["member@example.com"]},
				{"personID":"member-2","emails":["departed@example.com"]}
			]}`, nil), nil
		}
		return answering.RoundTrip(request)
	})}
	return service
}

func deliveredRosterEmails(t *testing.T, service *Service) []string {
	t.Helper()
	delivered, errorValue := os.ReadFile(service.Configuration.BlueclawPolicyDeliveryPath)
	if errorValue != nil {
		return nil
	}
	var policyDocument map[string]any
	if errorValue := json.Unmarshal(delivered, &policyDocument); errorValue != nil {
		t.Fatalf("parse the delivered roster: %v", errorValue)
	}
	return rosterPolicyEmails(policyDocument)
}

func TestTheNotificationAndTheClockApplyTheSameDirectory(t *testing.T) {
	directoryApplyInterval = 20 * time.Millisecond
	t.Cleanup(func() { directoryApplyInterval = 2 * time.Minute })
	triggers := map[string]func(t *testing.T, service *Service){
		"notification": func(t *testing.T, service *Service) {
			response := httptest.NewRecorder()
			service.handleDirectoryChanged(response, httptest.NewRequest(http.MethodPost, "/admin/api/directory/changed", nil))
			if response.Code != http.StatusAccepted {
				t.Fatalf("the door answered %d: %s", response.Code, response.Body.String())
			}
		},
		"clock": func(t *testing.T, service *Service) {
			ctx, stop := context.WithCancel(t.Context())
			defer stop()
			service.startDirectoryApplying(ctx)
			deadline := time.Now().Add(2 * time.Second)
			for deliveredRosterEmails(t, service) == nil && time.Now().Before(deadline) {
				time.Sleep(10 * time.Millisecond)
			}
		},
	}
	for name, trigger := range triggers {
		t.Run(name, func(t *testing.T) {
			service := serviceWhoseDirectoryDroppedSomebody(t)

			trigger(t, service)

			emails := deliveredRosterEmails(t, service)
			if !slices.Equal(emails, []string{"member@example.com"}) {
				t.Fatalf("the %s left the roster at %v; the directory names only member@example.com", name, emails)
			}
		})
	}
}

func TestAMessengerStepThatCannotFinishMakesTheDirectoryUnapplied(t *testing.T) {
	service := serviceWhoseDirectoryDroppedSomebody(t)
	service.Configuration.BuzzRelayURL = "ws://127.0.0.1:1"
	service.Configuration.BuzzDatabaseURL = "postgres://buzz@127.0.0.1:1/buzz?sslmode=disable&connect_timeout=1"

	errorValue := service.applyDirectory(t.Context())

	if errorValue == nil || !strings.Contains(errorValue.Error(), "messenger seats") {
		t.Fatalf("the messenger database cannot be reached, so the seats are not done and the pass must say so, got %v", errorValue)
	}
	if emails := deliveredRosterEmails(t, service); !slices.Equal(emails, []string{"member@example.com"}) {
		t.Fatalf("a messenger that cannot be reached must not hold the roster back, got %v", emails)
	}
}
