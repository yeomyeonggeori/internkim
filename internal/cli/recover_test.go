package cli

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestPerformSSHRecoveryRequestSignsPublicAdminRequest(t *testing.T) {

	originalRecoveryHTTPClient := recoveryHTTPClient
	defer func() { recoveryHTTPClient = originalRecoveryHTTPClient }()
	recoveryHTTPClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
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
		expectedSignature := signCLIRecoveryPayload("secret-1", payload.Action, "", payload.DeviceID, payload.Nonce, payload.Timestamp)
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
		deviceURL:   "https://device.example",
		fleetID:     "fleet-1",
		fleetSecret: "secret-1",
	}, "restart-cloudflared-node-ssh", "")
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if response.Action != "restart-cloudflared-node-ssh" || response.Services["ssh"] != "active" {
		t.Fatalf("unexpected response %+v", response)
	}
}

func TestPerformSSHRecoveryRequestRequiresFleetIdentity(t *testing.T) {
	_, errorValue := performSSHRecoveryRequest(commandTarget{
		deviceURL: "https://device.example",
	}, "status", "")
	if errorValue == nil || !strings.Contains(errorValue.Error(), "INTERNKIM_FLEET_SECRET") {
		t.Fatalf("expected the refusal to name the missing key, got %v", errorValue)
	}
}

func TestPerformSSHRecoveryRequestExplainsRedirectAsMissingEndpoint(t *testing.T) {

	originalRecoveryHTTPClient := recoveryHTTPClient
	originalStatusHTTPClient := healthHTTPClient
	defer func() {
		recoveryHTTPClient = originalRecoveryHTTPClient
		healthHTTPClient = originalStatusHTTPClient
	}()
	stubbedClient := &http.Client{
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
	recoveryHTTPClient = stubbedClient
	healthHTTPClient = stubbedClient

	_, errorValue := performSSHRecoveryRequest(commandTarget{
		deviceURL:   "https://device.example",
		fleetID:     "fleet-1",
		fleetSecret: "secret-1",
	}, "status", "")
	for _, expectedText := range []string{"recovery endpoint redirected", "location=\"/admin/\"", "health=HTTP 200"} {
		if errorValue == nil || !strings.Contains(errorValue.Error(), expectedText) {
			t.Fatalf("expected redirect explanation to contain %q, got %v", expectedText, errorValue)
		}
	}
}

type roundTripFunc func(request *http.Request) (*http.Response, error)

func (function roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return function(request)
}

func textHTTPResponse(statusCode int, body string) *http.Response {
	return &http.Response{
		StatusCode: statusCode,
		Header:     http.Header{"Content-Type": []string{"text/plain"}},
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}
