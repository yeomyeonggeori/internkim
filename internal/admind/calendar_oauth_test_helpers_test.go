package admind

import (
	"net/http"
	"os"
	"path/filepath"
	"testing"
)

func writeGoogleClientFile(t *testing.T, service *Service, content string) {
	t.Helper()
	directory := service.calendarSecretsDirectory()
	if errorValue := os.MkdirAll(directory, 0o700); errorValue != nil {
		t.Fatalf("mkdir: %v", errorValue)
	}
	path := filepath.Join(directory, googleOAuthClientFileName)
	if errorValue := os.WriteFile(path, []byte(content), 0o600); errorValue != nil {
		t.Fatalf("write client.json: %v", errorValue)
	}
}

func configureCalendarTestUsers(t *testing.T, service *Service, recordsJSON string) {
	t.Helper()
	service.Configuration.APIBaseURL = "https://api.intern.kim"
	service.Configuration.FleetIDPath = writeTestFile(t, "dc719d8e")
	service.Configuration.FleetSecretPath = writeTestFile(t, "secret-value")
	service.HTTPClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.Method == http.MethodGet && request.URL.String() == "https://api.intern.kim/api/users?fleet_id=dc719d8e" {
			return jsonResponse(http.StatusOK, `{"records":`+recordsJSON+`}`, nil), nil
		}
		t.Fatalf("unexpected request %s %s", request.Method, request.URL.String())
		return nil, nil
	})}
}
