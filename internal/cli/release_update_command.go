package cli

import (
	"errors"
	"fmt"
	"strings"
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
	api, errorValue := reachDeviceReleaseAPI(target)
	if errorValue != nil {
		return errorValue
	}
	job, errorValue := api.applyRelease("", defaultReleaseUpdateChannel)
	if errorValue != nil {
		return errorValue
	}
	fmt.Printf("Job: %s\n", job.JobID)
	_, errorValue = waitForReleaseUpdateJob(api, job, "")
	return errorValue
}

func fetchDeviceReleaseUpdateStatus(arguments []string) (releaseUpdateStatusResponse, error) {
	target := resolveCommandTarget(arguments)
	return fetchDeviceReleaseUpdateStatusForTarget(target)
}

func releaseDeviceEndpointURL(target commandTarget, endpointPath string) (string, error) {
	deviceURL := deviceURLForTarget(target)
	if deviceURL == "" {
		return "", errors.New("device URL is not configured; run setup or pass a saved target")
	}
	return publicEndpointURL(deviceURL, endpointPath)
}

func deviceURLForTarget(target commandTarget) string {
	return strings.TrimSpace(firstNonEmptyString(target.deviceURL, loadState(target.stateDir, "device_url")))
}

func fetchDeviceReleaseUpdateStatusForTarget(target commandTarget) (releaseUpdateStatusResponse, error) {
	api, errorValue := reachDeviceReleaseAPI(target)
	if errorValue != nil {
		return releaseUpdateStatusResponse{}, errorValue
	}
	return api.releaseUpdateStatus()
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
