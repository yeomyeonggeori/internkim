package admind

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"time"

	"gitlab.com/eastriver/internkim/internal/capabilities"
)

const (
	companionWatchStatusOpen              = "open"
	companionWatchStatusClosed            = "closed"
	companionAttentionRemotePending       = "pending_remote"
	companionAttentionRemoteSilent        = "silent"
	companionAttentionRemoteMessage       = "message"
	companionAttentionRemoteFailed        = "remote_failed"
	companionAttentionRemoteStored        = "stored_no_reply_target"
	companionAttentionConfidenceThreshold = 0.6
	companionAttentionCooldown            = 30 * time.Minute
)

type CompanionAttentionState struct {
	LocalDecision *capabilities.AttentionTriageDecision `json:"localDecision,omitempty"`
	RemoteStatus  string                                `json:"remoteStatus,omitempty"`
	RemoteMessage string                                `json:"remoteMessage,omitempty"`
	RemoteReason  string                                `json:"remoteReason,omitempty"`
	RemoteError   string                                `json:"remoteError,omitempty"`
	UpdatedAt     time.Time                             `json:"updatedAt,omitempty"`
}

func initializeCompanionJobWatch(job *CompanionJob, now time.Time) {
	if !shouldWatchCompanionJob(job) {
		return
	}
	job.WatchStatus = companionWatchStatusOpen
	job.NextWatchAt = now.Add(companionWatchDelay(0))
}

func shouldWatchCompanionJob(job *CompanionJob) bool {
	if job == nil {
		return false
	}
	toolName := strings.TrimSpace(job.ToolName)
	return toolName == "user_confirm" ||
		toolName == "user.input" ||
		toolName == "file_pick" ||
		strings.HasPrefix(toolName, "filesystem.mount.")
}

func closeCompanionJobWatchLocked(job *CompanionJob, now time.Time) {
	if job == nil || job.WatchStatus == "" {
		return
	}
	job.WatchStatus = companionWatchStatusClosed
	job.NextWatchAt = time.Time{}
	job.UpdatedAt = now
}

func (service *Service) claimDueCompanionAttentionJobLocked(companion *CompanionRecord, now time.Time) (*CompanionJob, bool) {
	watchedJob := service.nextDueCompanionWatchLocked(companion, now)
	if watchedJob == nil {
		return nil, false
	}
	if !companionCanRunTool(companion, capabilities.AttentionTriageToolName) {
		watchedJob.WatchAttemptCount++
		watchedJob.NextWatchAt = time.Time{}
		service.applyCompanionAttentionFallbackLocked(watchedJob, now, "local_triage_unavailable")
		return nil, true
	}
	if service.hasActiveCompanionAttentionJobLocked(watchedJob) {
		return nil, false
	}
	attentionJob := service.createCompanionAttentionJobLocked(companion, watchedJob, now)
	return attentionJob, true
}

func (service *Service) nextDueCompanionWatchLocked(companion *CompanionRecord, now time.Time) *CompanionJob {
	var selectedJob *CompanionJob
	for _, job := range service.companionJobs {
		if !isDueCompanionWatch(job, now) {
			continue
		}
		if !service.companionCanClaimAttentionWatchLocked(companion, job) {
			continue
		}
		if selectedJob == nil || job.NextWatchAt.Before(selectedJob.NextWatchAt) {
			selectedJob = job
		}
	}
	return selectedJob
}

func isDueCompanionWatch(job *CompanionJob, now time.Time) bool {
	if job == nil || job.WatchStatus != companionWatchStatusOpen {
		return false
	}
	if job.Status != "pending" && job.Status != "running" {
		return false
	}
	return !job.NextWatchAt.IsZero() && !now.Before(job.NextWatchAt)
}

func (service *Service) companionCanClaimAttentionWatchLocked(companion *CompanionRecord, job *CompanionJob) bool {
	if companion == nil || job == nil {
		return false
	}
	if job.CompanionID != "" && job.CompanionID != companion.CompanionID {
		return false
	}
	if shouldUseRequesterOwnedCompanion(job.Request) && !companionOwnsJob(companion, job) {
		return false
	}
	if service.activeCompanionAttentionJobsForRequesterLocked(job) > 0 {
		return false
	}
	return true
}

func (service *Service) activeCompanionAttentionJobsForRequesterLocked(watchedJob *CompanionJob) int {
	activeJobCount := 0
	for _, job := range service.companionJobs {
		if job == nil || job.ToolName != capabilities.AttentionTriageToolName {
			continue
		}
		if job.Status != "pending" && job.Status != "running" {
			continue
		}
		if job.RequesterPersonID == watchedJob.RequesterPersonID && strings.EqualFold(job.RequesterEmail, watchedJob.RequesterEmail) {
			activeJobCount++
		}
	}
	return activeJobCount
}

func (service *Service) hasActiveCompanionAttentionJobLocked(watchedJob *CompanionJob) bool {
	for _, job := range service.companionJobs {
		if job == nil || job.ParentJobID != watchedJob.JobID || job.ToolName != capabilities.AttentionTriageToolName {
			continue
		}
		if job.Status == "pending" || job.Status == "running" {
			return true
		}
	}
	return false
}

func (service *Service) createCompanionAttentionJobLocked(companion *CompanionRecord, watchedJob *CompanionJob, now time.Time) *CompanionJob {
	watchedJob.WatchAttemptCount++
	watchedJob.NextWatchAt = time.Time{}
	watchedJob.UpdatedAt = now
	inputDocument, _ := json.Marshal(attentionTriageRequestFromJob(watchedJob))
	attentionRequest := capabilities.ToolInvokeRequest{
		ToolName:      capabilities.AttentionTriageToolName,
		Input:         inputDocument,
		Context:       watchedJob.Request.Context,
		ExecutionMode: capabilities.ExecutionModeCompanion,
		PrivacyClass:  "model_input",
		ParentJobID:   watchedJob.JobID,
		TimeoutSecond: 30,
	}
	attentionJob := &CompanionJob{
		JobID:             randomHex(16),
		ParentJobID:       watchedJob.JobID,
		Status:            "running",
		CompanionID:       companion.CompanionID,
		RequesterPersonID: watchedJob.RequesterPersonID,
		RequesterEmail:    watchedJob.RequesterEmail,
		ToolName:          capabilities.AttentionTriageToolName,
		PrivacyClass:      "model_input",
		Request:           attentionRequest,
		CreatedAt:         now,
		UpdatedAt:         now,
		ExpiresAt:         now.Add(30 * time.Second),
		Depth:             watchedJob.Depth + 1,
	}
	service.companionJobs[attentionJob.JobID] = attentionJob
	return attentionJob
}

func attentionTriageRequestFromJob(job *CompanionJob) capabilities.AttentionTriageRequest {
	request := capabilities.AttentionTriageRequest{
		JobID:             job.JobID,
		ParentJobID:       job.ParentJobID,
		ToolName:          job.ToolName,
		Status:            job.Status,
		RequesterPersonID: job.RequesterPersonID,
		RequesterEmail:    job.RequesterEmail,
		PrivacyClass:      job.PrivacyClass,
		ResourceScope:     job.ResourceScope,
		WatchAttemptCount: job.WatchAttemptCount,
		CreatedAt:         job.CreatedAt.Format(time.RFC3339),
		UpdatedAt:         job.UpdatedAt.Format(time.RFC3339),
		ExpiresAt:         job.ExpiresAt.Format(time.RFC3339),
		Error:             job.Error,
	}
	if !job.LastAttentionAt.IsZero() {
		request.LastAttentionAt = job.LastAttentionAt.Format(time.RFC3339)
	}
	if job.Denial != nil {
		request.DenialCode = job.Denial.Code
	}
	return request
}

func (service *Service) finishCompanionAttentionJobLocked(job *CompanionJob, response *capabilities.ToolInvokeResponse, errorMessage string, now time.Time) *capabilities.RemoteAttentionRequest {
	parentJob := service.companionJobs[job.ParentJobID]
	if parentJob == nil {
		return nil
	}
	if errorMessage != "" || response == nil {
		service.applyCompanionAttentionFallbackLocked(parentJob, now, "local_triage_failed")
		return nil
	}
	var decision capabilities.AttentionTriageDecision
	if errorValue := json.Unmarshal(response.Result, &decision); errorValue != nil {
		service.applyCompanionAttentionFallbackLocked(parentJob, now, "local_triage_invalid")
		return nil
	}
	service.applyCompanionAttentionDecisionLocked(parentJob, decision, now)
	if !service.shouldRequestRemoteAttentionLocked(parentJob, decision, now) {
		service.scheduleNextCompanionWatchLocked(parentJob, now)
		return nil
	}
	remoteRequest := remoteAttentionRequestFromJob(parentJob, decision)
	parentJob.AttentionKey = remoteRequest.DeduplicationKey
	parentJob.LastAttentionAt = now
	parentJob.Attention.RemoteStatus = companionAttentionRemotePending
	parentJob.Attention.UpdatedAt = now
	service.scheduleNextCompanionWatchLocked(parentJob, now)
	return &remoteRequest
}

func (service *Service) applyCompanionAttentionFallbackLocked(job *CompanionJob, now time.Time, reasonCode string) {
	if job == nil || job.ToolName == capabilities.AttentionTriageToolName {
		return
	}
	decision := capabilities.AttentionTriageDecision{
		ShouldEscalate:   job.WatchAttemptCount >= 2,
		Importance:       "low",
		Confidence:       0.7,
		ReasonCodes:      []string{reasonCode},
		SummaryForRemote: "Companion job " + job.JobID + " is still " + job.Status + ".",
		PrivacyClass:     job.PrivacyClass,
	}
	service.applyCompanionAttentionDecisionLocked(job, decision, now)
	service.scheduleNextCompanionWatchLocked(job, now)
}

func (service *Service) applyCompanionAttentionDecisionLocked(job *CompanionJob, decision capabilities.AttentionTriageDecision, now time.Time) {
	if job.Attention == nil {
		job.Attention = &CompanionAttentionState{}
	}
	job.Attention.LocalDecision = &decision
	job.Attention.UpdatedAt = now
	job.UpdatedAt = now
}

func (service *Service) shouldRequestRemoteAttentionLocked(job *CompanionJob, decision capabilities.AttentionTriageDecision, now time.Time) bool {
	if !decision.ShouldEscalate || decision.Confidence < companionAttentionConfidenceThreshold {
		return false
	}
	if !job.LastAttentionAt.IsZero() && now.Sub(job.LastAttentionAt) < companionAttentionCooldown {
		return false
	}
	return job.AttentionKey != attentionDeduplicationKey(job, decision)
}

func remoteAttentionRequestFromJob(job *CompanionJob, decision capabilities.AttentionTriageDecision) capabilities.RemoteAttentionRequest {
	return capabilities.RemoteAttentionRequest{
		JobID:             job.JobID,
		ToolName:          job.ToolName,
		RequesterPersonID: job.RequesterPersonID,
		RequesterEmail:    job.RequesterEmail,
		ConversationID:    job.Request.Context.ConversationID,
		Platform:          job.Request.Context.Platform,
		PrivacyClass:      decision.PrivacyClass,
		ResourceScope:     job.ResourceScope,
		LocalDecision:     decision,
		DeduplicationKey:  attentionDeduplicationKey(job, decision),
	}
}

func attentionDeduplicationKey(job *CompanionJob, decision capabilities.AttentionTriageDecision) string {
	return strings.Join([]string{
		job.JobID,
		job.ToolName,
		job.Status,
		strings.Join(decision.ReasonCodes, ","),
		decision.SummaryForRemote,
	}, "|")
}

func (service *Service) scheduleNextCompanionWatchLocked(job *CompanionJob, now time.Time) {
	if job == nil || job.WatchStatus != companionWatchStatusOpen {
		return
	}
	if job.Status != "pending" && job.Status != "running" {
		closeCompanionJobWatchLocked(job, now)
		return
	}
	job.NextWatchAt = now.Add(companionWatchDelay(job.WatchAttemptCount))
	job.UpdatedAt = now
}

func companionWatchDelay(attemptCount int) time.Duration {
	delays := []time.Duration{5 * time.Minute, 10 * time.Minute, 30 * time.Minute, 2 * time.Hour}
	if attemptCount < 0 {
		return delays[0]
	}
	if attemptCount >= len(delays) {
		return delays[len(delays)-1]
	}
	return delays[attemptCount]
}

func (service *Service) processRemoteAttention(contextValue context.Context, request capabilities.RemoteAttentionRequest) {
	if strings.TrimSpace(request.Platform) == "" || strings.TrimSpace(request.ConversationID) == "" {
		service.updateRemoteAttentionResult(request.JobID, companionAttentionRemoteStored, "", "no_reply_target", "")
		return
	}
	var response capabilities.RemoteAttentionResponse
	errorValue := service.blueclawJSONRequest(contextValue, http.MethodPost, "/admin/api/attention/run", request, &response)
	if errorValue != nil {
		service.updateRemoteAttentionResult(request.JobID, companionAttentionRemoteFailed, "", "", errorValue.Error())
		return
	}
	if response.Status == "ATTENTION_SILENT" {
		service.updateRemoteAttentionResult(request.JobID, companionAttentionRemoteSilent, "", response.Reason, "")
		return
	}
	service.updateRemoteAttentionResult(request.JobID, companionAttentionRemoteMessage, response.Message, response.Reason, "")
}

func (service *Service) updateRemoteAttentionResult(jobID string, status string, message string, reason string, errorMessage string) {
	service.mutex.Lock()
	job := service.companionJobs[jobID]
	if job != nil {
		if job.Attention == nil {
			job.Attention = &CompanionAttentionState{}
		}
		job.Attention.RemoteStatus = status
		job.Attention.RemoteMessage = message
		job.Attention.RemoteReason = reason
		job.Attention.RemoteError = errorMessage
		job.Attention.UpdatedAt = time.Now().UTC()
		job.UpdatedAt = job.Attention.UpdatedAt
	}
	service.mutex.Unlock()
	if errorValue := service.saveCompanionJobs(); errorValue != nil {
		log.Printf("companion attention result persistence failed: %v", errorValue)
	}
}

func companionHeartbeatCapabilities(payload companionHeartbeatRequest) []capabilities.Descriptor {
	descriptors := payload.Capabilities
	if !payload.LocalLLMAvailable {
		return descriptors
	}
	return appendMissingCompanionDescriptors(descriptors, capabilities.CompanionLLMDescriptors())
}

func appendMissingCompanionDescriptors(existingDescriptors []capabilities.Descriptor, addedDescriptors []capabilities.Descriptor) []capabilities.Descriptor {
	result := append([]capabilities.Descriptor{}, existingDescriptors...)
	for _, addedDescriptor := range addedDescriptors {
		if hasCompanionDescriptor(result, addedDescriptor.Name) {
			continue
		}
		result = append(result, addedDescriptor)
	}
	return result
}

func hasCompanionDescriptor(descriptors []capabilities.Descriptor, toolName string) bool {
	for _, descriptor := range descriptors {
		if descriptor.Name == toolName {
			return true
		}
	}
	return false
}
