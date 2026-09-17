package companion

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"time"

	"gitlab.com/eastriver/internkim/internal/capabilities"
)

type JobExecutor interface {
	ExecuteJob(ctx context.Context, envelope JobEnvelope, request capabilities.ToolInvokeRequest) (capabilities.ToolInvokeResponse, error)
}

type HeartbeatRuntime interface {
	LocalLLMAvailable() bool
	RecordHeartbeat(errorValue error)
}

type JobRunner struct {
	DeviceClient           DeviceClient
	Executor               JobExecutor
	Runtime                HeartbeatRuntime
	PreferCompanionBrowser bool
	RunOnce                bool
	HeartbeatLoopInterval  time.Duration
	PostJobPollingBackoff  time.Duration
}

type jobPollResponse struct {
	capabilities.CompanionJobEnvelope
	Status string `json:"status"`
}

func (runner JobRunner) Run(ctx context.Context) error {
	for {
		if errorValue := runner.sendHeartbeat(); errorValue != nil {
			runner.recordHeartbeat(errorValue)
			return errorValue
		}
		runner.recordHeartbeat(nil)
		job, errorValue := runner.nextJob(ctx)
		if errorValue != nil {
			return errorValue
		}
		if job == nil {
			if runner.RunOnce {
				return nil
			}
			continue
		}
		executionError := runner.executeAndRouteJob(job)
		if runner.RunOnce {
			return executionError
		}
		if runner.PostJobPollingBackoff > 0 {
			time.Sleep(runner.PostJobPollingBackoff)
		}
	}
}

func (runner JobRunner) RunHeartbeatLoop(ctx context.Context) {
	interval := runner.HeartbeatLoopInterval
	if interval <= 0 {
		interval = 15 * time.Second
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			runner.recordHeartbeat(runner.sendHeartbeat())
		}
	}
}

func (runner JobRunner) sendHeartbeat() error {
	payload := runner.heartbeatPayload()
	endpoint := runner.DeviceClient.State.DeviceURL + "/_internkim/companion/heartbeat"
	return runner.DeviceClient.PostSignedJSON(endpoint, payload, &map[string]any{})
}

func (runner JobRunner) heartbeatPayload() map[string]any {
	payload := map[string]any{
		"capabilities":           runner.DeviceClient.State.Capabilities,
		"localOnly":              runner.DeviceClient.State.LocalOnly,
		"preferCompanionBrowser": runner.PreferCompanionBrowser,
	}
	if runner.Runtime != nil {
		payload["localLLMAvailable"] = runner.Runtime.LocalLLMAvailable()
	}
	return payload
}

func (runner JobRunner) nextJob(ctx context.Context) (*jobPollResponse, error) {
	endpoint := runner.DeviceClient.State.DeviceURL + "/_internkim/companion/jobs/next"
	request, errorValue := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if errorValue != nil {
		return nil, errorValue
	}
	for key, value := range CompanionHeaders(runner.DeviceClient.State) {
		request.Header.Set(key, value)
	}
	if errorValue := SignRequest(request, nil, runner.DeviceClient.PrivateKey); errorValue != nil {
		return nil, errorValue
	}
	response, errorValue := runner.DeviceClient.httpClient().Do(request)
	if errorValue != nil {
		return nil, errorValue
	}
	defer response.Body.Close()
	var job jobPollResponse
	if errorValue := DecodeJSONResponse(endpoint, response, &job); errorValue != nil {
		return nil, errorValue
	}
	if job.Status == "empty" || job.JobID == "" {
		return nil, nil
	}
	if errorValue := validateJobExpiration(&job); errorValue != nil {
		return nil, errorValue
	}
	return &job, nil
}

func (runner JobRunner) executeAndRouteJob(job *jobPollResponse) error {
	if runner.Executor == nil {
		return errors.New("companion job executor is not configured")
	}
	jobContext, cancelJobContext := contextForJob(job)
	response, executionError := runner.Executor.ExecuteJob(jobContext, executorEnvelopeForJob(job), job.Request)
	cancelJobContext()
	if executionError != nil {
		runner.routeJobError(job.JobID, executionError)
		return executionError
	}
	_ = runner.completeJob(job.JobID, response)
	return nil
}

func (runner JobRunner) routeJobError(jobID string, executionError error) {
	var denialError DenialError
	if errors.As(executionError, &denialError) {
		_ = runner.denyJob(jobID, denialError.Denial)
		return
	}
	_ = runner.failJob(jobID, executionError.Error())
}

func (runner JobRunner) completeJob(jobID string, response capabilities.ToolInvokeResponse) error {
	endpoint := runner.DeviceClient.State.DeviceURL + "/_internkim/companion/jobs/" + url.PathEscape(jobID) + "/complete"
	return runner.DeviceClient.PostSignedJSON(endpoint, response, &map[string]any{})
}

func (runner JobRunner) failJob(jobID string, errorMessage string) error {
	endpoint := runner.DeviceClient.State.DeviceURL + "/_internkim/companion/jobs/" + url.PathEscape(jobID) + "/fail"
	return runner.DeviceClient.PostSignedJSON(endpoint, map[string]string{"error": errorMessage}, &map[string]any{})
}

func (runner JobRunner) denyJob(jobID string, denial capabilities.DenialResult) error {
	endpoint := runner.DeviceClient.State.DeviceURL + "/_internkim/companion/jobs/" + url.PathEscape(jobID) + "/deny"
	return runner.DeviceClient.PostSignedJSON(endpoint, denial, &map[string]any{})
}

func (runner JobRunner) recordHeartbeat(errorValue error) {
	if runner.Runtime != nil {
		runner.Runtime.RecordHeartbeat(errorValue)
	}
}

func contextForJob(job *jobPollResponse) (context.Context, context.CancelFunc) {
	if job == nil || strings.TrimSpace(job.ExpiresAt) == "" {
		return context.WithCancel(context.Background())
	}
	expiresAt, errorValue := time.Parse(time.RFC3339Nano, strings.TrimSpace(job.ExpiresAt))
	if errorValue != nil {
		return context.WithCancel(context.Background())
	}
	deadline := expiresAt.Add(-250 * time.Millisecond)
	if time.Until(deadline) <= 0 {
		deadline = time.Now().Add(250 * time.Millisecond)
	}
	return context.WithDeadline(context.Background(), deadline)
}

func validateJobExpiration(job *jobPollResponse) error {
	if job == nil || strings.TrimSpace(job.ExpiresAt) == "" {
		return nil
	}
	_, errorValue := time.Parse(time.RFC3339Nano, strings.TrimSpace(job.ExpiresAt))
	return errorValue
}

func executorEnvelopeForJob(job *jobPollResponse) JobEnvelope {
	return JobEnvelope{
		JobID:         job.JobID,
		ParentJobID:   job.ParentJobID,
		GrantID:       job.GrantID,
		ToolName:      firstNonEmpty(job.ToolName, job.Request.ToolName),
		PrivacyClass:  job.PrivacyClass,
		ResourceScope: job.ResourceScope,
		Depth:         job.Depth,
	}
}
