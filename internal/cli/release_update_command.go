package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"
)

type releaseUpdateStatusResponse struct {
	Current       *releaseUpdateSummary      `json:"current,omitempty"`
	Latest        *releaseUpdateSummary      `json:"latest,omitempty"`
	State         string                     `json:"state"`
	UpdateAllowed bool                       `json:"updateAllowed"`
	ActiveJob     *blueclawUpdateJobResponse `json:"activeJob,omitempty"`
}

type releaseUpdateSummary struct {
	ReleaseID  string                           `json:"releaseID"`
	Channel    string                           `json:"channel"`
	CreatedAt  string                           `json:"createdAt"`
	Components map[string]releaseComponentBrief `json:"components,omitempty"`
}

type releaseComponentBrief struct {
	Revision string `json:"revision"`
	SHA256   string `json:"sha256,omitempty"`
}

func runReleaseUpdateCheck(arguments []string) error {
	status, errorValue := fetchDeviceReleaseUpdateStatus(arguments)
	if errorValue != nil {
		return errorValue
	}
	printReleaseUpdateStatus(status)
	return nil
}

func runReleaseUpdateApply(arguments []string) error {
	target := resolveCommandTarget(arguments)
	requestDocument, errorValue := releaseUpdateApplyRequestDocument(target)
	if errorValue != nil {
		return errorValue
	}
	admin, errorValue := reachDeviceAdmin(target)
	if errorValue != nil {
		return errorValue
	}
	var job blueclawUpdateJobResponse
	if errorValue := admin.ask(http.MethodPost, "/admin/api/updates/apply", requestDocument, &job); errorValue != nil {
		return errorValue
	}
	fmt.Printf("Job: %s\n", job.JobID)
	for attempt := 0; attempt < 120; attempt++ {
		errorValue = admin.ask(http.MethodGet, "/admin/api/updates/jobs/"+job.JobID, nil, &job)
		if errorValue != nil {
			time.Sleep(1500 * time.Millisecond)
			continue
		}
		fmt.Printf("Status: %s/%s\n", job.Status, job.Phase)
		if job.Status == "completed" || job.Status == "already_current" {
			return nil
		}
		if job.Status == "failed" {
			return errors.New(strings.TrimSpace(job.Error))
		}
		time.Sleep(1500 * time.Millisecond)
	}
	return errors.New("release update did not finish before timeout")
}

func releaseUpdateApplyRequestDocument(target commandTarget) ([]byte, error) {
	fleetID := strings.TrimSpace(loadState(target.stateDir, "fleet_id"))
	fleetSecret := strings.TrimSpace(loadState(target.stateDir, "fleet_secret"))
	if fleetID == "" || fleetSecret == "" {
		return nil, errors.New("fleet identity is not configured in local device state")
	}
	return json.Marshal(signedRecoveryRequestPayload(fleetSecret, "release-update-apply", fleetID))
}

func fetchDeviceReleaseUpdateStatus(arguments []string) (releaseUpdateStatusResponse, error) {
	target := resolveCommandTarget(arguments)
	return fetchDeviceReleaseUpdateStatusForTarget(target)
}

func releaseDeviceEndpointURL(target commandTarget, endpointPath string) (string, error) {
	deviceURL := strings.TrimSpace(firstNonEmptyString(target.deviceURL, loadState(target.stateDir, "device_url")))
	if deviceURL == "" {
		return "", errors.New("device URL is not configured; run setup or pass a saved target")
	}
	return publicEndpointURL(deviceURL, endpointPath)
}

func fetchDeviceReleaseUpdateJob(target commandTarget, jobID string) (blueclawUpdateJobResponse, error) {
	admin, errorValue := reachDeviceAdmin(target)
	if errorValue != nil {
		return blueclawUpdateJobResponse{}, errorValue
	}
	var job blueclawUpdateJobResponse
	return job, admin.ask(http.MethodGet, "/admin/api/updates/jobs/"+jobID, nil, &job)
}

func fetchDeviceReleaseUpdateStatusForTarget(target commandTarget) (releaseUpdateStatusResponse, error) {
	admin, errorValue := reachDeviceAdmin(target)
	if errorValue != nil {
		return releaseUpdateStatusResponse{}, errorValue
	}
	var status releaseUpdateStatusResponse
	return status, admin.ask(http.MethodGet, "/admin/api/updates/status", nil, &status)
}

func printReleaseUpdateStatus(status releaseUpdateStatusResponse) {
	fmt.Printf("State: %s\n", status.State)
	fmt.Printf("Current: %s\n", releaseUpdateID(status.Current))
	fmt.Printf("Latest: %s\n", releaseUpdateID(status.Latest))
	if status.ActiveJob != nil {
		fmt.Printf("Job: %s %s/%s\n", status.ActiveJob.JobID, status.ActiveJob.Status, status.ActiveJob.Phase)
	}
}

func releaseUpdateID(summary *releaseUpdateSummary) string {
	if summary == nil || strings.TrimSpace(summary.ReleaseID) == "" {
		return "none"
	}
	return summary.ReleaseID
}

func readAllLimitedResponse(response *http.Response, limit int64) ([]byte, error) {
	return readAllLimited(response.Body, limit)
}
