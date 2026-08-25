package cli

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestSignedReleaseApplyDocumentNamesTheReleaseAndChannel(t *testing.T) {
	stateDirectory := t.TempDir()
	saveState(stateDirectory, "fleet_id", "fleet-1")
	saveState(stateDirectory, "fleet_secret", "secret-1")

	document, errorValue := signedReleaseApplyDocument(commandTarget{stateDir: stateDirectory}, "release-1", deployReleaseChannel)

	if errorValue != nil {
		t.Fatal(errorValue)
	}
	var payload struct {
		recoveryRequest
		releaseSelection
	}
	if errorValue := json.Unmarshal(document, &payload); errorValue != nil {
		t.Fatal(errorValue)
	}
	if payload.Action != releaseUpdateSignedAction || payload.DeviceID != "fleet-1" {
		t.Fatalf("payload = %+v", payload)
	}
	if payload.ReleaseID != "release-1" || payload.Channel != "direct" {
		t.Fatalf("a deploy names the release it just published, got %+v", payload.releaseSelection)
	}
	expectedSignature := signCLIRecoveryPayload("secret-1", payload.Action, "", payload.DeviceID, payload.Nonce, payload.Timestamp)
	if payload.Signature != expectedSignature {
		t.Fatalf("signature = %q, expected %q", payload.Signature, expectedSignature)
	}
}

func TestSignedReleaseApplyDocumentNeedsTheFleetIdentity(t *testing.T) {
	_, errorValue := signedReleaseApplyDocument(commandTarget{stateDir: t.TempDir()}, "release-1", deployReleaseChannel)

	if errorValue == nil {
		t.Fatal("a device with no fleet secret cannot sign an apply")
	}
}

func TestReleaseAPIAppliesOverThePublicEndpointWhenItHasNoSSH(t *testing.T) {
	stateDirectory := t.TempDir()
	saveState(stateDirectory, "fleet_id", "fleet-1")
	saveState(stateDirectory, "fleet_secret", "secret-1")
	observedRequestURL := ""
	observedBody := []byte(nil)
	restoreReleaseUpdateHTTPClient := stubReleaseUpdateHTTPClient(t, func(request *http.Request) (*http.Response, error) {
		observedRequestURL = request.URL.String()
		observedBody, _ = io.ReadAll(request.Body)
		return jsonTestResponse(`{"jobID":"job-1","status":"running","phase":"downloading"}`), nil
	})
	defer restoreReleaseUpdateHTTPClient()
	api := deviceReleaseAPI{target: commandTarget{stateDir: stateDirectory, deviceURL: "https://device.example"}}

	job, errorValue := api.applyRelease("release-1", deployReleaseChannel)

	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if job.JobID != "job-1" {
		t.Fatalf("job = %+v", job)
	}
	if observedRequestURL != "https://device.example/admin/api/updates/apply" {
		t.Fatalf("request URL = %q", observedRequestURL)
	}
	if !strings.Contains(string(observedBody), `"releaseID":"release-1"`) || !strings.Contains(string(observedBody), `"channel":"direct"`) {
		t.Fatalf("body = %s", observedBody)
	}
}

func TestReleaseAPIReadsJobStatusOverThePublicEndpoint(t *testing.T) {
	observedRequestURL := ""
	restoreReleaseUpdateHTTPClient := stubReleaseUpdateHTTPClient(t, func(request *http.Request) (*http.Response, error) {
		observedRequestURL = request.URL.String()
		return jsonTestResponse(`{"jobID":"job-1","status":"completed","phase":"verified"}`), nil
	})
	defer restoreReleaseUpdateHTTPClient()
	api := deviceReleaseAPI{target: commandTarget{stateDir: t.TempDir(), deviceURL: "https://device.example"}}

	job, errorValue := api.releaseUpdateJob("job-1")

	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if job.Status != "completed" {
		t.Fatalf("job = %+v", job)
	}
	if observedRequestURL != "https://device.example/admin/api/updates/jobs/job-1" {
		t.Fatalf("request URL = %q", observedRequestURL)
	}
}

func stubReleaseUpdateHTTPClient(t *testing.T, roundTrip roundTripFunc) func() {
	t.Helper()
	originalClient := releaseUpdateHTTPClient
	releaseUpdateHTTPClient = &http.Client{Transport: roundTrip}
	return func() { releaseUpdateHTTPClient = originalClient }
}

func jsonTestResponse(document string) *http.Response {
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(document)),
	}
}
