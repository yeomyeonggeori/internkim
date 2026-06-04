package cli

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestPerformSSHRecoveryRequestSignsPublicAdminRequest(t *testing.T) {
	stateDirectory := t.TempDir()
	saveState(stateDirectory, "device_url", "https://device.example")
	saveState(stateDirectory, "fleet_id", "fleet-1")
	saveState(stateDirectory, "fleet_secret", "secret-1")

	originalStatusHTTPClient := statusHTTPClient
	defer func() { statusHTTPClient = originalStatusHTTPClient }()
	statusHTTPClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.Method != http.MethodPost || request.URL.Path != "/admin/api/recovery/ssh-tunnel/restart" {
			t.Fatalf("unexpected request %s %s", request.Method, request.URL.Path)
		}
		var payload recoveryRequest
		if errorValue := json.NewDecoder(request.Body).Decode(&payload); errorValue != nil {
			t.Fatal(errorValue)
		}
		if payload.Action != "restart-cloudflared-node-ssh" || payload.DeviceID != "fleet-1" {
			t.Fatalf("unexpected payload %+v", payload)
		}
		expectedSignature := signCLIRecoveryPayload("secret-1", payload.Action, payload.DeviceID, payload.Nonce, payload.Timestamp)
		if payload.Signature != expectedSignature {
			t.Fatalf("signature = %q, expected %q", payload.Signature, expectedSignature)
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(strings.NewReader(`{"status":"ok","action":"restart-cloudflared-node-ssh","services":{"ssh":"active"}}`)),
		}, nil
	})}

	response, errorValue := performSSHRecoveryRequest(commandTarget{
		stateDir:  stateDirectory,
		deviceURL: "https://device.example",
	}, "restart-cloudflared-node-ssh")
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if response.Action != "restart-cloudflared-node-ssh" || response.Services["ssh"] != "active" {
		t.Fatalf("unexpected response %+v", response)
	}
}

func TestPerformSSHRecoveryRequestRequiresFleetIdentity(t *testing.T) {
	_, errorValue := performSSHRecoveryRequest(commandTarget{
		stateDir:  t.TempDir(),
		deviceURL: "https://device.example",
	}, "status")
	if errorValue == nil || !strings.Contains(errorValue.Error(), "fleet identity") {
		t.Fatalf("expected fleet identity error, got %v", errorValue)
	}
}

func TestPerformSSHRecoveryRequestExplainsRedirectAsMissingEndpoint(t *testing.T) {
	stateDirectory := t.TempDir()
	saveState(stateDirectory, "fleet_id", "fleet-1")
	saveState(stateDirectory, "fleet_secret", "secret-1")

	originalStatusHTTPClient := statusHTTPClient
	defer func() { statusHTTPClient = originalStatusHTTPClient }()
	statusHTTPClient = &http.Client{
		CheckRedirect: func(request *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
		Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			if request.URL.Path == "/admin/api/health" {
				return textHTTPResponse(http.StatusOK, `{"status":"ok","admindBuildID":"build-1","recoveryAvailable":true}`), nil
			}
			response := textHTTPResponse(http.StatusFound, "")
			response.Header.Set("Location", "/admin/")
			return response, nil
		})}

	_, errorValue := performSSHRecoveryRequest(commandTarget{
		stateDir:  stateDirectory,
		deviceURL: "https://device.example",
	}, "status")
	for _, expectedText := range []string{"recovery endpoint redirected", "location=\"/admin/\"", "health=HTTP 200"} {
		if errorValue == nil || !strings.Contains(errorValue.Error(), expectedText) {
			t.Fatalf("expected redirect explanation to contain %q, got %v", expectedText, errorValue)
		}
	}
}
