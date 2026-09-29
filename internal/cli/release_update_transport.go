package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

const releaseUpdateApplyPath = "/admin/api/updates/apply"
const releaseUpdateJobPath = "/admin/api/updates/jobs/"
const releaseUpdateStatusPath = "/admin/api/updates/status"
const releaseUpdateSignedAction = "release-update-apply"
const defaultReleaseUpdateChannel = "stable"

type releaseSelection struct {
	ReleaseID string `json:"releaseID,omitempty"`
	Channel   string `json:"channel,omitempty"`
}

type signedReleaseUpdateApplyRequest struct {
	recoveryRequest
	releaseSelection
}

// The release API answers on the device's loopback, so ssh is the usual way in.
// When ssh is unreachable and the device's public endpoint still answers, apply
// goes through it as a fleet-signed request and the reads through the same
// endpoint, which serves them without authorization.
type deviceReleaseAPI struct {
	target        commandTarget
	loopbackAdmin *deviceAdmin
}

func reachDeviceReleaseAPI(target commandTarget) (deviceReleaseAPI, error) {
	admin, sshError := reachDeviceAdmin(target)
	if sshError == nil {
		return deviceReleaseAPI{target: target, loopbackAdmin: &admin}, nil
	}
	if !isPublicAdminHealthy(target) {
		return deviceReleaseAPI{}, sshError
	}
	fmt.Printf("SSH: unreachable (%v); reaching the release API over the device's public endpoint\n", sshError)
	return deviceReleaseAPI{target: target}, nil
}

func isPublicAdminHealthy(target commandTarget) bool {
	statusCode, responseBody, errorValue := fetchPublicEndpoint(deviceURLForTarget(target), "/admin/api/health")
	if errorValue != nil || statusCode < 200 || statusCode >= 300 {
		return false
	}
	return strings.Contains(compactJSONSpaces(responseBody), `"status":"ok"`)
}

func (api deviceReleaseAPI) applyRelease(releaseID string, channel string) (blueclawUpdateJobResponse, error) {
	var job blueclawUpdateJobResponse
	if api.loopbackAdmin != nil {
		document, errorValue := json.Marshal(releaseSelection{ReleaseID: releaseID, Channel: channel})
		if errorValue != nil {
			return job, errorValue
		}
		return job, api.loopbackAdmin.ask(http.MethodPost, releaseUpdateApplyPath, document, &job)
	}
	document, errorValue := signedReleaseApplyDocument(api.target, releaseID, channel)
	if errorValue != nil {
		return job, errorValue
	}
	return job, api.askPublicEndpoint(http.MethodPost, releaseUpdateApplyPath, document, &job)
}

func signedReleaseApplyDocument(target commandTarget, releaseID string, channel string) ([]byte, error) {
	fleetID, fleetSecret, errorValue := target.fleetIdentity()
	if errorValue != nil {
		return nil, errorValue
	}
	return json.Marshal(signedReleaseUpdateApplyRequest{
		recoveryRequest:  signedRecoveryRequestPayload(fleetSecret, releaseUpdateSignedAction, "", fleetID),
		releaseSelection: releaseSelection{ReleaseID: releaseID, Channel: channel},
	})
}

func (api deviceReleaseAPI) releaseUpdateJob(jobID string) (blueclawUpdateJobResponse, error) {
	var job blueclawUpdateJobResponse
	return job, api.read(releaseUpdateJobPath+jobID, &job)
}

func (api deviceReleaseAPI) releaseUpdateStatus() (releaseUpdateStatusResponse, error) {
	var status releaseUpdateStatusResponse
	return status, api.read(releaseUpdateStatusPath, &status)
}

func (api deviceReleaseAPI) read(path string, answer any) error {
	if api.loopbackAdmin != nil {
		return api.loopbackAdmin.ask(http.MethodGet, path, nil, answer)
	}
	return api.askPublicEndpoint(http.MethodGet, path, nil, answer)
}

func (api deviceReleaseAPI) askPublicEndpoint(method string, path string, body []byte, answer any) error {
	endpointURL, errorValue := releaseDeviceEndpointURL(api.target, path)
	if errorValue != nil {
		return errorValue
	}
	statusCode, responseBody, errorValue := sendReleaseUpdateRequest(func() (*http.Request, error) {
		return newPublicEndpointRequest(method, endpointURL, body)
	})
	if errorValue != nil {
		return fmt.Errorf("%s %s over the public endpoint failed: %w", method, path, errorValue)
	}
	if statusCode < 200 || statusCode >= 300 {
		return fmt.Errorf("%s %s over the public endpoint answered HTTP %d: %s", method, path, statusCode, strings.TrimSpace(string(responseBody)))
	}
	if answer == nil {
		return nil
	}
	if errorValue := json.Unmarshal(responseBody, answer); errorValue != nil {
		return fmt.Errorf("%s %s answered something that is not JSON: %s", method, path, strings.TrimSpace(string(responseBody)))
	}
	return nil
}

func newPublicEndpointRequest(method string, endpointURL string, body []byte) (*http.Request, error) {
	request, errorValue := http.NewRequest(method, endpointURL, bytes.NewReader(body))
	if errorValue != nil {
		return nil, errorValue
	}
	if len(body) > 0 {
		request.Header.Set("Content-Type", "application/json")
	}
	return request, nil
}
