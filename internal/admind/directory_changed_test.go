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
		{"memberID":"member-1","email":"member@example.com","name":"이샘플","role":"member","status":"active","isWorking":true,"canReadTheCompany":true}
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

func applyTheDirectoryUntilTheTestEnds(t *testing.T, service *Service) {
	t.Helper()
	ctx, stop := context.WithCancel(t.Context())
	stopped := make(chan struct{})
	go func() {
		service.keepTheDirectoryApplied(ctx)
		close(stopped)
	}()
	t.Cleanup(func() {
		stop()
		<-stopped
	})
}

func waitForTheDeliveredRoster(t *testing.T, service *Service) []string {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if emails := deliveredRosterEmails(t, service); emails != nil {
			return emails
		}
		time.Sleep(10 * time.Millisecond)
	}
	return nil
}

func TestTheNotificationAndTheClockApplyTheSameDirectory(t *testing.T) {
	triggers := map[string]func(t *testing.T, service *Service){
		"notification": func(t *testing.T, service *Service) {
			waitForTheDeliveredRoster(t, service)
			if errorValue := os.Remove(service.Configuration.BlueclawPolicyDeliveryPath); errorValue != nil {
				t.Fatalf("clear the roster the first pass delivered: %v", errorValue)
			}
			response := httptest.NewRecorder()
			service.handleDirectoryChanged(response, httptest.NewRequest(http.MethodPost, "/admin/api/directory/changed", nil))
			if response.Code != http.StatusAccepted {
				t.Fatalf("the door answered %d: %s", response.Code, response.Body.String())
			}
		},
		"clock": func(t *testing.T, service *Service) {
			waitForTheDeliveredRoster(t, service)
			if errorValue := os.Remove(service.Configuration.BlueclawPolicyDeliveryPath); errorValue != nil {
				t.Fatalf("clear the roster the first pass delivered: %v", errorValue)
			}
		},
	}
	applyIntervals := map[string]time.Duration{"notification": 2 * time.Minute, "clock": 20 * time.Millisecond}
	for name, trigger := range triggers {
		t.Run(name, func(t *testing.T) {
			directoryApplyInterval = applyIntervals[name]
			t.Cleanup(func() { directoryApplyInterval = 2 * time.Minute })
			service := serviceWhoseDirectoryDroppedSomebody(t)
			applyTheDirectoryUntilTheTestEnds(t, service)

			trigger(t, service)

			emails := waitForTheDeliveredRoster(t, service)
			if !slices.Equal(emails, []string{"member@example.com"}) {
				t.Fatalf("the %s left the roster at %v; the directory names only member@example.com", name, emails)
			}
		})
	}
}

func TestTheNotificationIsAnsweredBeforeTheDirectoryIsApplied(t *testing.T) {
	service := serviceWhoseDirectoryDroppedSomebody(t)
	release := make(chan struct{})
	t.Cleanup(func() { close(release) })
	answering := service.HTTPClient.Transport
	service.HTTPClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		select {
		case <-release:
		case <-request.Context().Done():
			return nil, request.Context().Err()
		}
		return answering.RoundTrip(request)
	})}
	applyTheDirectoryUntilTheTestEnds(t, service)

	answered := make(chan int, 1)
	go func() {
		response := httptest.NewRecorder()
		service.handleDirectoryChanged(response, httptest.NewRequest(http.MethodPost, "/admin/api/directory/changed", nil))
		answered <- response.Code
	}()

	select {
	case status := <-answered:
		if status != http.StatusAccepted {
			t.Fatalf("the door answered %d, want 202", status)
		}
	case <-time.After(time.Second):
		t.Fatal("the door waited for a directory pass that has not finished; the company waits on this answer while it writes a person")
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
