package admind

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gitlab.com/eastriver/internkim/internal/capabilities"
	companionruntime "gitlab.com/eastriver/internkim/internal/companion"
	"gitlab.com/eastriver/internkim/internal/runtime/blueclaw"
)

const companionOnlineWindow = 45 * time.Second

type CompanionPairingCode struct {
	Code          string    `json:"code"`
	OwnerPersonID string    `json:"ownerPersonID,omitempty"`
	OwnerEmail    string    `json:"ownerEmail,omitempty"`
	CreatedAt     time.Time `json:"createdAt"`
	ExpiresAt     time.Time `json:"expiresAt"`
	UsedAt        time.Time `json:"usedAt,omitempty"`
}

type CompanionRecord struct {
	CompanionID   string                    `json:"companionID"`
	DisplayName   string                    `json:"displayName"`
	OwnerPersonID string                    `json:"ownerPersonID,omitempty"`
	OwnerEmail    string                    `json:"ownerEmail,omitempty"`
	PublicKey     string                    `json:"publicKey"`
	TokenHash     string                    `json:"tokenHash"`
	Capabilities  []capabilities.Descriptor `json:"capabilities"`
	LocalOnly     bool                      `json:"localOnly"`
	CreatedAt     time.Time                 `json:"createdAt"`
	LastSeenAt    time.Time                 `json:"lastSeenAt"`
	RevokedAt     time.Time                 `json:"revokedAt,omitempty"`
}

type CompanionStatus struct {
	CompanionID   string                    `json:"companionID"`
	DisplayName   string                    `json:"displayName"`
	OwnerPersonID string                    `json:"ownerPersonID,omitempty"`
	OwnerEmail    string                    `json:"ownerEmail,omitempty"`
	Capabilities  []capabilities.Descriptor `json:"capabilities"`
	LocalOnly     bool                      `json:"localOnly"`
	CreatedAt     time.Time                 `json:"createdAt"`
	LastSeenAt    time.Time                 `json:"lastSeenAt"`
	IsOnline      bool                      `json:"isOnline"`
}

type CompanionJob struct {
	JobID             string                           `json:"jobID"`
	ParentJobID       string                           `json:"parentJobID,omitempty"`
	GrantID           string                           `json:"grantID,omitempty"`
	Status            string                           `json:"status"`
	CompanionID       string                           `json:"companionID,omitempty"`
	RequesterPersonID string                           `json:"requesterPersonID,omitempty"`
	RequesterEmail    string                           `json:"requesterEmail,omitempty"`
	ToolName          string                           `json:"toolName"`
	PrivacyClass      string                           `json:"privacyClass"`
	ResourceScope     capabilities.ResourceScope       `json:"resourceScope,omitempty"`
	Depth             int                              `json:"depth"`
	Request           capabilities.ToolInvokeRequest   `json:"request"`
	Response          *capabilities.ToolInvokeResponse `json:"response,omitempty"`
	Denial            *capabilities.DenialResult       `json:"denial,omitempty"`
	Error             string                           `json:"error,omitempty"`
	CreatedAt         time.Time                        `json:"createdAt"`
	UpdatedAt         time.Time                        `json:"updatedAt"`
	ExpiresAt         time.Time                        `json:"expiresAt"`
}

type companionPairRequest struct {
	Code         string                    `json:"code"`
	DisplayName  string                    `json:"displayName"`
	PublicKey    string                    `json:"publicKey"`
	Capabilities []capabilities.Descriptor `json:"capabilities"`
	LocalOnly    bool                      `json:"localOnly"`
}

type companionPairResponse struct {
	CompanionID string `json:"companionID"`
	Token       string `json:"token"`
}

type companionHeartbeatRequest struct {
	Capabilities           []capabilities.Descriptor `json:"capabilities"`
	LocalOnly              bool                      `json:"localOnly"`
	LocalLLMAvailable      bool                      `json:"localLLMAvailable,omitempty"`
	PreferCompanionBrowser bool                      `json:"preferCompanionBrowser,omitempty"`
	Mounts                 []CompanionMountSnapshot  `json:"mounts,omitempty"`
}

type companionPairingCodeResponse struct {
	Code      string    `json:"code"`
	ExpiresAt time.Time `json:"expiresAt"`
	DeepLink  string    `json:"deepLink"`
}

type companionStatusResponse struct {
	Companions []CompanionStatus `json:"companions"`
}

type companionRemoteModelResponse struct {
	Model       string `json:"model"`
	Restarted   bool   `json:"restarted,omitempty"`
	UpdatedAt   string `json:"updatedAt,omitempty"`
	RuntimePath string `json:"runtimePath,omitempty"`
}

type companionRemoteModelUpdateRequest struct {
	Model string `json:"model"`
}

func (service *Service) handleCompanion(responseWriter http.ResponseWriter, request *http.Request) {
	path := strings.TrimPrefix(request.URL.Path, "/_internkim/companion")
	switch {
	case request.Method == http.MethodPost && path == "/pair":
		service.pairCompanion(responseWriter, request)
	case request.Method == http.MethodPost && path == "/heartbeat":
		service.companionHeartbeat(responseWriter, request)
	case request.Method == http.MethodGet && path == "/capabilities":
		service.writeLocalCompanionCapabilities(responseWriter, request)
	case request.Method == http.MethodGet && path == "/auth/check":
		service.checkCompanionAuth(responseWriter, request)
	case request.Method == http.MethodGet && path == "/remote-model":
		service.readCompanionRemoteModel(responseWriter, request)
	case request.Method == http.MethodPut && path == "/remote-model":
		service.updateCompanionRemoteModel(responseWriter, request)
	case request.Method == http.MethodGet && path == "/jobs/next":
		service.nextCompanionJob(responseWriter, request)
	case request.Method == http.MethodPost && path == "/jobs":
		service.createCompanionJob(responseWriter, request)
	case request.Method == http.MethodPost && path == "/files/uploads":
		service.createCompanionFileUpload(responseWriter, request)
	case request.Method == http.MethodPut && strings.HasPrefix(path, "/files/uploads/") && strings.Contains(path, "/chunks/"):
		service.writeCompanionFileUploadChunk(responseWriter, request, path)
	case request.Method == http.MethodPost && strings.HasPrefix(path, "/files/uploads/") && strings.HasSuffix(path, "/complete"):
		service.completeCompanionFileUpload(responseWriter, request, path)
	case request.Method == http.MethodPost && strings.HasPrefix(path, "/jobs/") && strings.HasSuffix(path, "/complete"):
		service.completeCompanionJob(responseWriter, request, strings.TrimSuffix(strings.TrimPrefix(path, "/jobs/"), "/complete"))
	case request.Method == http.MethodPost && strings.HasPrefix(path, "/jobs/") && strings.HasSuffix(path, "/fail"):
		service.failCompanionJob(responseWriter, request, strings.TrimSuffix(strings.TrimPrefix(path, "/jobs/"), "/fail"))
	case request.Method == http.MethodPost && strings.HasPrefix(path, "/jobs/") && strings.HasSuffix(path, "/deny"):
		service.denyCompanionJob(responseWriter, request, strings.TrimSuffix(strings.TrimPrefix(path, "/jobs/"), "/deny"))
	default:
		http.NotFound(responseWriter, request)
	}
}

func (service *Service) checkCompanionAuth(responseWriter http.ResponseWriter, request *http.Request) {
	companion := service.authorizedCompanion(request)
	if companion == nil {
		http.Error(responseWriter, "companion auth required", http.StatusForbidden)
		return
	}
	service.writeJSON(responseWriter, map[string]string{
		"status":      "ok",
		"companionID": companion.CompanionID,
	})
}

func (service *Service) readCompanionRemoteModel(responseWriter http.ResponseWriter, request *http.Request) {
	if companion := service.authorizedCompanion(request); companion == nil {
		http.Error(responseWriter, "companion auth required", http.StatusForbidden)
		return
	}
	model, errorValue := service.readRemoteModel()
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusInternalServerError)
		return
	}
	service.writeJSON(responseWriter, companionRemoteModelResponse{
		Model:       model,
		RuntimePath: service.blueclawRuntimeConfigPath(),
	})
}

func (service *Service) updateCompanionRemoteModel(responseWriter http.ResponseWriter, request *http.Request) {
	if companion := service.authorizedCompanion(request); companion == nil {
		http.Error(responseWriter, "companion auth required", http.StatusForbidden)
		return
	}
	var payload companionRemoteModelUpdateRequest
	if errorValue := json.NewDecoder(request.Body).Decode(&payload); errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusBadRequest)
		return
	}
	model := strings.TrimSpace(payload.Model)
	if model == "" {
		http.Error(responseWriter, "model is required", http.StatusBadRequest)
		return
	}
	if errorValue := service.writeRemoteModel(request.Context(), model); errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusInternalServerError)
		return
	}
	service.writeJSON(responseWriter, companionRemoteModelResponse{
		Model:       model,
		Restarted:   true,
		UpdatedAt:   time.Now().UTC().Format(time.RFC3339),
		RuntimePath: service.blueclawRuntimeConfigPath(),
	})
}

func (service *Service) readRemoteModel() (string, error) {
	document, errorValue := service.readBlueclawRuntimeDocument()
	if errorValue != nil {
		return "", errorValue
	}
	return remoteModelFromRuntimeDocument(document), nil
}

func (service *Service) writeRemoteModel(ctx context.Context, model string) error {
	document, errorValue := service.readBlueclawRuntimeDocument()
	if errorValue != nil {
		return errorValue
	}
	setRemoteModelInRuntimeDocument(document, model)
	if errorValue := service.writeBlueclawRuntimeDocument(document); errorValue != nil {
		return errorValue
	}
	_, errorValue = service.runCommand(ctx, "systemctl", "restart", blueclaw.BlueclawServiceName)
	return errorValue
}

func (service *Service) readBlueclawRuntimeDocument() (map[string]any, error) {
	documentBytes, errorValue := os.ReadFile(service.blueclawRuntimeConfigPath())
	if errorValue != nil {
		return nil, errorValue
	}
	var document map[string]any
	if errorValue := json.Unmarshal(documentBytes, &document); errorValue != nil {
		return nil, errorValue
	}
	return document, nil
}

func (service *Service) writeBlueclawRuntimeDocument(document map[string]any) error {
	documentBytes, errorValue := json.MarshalIndent(document, "", "  ")
	if errorValue != nil {
		return errorValue
	}
	path := service.blueclawRuntimeConfigPath()
	if errorValue := os.WriteFile(path, append(documentBytes, '\n'), 0o640); errorValue != nil {
		return errorValue
	}
	return nil
}

func (service *Service) blueclawRuntimeConfigPath() string {
	if strings.TrimSpace(service.Configuration.BlueclawRuntimeConfigPath) != "" {
		return service.Configuration.BlueclawRuntimeConfigPath
	}
	return blueclaw.BlueclawRuntimeConfigPath
}

func remoteModelFromRuntimeDocument(document map[string]any) string {
	languageModel, _ := document["languageModel"].(map[string]any)
	capabilityModel, _ := languageModel["capability"].(map[string]any)
	model, _ := capabilityModel["model"].(string)
	return strings.TrimSpace(model)
}

func setRemoteModelInRuntimeDocument(document map[string]any, model string) {
	languageModel, _ := document["languageModel"].(map[string]any)
	if languageModel == nil {
		languageModel = map[string]any{}
		document["languageModel"] = languageModel
	}
	capabilityModel, _ := languageModel["capability"].(map[string]any)
	if capabilityModel == nil {
		capabilityModel = map[string]any{}
		languageModel["capability"] = capabilityModel
	}
	capabilityModel["model"] = strings.TrimSpace(model)
}

func (service *Service) createCompanionPairingCode(responseWriter http.ResponseWriter, request *http.Request) {
	code := randomPairingCode()
	now := time.Now().UTC()
	pairingCode := &CompanionPairingCode{
		Code:       code,
		OwnerEmail: service.companionPairingOwnerEmail(request),
		CreatedAt:  now,
		ExpiresAt:  now.Add(10 * time.Minute),
	}
	service.mutex.Lock()
	service.pairingCodes[code] = pairingCode
	service.mutex.Unlock()

	service.writeJSON(responseWriter, companionPairingCodeResponse{
		Code:      code,
		ExpiresAt: pairingCode.ExpiresAt,
		DeepLink:  companionDeepLink(request, code),
	})
}

func (service *Service) writeCompanionStatus(responseWriter http.ResponseWriter, request *http.Request) {
	_ = request
	service.writeJSON(responseWriter, companionStatusResponse{Companions: service.companionStatuses()})
}

func (service *Service) companionPairingOwnerEmail(request *http.Request) string {
	return firstNonEmpty(
		authenticatedCallerEmail(request),
		service.claimedAdminEmail(),
		service.seedAdminEmail(),
	)
}

func (service *Service) revokeCompanion(responseWriter http.ResponseWriter, request *http.Request, companionID string) {
	_ = request
	service.mutex.Lock()
	companion := service.companions[companionID]
	if companion == nil {
		service.mutex.Unlock()
		http.NotFound(responseWriter, request)
		return
	}
	companion.RevokedAt = time.Now().UTC()
	service.mutex.Unlock()
	_ = service.saveCompanions()
	service.writeJSON(responseWriter, map[string]string{"status": "revoked"})
}

func (service *Service) pairCompanion(responseWriter http.ResponseWriter, request *http.Request) {
	var payload companionPairRequest
	if errorValue := json.NewDecoder(request.Body).Decode(&payload); errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusBadRequest)
		return
	}
	code := strings.TrimSpace(payload.Code)
	now := time.Now().UTC()

	service.mutex.Lock()
	pairingCode := service.pairingCodes[code]
	if pairingCode == nil || !pairingCode.UsedAt.IsZero() || now.After(pairingCode.ExpiresAt) {
		service.mutex.Unlock()
		http.Error(responseWriter, "pairing code is invalid or expired", http.StatusForbidden)
		return
	}
	pairingCode.UsedAt = now
	companionID := randomHex(16)
	token := randomHex(32)
	service.companions[companionID] = &CompanionRecord{
		CompanionID:   companionID,
		DisplayName:   firstNonEmpty(payload.DisplayName, "Companion"),
		OwnerPersonID: pairingCode.OwnerPersonID,
		OwnerEmail:    pairingCode.OwnerEmail,
		PublicKey:     payload.PublicKey,
		TokenHash:     companionTokenHash(token),
		Capabilities:  payload.Capabilities,
		LocalOnly:     payload.LocalOnly,
		CreatedAt:     now,
		LastSeenAt:    now,
	}
	service.mutex.Unlock()

	if errorValue := service.saveCompanions(); errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusInternalServerError)
		return
	}
	service.writeJSON(responseWriter, companionPairResponse{CompanionID: companionID, Token: token})
}

func (service *Service) companionHeartbeat(responseWriter http.ResponseWriter, request *http.Request) {
	companion := service.authorizedCompanion(request)
	if companion == nil {
		http.Error(responseWriter, "companion auth required", http.StatusForbidden)
		return
	}
	var payload companionHeartbeatRequest
	if errorValue := json.NewDecoder(request.Body).Decode(&payload); errorValue != nil && !errors.Is(errorValue, io.EOF) {
		http.Error(responseWriter, errorValue.Error(), http.StatusBadRequest)
		return
	}

	service.mutex.Lock()
	storedCompanion := service.companions[companion.CompanionID]
	storedCompanion.LastSeenAt = time.Now().UTC()
	if len(payload.Capabilities) > 0 {
		storedCompanion.Capabilities = payload.Capabilities
	}
	storedCompanion.LocalOnly = payload.LocalOnly
	service.mutex.Unlock()
	service.updateCompanionMounts(companion.CompanionID, payload.Mounts)
	_ = service.saveCompanions()
	service.writeJSON(responseWriter, map[string]string{"status": "ok"})
}

func (service *Service) writeLocalCompanionCapabilities(responseWriter http.ResponseWriter, request *http.Request) {
	if !isLocalRequest(request) {
		http.Error(responseWriter, "local access required", http.StatusForbidden)
		return
	}
	service.writeJSON(responseWriter, service.localCompanionCapabilities())
}

func (service *Service) nextCompanionJob(responseWriter http.ResponseWriter, request *http.Request) {
	companion := service.authorizedCompanion(request)
	if companion == nil {
		http.Error(responseWriter, "companion auth required", http.StatusForbidden)
		return
	}
	deadline := time.Now().Add(25 * time.Second)
	for {
		job := service.claimNextCompanionJob(companion)
		if job != nil {
			service.writeJSON(responseWriter, job)
			return
		}
		if time.Now().After(deadline) {
			service.writeJSON(responseWriter, map[string]string{"status": "empty"})
			return
		}
		time.Sleep(250 * time.Millisecond)
	}
}

func (service *Service) createCompanionJob(responseWriter http.ResponseWriter, request *http.Request) {
	if !isLocalRequest(request) {
		http.Error(responseWriter, "local access required", http.StatusForbidden)
		return
	}
	var payload capabilities.ToolInvokeRequest
	if errorValue := json.NewDecoder(request.Body).Decode(&payload); errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusBadRequest)
		return
	}
	response, errorValue := service.invokeCompanionJob(request.Context(), payload)
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusBadGateway)
		return
	}
	service.writeJSON(responseWriter, response)
}

func (service *Service) completeCompanionJob(responseWriter http.ResponseWriter, request *http.Request, jobID string) {
	companion := service.authorizedCompanion(request)
	if companion == nil {
		http.Error(responseWriter, "companion auth required", http.StatusForbidden)
		return
	}
	var payload capabilities.ToolInvokeResponse
	if errorValue := json.NewDecoder(request.Body).Decode(&payload); errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusBadRequest)
		return
	}
	if errorValue := service.finishCompanionJob(companion.CompanionID, jobID, &payload, ""); errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusForbidden)
		return
	}
	service.writeJSON(responseWriter, map[string]string{"status": "completed"})
}

func (service *Service) failCompanionJob(responseWriter http.ResponseWriter, request *http.Request, jobID string) {
	companion := service.authorizedCompanion(request)
	if companion == nil {
		http.Error(responseWriter, "companion auth required", http.StatusForbidden)
		return
	}
	var payload struct {
		Error string `json:"error"`
	}
	if errorValue := json.NewDecoder(request.Body).Decode(&payload); errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusBadRequest)
		return
	}
	if errorValue := service.finishCompanionJob(companion.CompanionID, jobID, nil, firstNonEmpty(payload.Error, "companion job failed")); errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusForbidden)
		return
	}
	service.writeJSON(responseWriter, map[string]string{"status": "failed"})
}

func (service *Service) denyCompanionJob(responseWriter http.ResponseWriter, request *http.Request, jobID string) {
	companion := service.authorizedCompanion(request)
	if companion == nil {
		http.Error(responseWriter, "companion auth required", http.StatusForbidden)
		return
	}
	var payload capabilities.DenialResult
	if errorValue := json.NewDecoder(request.Body).Decode(&payload); errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusBadRequest)
		return
	}
	if errorValue := service.denyCompanionJobResult(companion.CompanionID, jobID, payload); errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusForbidden)
		return
	}
	service.writeJSON(responseWriter, map[string]string{"status": "denied"})
}

func (service *Service) invokeCompanionJob(ctx context.Context, request capabilities.ToolInvokeRequest) (capabilities.ToolInvokeResponse, error) {
	if errorValue := validateCompanionToolRequest(request); errorValue != nil {
		return capabilities.ToolInvokeResponse{}, errorValue
	}
	request.ResourceScope = service.inferCompanionResourceScope(request)
	if errorValue := service.validateCompanionMountRequest(request); errorValue != nil {
		return capabilities.ToolInvokeResponse{}, errorValue
	}
	if errorValue := service.validateRequesterForCompanionTool(request); errorValue != nil {
		return capabilities.ToolInvokeResponse{}, errorValue
	}
	if !service.hasOnlineCompanionForRequest(request) {
		return capabilities.ToolInvokeResponse{}, errors.New("requester's companion unavailable for capability: " + request.ToolName)
	}
	now := time.Now().UTC()
	timeout := request.TimeoutSecond
	if timeout <= 0 {
		timeout = 30
	}
	if timeout > 120 {
		timeout = 120
	}
	job := &CompanionJob{
		JobID:             randomHex(16),
		ParentJobID:       request.ParentJobID,
		GrantID:           request.GrantID,
		Status:            "pending",
		RequesterPersonID: strings.TrimSpace(request.Context.RequesterPersonID),
		RequesterEmail:    strings.ToLower(strings.TrimSpace(request.Context.RequesterEmail)),
		ToolName:          request.ToolName,
		PrivacyClass:      companionPrivacyClass(request),
		ResourceScope:     companionResourceScope(request),
		Request:           normalizeCompanionJobRequest(request),
		CreatedAt:         now,
		UpdatedAt:         now,
		ExpiresAt:         now.Add(time.Duration(timeout) * time.Second),
	}
	job.Depth = service.nextCompanionJobDepth(job.ParentJobID)
	service.mutex.Lock()
	service.companionJobs[job.JobID] = job
	service.mutex.Unlock()
	_ = service.saveCompanionJobs()

	deadline := time.Now().Add(time.Duration(timeout) * time.Second)
	for {
		select {
		case <-ctx.Done():
			return capabilities.ToolInvokeResponse{}, ctx.Err()
		default:
		}
		service.mutex.Lock()
		currentJob := service.companionJobs[job.JobID]
		if currentJob != nil && currentJob.Status == "completed" && currentJob.Response != nil {
			response := *currentJob.Response
			service.mutex.Unlock()
			return response, nil
		}
		if currentJob != nil && currentJob.Status == "failed" {
			errorMessage := currentJob.Error
			service.mutex.Unlock()
			return capabilities.ToolInvokeResponse{}, errors.New(errorMessage)
		}
		if currentJob != nil && currentJob.Status == "denied" && currentJob.Denial != nil {
			response, errorValue := companionDenialResponse(*currentJob.Denial)
			service.mutex.Unlock()
			return response, errorValue
		}
		if currentJob != nil && time.Now().After(currentJob.ExpiresAt) {
			currentJob.Status = "expired"
			currentJob.Error = "companion job expired"
			currentJob.UpdatedAt = time.Now().UTC()
			service.mutex.Unlock()
			_ = service.saveCompanionJobs()
			return capabilities.ToolInvokeResponse{}, errors.New("companion job expired")
		}
		service.mutex.Unlock()
		if time.Now().After(deadline) {
			return capabilities.ToolInvokeResponse{}, errors.New("companion job timed out")
		}
		time.Sleep(100 * time.Millisecond)
	}
}

func (service *Service) claimNextCompanionJob(companion *CompanionRecord) *CompanionJob {
	service.mutex.Lock()
	now := time.Now().UTC()
	recoveredJobs := service.recoverExpiredAndStaleCompanionJobsLocked(now)
	for _, job := range service.companionJobs {
		if job.Status != "pending" || now.After(job.ExpiresAt) {
			continue
		}
		if !companionCanRunTool(companion, job.Request.ToolName) {
			continue
		}
		if requiresRequesterOwnedCompanion(job.Request.ToolName) && !companionOwnsJob(companion, job) {
			continue
		}
		if !service.companionCanClaimMountJobLocked(companion, job) {
			continue
		}
		job.Status = "running"
		job.CompanionID = companion.CompanionID
		job.UpdatedAt = now
		service.mutex.Unlock()
		_ = service.saveCompanionJobs()
		return job
	}
	service.mutex.Unlock()
	if recoveredJobs {
		_ = service.saveCompanionJobs()
	}
	return nil
}

func (service *Service) finishCompanionJob(companionID string, jobID string, response *capabilities.ToolInvokeResponse, errorMessage string) error {
	service.mutex.Lock()
	job := service.companionJobs[jobID]
	if job == nil {
		service.mutex.Unlock()
		return errors.New("companion job not found")
	}
	if job.CompanionID != companionID {
		service.mutex.Unlock()
		return errors.New("companion job owner mismatch")
	}
	if errorMessage != "" {
		job.Status = "failed"
		job.Error = errorMessage
	} else {
		job.Status = "completed"
		job.Response = response
	}
	job.UpdatedAt = time.Now().UTC()
	service.mutex.Unlock()
	service.updateCompanionMountsFromJob(companionID, jobID, response)
	_ = service.saveCompanionJobs()
	return nil
}

func (service *Service) denyCompanionJobResult(companionID string, jobID string, denial capabilities.DenialResult) error {
	service.mutex.Lock()
	job := service.companionJobs[jobID]
	if job == nil {
		service.mutex.Unlock()
		return errors.New("companion job not found")
	}
	if job.CompanionID != companionID {
		service.mutex.Unlock()
		return errors.New("companion job owner mismatch")
	}
	denial.Status = firstNonEmpty(denial.Status, "denied")
	denial.Code = firstNonEmpty(denial.Code, "user_denied")
	denial.JobID = firstNonEmpty(denial.JobID, job.JobID)
	denial.ToolName = firstNonEmpty(denial.ToolName, job.ToolName)
	if denial.ResourceScope.Kind == "" && denial.ResourceScope.Value == "" {
		denial.ResourceScope = job.ResourceScope
	}
	denial.UserReason = sanitizeCompanionDenialText(denial.UserReason)
	denial.SuggestedConstraint = sanitizeCompanionDenialText(denial.SuggestedConstraint)
	job.Status = "denied"
	job.Denial = &denial
	job.UpdatedAt = time.Now().UTC()
	service.mutex.Unlock()
	_ = service.saveCompanionJobs()
	return nil
}

func (service *Service) authorizedCompanion(request *http.Request) *CompanionRecord {
	companionID := strings.TrimSpace(request.Header.Get("X-InternKim-Companion-ID"))
	token := strings.TrimSpace(request.Header.Get("X-InternKim-Companion-Token"))
	if companionID == "" || token == "" {
		return nil
	}
	service.mutex.Lock()
	defer service.mutex.Unlock()
	companion := service.companions[companionID]
	if companion == nil || !companion.RevokedAt.IsZero() {
		return nil
	}
	if companion.TokenHash != companionTokenHash(token) {
		return nil
	}
	body, _ := io.ReadAll(request.Body)
	request.Body = io.NopCloser(bytes.NewReader(body))
	if !companionruntime.VerifyRequestSignature(request, body, companion.PublicKey) {
		return nil
	}
	return companion
}

func (service *Service) companionStatuses() []CompanionStatus {
	service.mutex.Lock()
	defer service.mutex.Unlock()
	statuses := []CompanionStatus{}
	now := time.Now().UTC()
	for _, companion := range service.companions {
		if !companion.RevokedAt.IsZero() {
			continue
		}
		statuses = append(statuses, CompanionStatus{
			CompanionID:   companion.CompanionID,
			DisplayName:   companion.DisplayName,
			OwnerPersonID: companion.OwnerPersonID,
			OwnerEmail:    companion.OwnerEmail,
			Capabilities:  companion.Capabilities,
			LocalOnly:     companion.LocalOnly,
			CreatedAt:     companion.CreatedAt,
			LastSeenAt:    companion.LastSeenAt,
			IsOnline:      now.Sub(companion.LastSeenAt) <= companionOnlineWindow,
		})
	}
	return statuses
}

func (service *Service) hasOnlineCompanionForRequest(request capabilities.ToolInvokeRequest) bool {
	for _, status := range service.companionStatuses() {
		if !status.IsOnline {
			continue
		}
		if requiresRequesterOwnedCompanion(request.ToolName) && !companionStatusMatchesRequester(status, request) {
			continue
		}
		for _, descriptor := range status.Capabilities {
			if descriptor.Name == request.ToolName {
				return true
			}
		}
	}
	return false
}

func (service *Service) validateRequesterForCompanionTool(request capabilities.ToolInvokeRequest) error {
	if !requiresRequesterOwnedCompanion(request.ToolName) {
		return nil
	}
	if strings.TrimSpace(request.Context.RequesterPersonID) != "" || strings.TrimSpace(request.Context.RequesterEmail) != "" {
		return nil
	}
	return errors.New("requester identity is required for user-local companion capability: " + request.ToolName)
}

func companionStatusMatchesRequester(status CompanionStatus, request capabilities.ToolInvokeRequest) bool {
	requesterPersonID := strings.TrimSpace(request.Context.RequesterPersonID)
	requesterEmail := strings.ToLower(strings.TrimSpace(request.Context.RequesterEmail))
	if requesterPersonID != "" && strings.TrimSpace(status.OwnerPersonID) == requesterPersonID {
		return true
	}
	if requesterEmail != "" && strings.EqualFold(status.OwnerEmail, requesterEmail) {
		return true
	}
	return requesterPersonID == "" && requesterEmail == "" && status.OwnerPersonID == "" && status.OwnerEmail == ""
}

func companionOwnsJob(companion *CompanionRecord, job *CompanionJob) bool {
	if companion == nil || job == nil {
		return false
	}
	if strings.TrimSpace(job.RequesterPersonID) != "" && strings.TrimSpace(companion.OwnerPersonID) == strings.TrimSpace(job.RequesterPersonID) {
		return true
	}
	if strings.TrimSpace(job.RequesterEmail) != "" && strings.EqualFold(companion.OwnerEmail, job.RequesterEmail) {
		return true
	}
	return job.RequesterPersonID == "" && job.RequesterEmail == "" && companion.OwnerPersonID == "" && companion.OwnerEmail == ""
}

func requiresRequesterOwnedCompanion(toolName string) bool {
	trimmedToolName := strings.TrimSpace(toolName)
	return strings.HasPrefix(trimmedToolName, "browser.") || strings.HasPrefix(trimmedToolName, "user.") || trimmedToolName == "file.pick"
}

func companionCanRunTool(companion *CompanionRecord, toolName string) bool {
	if companion == nil {
		return false
	}
	for _, descriptor := range companion.Capabilities {
		if descriptor.Name == toolName {
			return true
		}
	}
	return false
}

func (service *Service) nextCompanionJobDepth(parentJobID string) int {
	if strings.TrimSpace(parentJobID) == "" {
		return 0
	}
	service.mutex.Lock()
	defer service.mutex.Unlock()
	parentJob := service.companionJobs[parentJobID]
	if parentJob == nil {
		return 0
	}
	return parentJob.Depth + 1
}

func (service *Service) recoverExpiredAndStaleCompanionJobsLocked(now time.Time) bool {
	changed := false
	for _, job := range service.companionJobs {
		if job == nil {
			continue
		}
		if now.After(job.ExpiresAt) && (job.Status == "pending" || job.Status == "running") {
			job.Status = "expired"
			job.Error = "companion job expired"
			job.UpdatedAt = now
			changed = true
			continue
		}
		if job.Status != "running" {
			continue
		}
		companion := service.companions[job.CompanionID]
		if companion == nil || !companion.RevokedAt.IsZero() || now.Sub(companion.LastSeenAt) > companionOnlineWindow {
			job.Status = "pending"
			job.CompanionID = ""
			job.UpdatedAt = now
			changed = true
		}
	}
	return changed
}

func normalizeCompanionJobRequest(request capabilities.ToolInvokeRequest) capabilities.ToolInvokeRequest {
	request.SessionID = ""
	request.PrivacyClass = companionPrivacyClass(request)
	request.ResourceScope = companionResourceScope(request)
	return request
}

func companionPrivacyClass(request capabilities.ToolInvokeRequest) string {
	if strings.TrimSpace(request.PrivacyClass) != "" {
		return strings.TrimSpace(request.PrivacyClass)
	}
	for _, descriptor := range capabilities.CompanionToolDescriptors() {
		if descriptor.Name == request.ToolName {
			return descriptor.PrivacyClass
		}
	}
	return ""
}

func validateCompanionToolRequest(request capabilities.ToolInvokeRequest) error {
	descriptor, ok := companionToolDescriptor(request.ToolName)
	if !ok {
		return errors.New("companion capability is not configured: " + request.ToolName)
	}
	if strings.TrimSpace(request.PrivacyClass) != "" && strings.TrimSpace(request.PrivacyClass) != descriptor.PrivacyClass {
		return errors.New("companion capability privacy class mismatch")
	}
	return nil
}

func companionToolDescriptor(toolName string) (capabilities.Descriptor, bool) {
	for _, descriptor := range capabilities.CompanionToolDescriptors() {
		if descriptor.Name == toolName {
			return descriptor, true
		}
	}
	for _, descriptor := range capabilities.CompanionLLMDescriptors() {
		if descriptor.Name == toolName {
			return descriptor, true
		}
	}
	return capabilities.Descriptor{}, false
}

func companionResourceScope(request capabilities.ToolInvokeRequest) capabilities.ResourceScope {
	if request.ResourceScope.Kind != "" || request.ResourceScope.Value != "" {
		return request.ResourceScope
	}
	switch request.ToolName {
	case "browser.open", "browser.snapshot", "browser.screenshot", "browser.handoff", "browser.click", "browser.fill", "browser.select", "browser.press", "browser.wait":
		return capabilities.ResourceScope{Kind: "web_origin", Value: browserOriginFromInput(request.Input)}
	case "file.pick":
		return capabilities.ResourceScope{Kind: "file_root", Value: ""}
	case "filesystem.mount.create", "filesystem.mount.list", "filesystem.mount.pause", "filesystem.mount.resume", "filesystem.mount.revoke", "filesystem.mount.status", "filesystem.mount.stat", "filesystem.mount.list_directory", "filesystem.mount.read", "filesystem.mount.write", "filesystem.mount.mkdir", "filesystem.mount.rename", "filesystem.mount.delete", "filesystem.mount.truncate", "filesystem.mount.chmod", "filesystem.mount.watch":
		return companionMountResourceScope(request)
	default:
		return capabilities.ResourceScope{}
	}
}

func (service *Service) inferCompanionResourceScope(request capabilities.ToolInvokeRequest) capabilities.ResourceScope {
	resourceScope := companionResourceScope(request)
	if resourceScope.Kind != "web_origin" || strings.TrimSpace(resourceScope.Value) != "" {
		return resourceScope
	}
	parentResourceScope := service.parentCompanionResourceScope(request.ParentJobID)
	if parentResourceScope.Kind == "web_origin" && strings.TrimSpace(parentResourceScope.Value) != "" {
		return parentResourceScope
	}
	return resourceScope
}

func (service *Service) parentCompanionResourceScope(parentJobID string) capabilities.ResourceScope {
	if strings.TrimSpace(parentJobID) == "" {
		return capabilities.ResourceScope{}
	}
	service.mutex.Lock()
	defer service.mutex.Unlock()
	parentJob := service.companionJobs[parentJobID]
	if parentJob == nil {
		return capabilities.ResourceScope{}
	}
	return parentJob.ResourceScope
}

func browserOriginFromInput(document json.RawMessage) string {
	var input struct {
		URL      string `json:"url"`
		StartURL string `json:"startURL"`
	}
	if errorValue := json.Unmarshal(document, &input); errorValue != nil {
		return ""
	}
	rawURL := firstNonEmpty(input.URL, input.StartURL)
	parsedURL, errorValue := url.Parse(rawURL)
	if errorValue != nil || parsedURL.Scheme == "" || parsedURL.Host == "" {
		return ""
	}
	return parsedURL.Scheme + "://" + parsedURL.Host
}

func companionDenialResponse(denial capabilities.DenialResult) (capabilities.ToolInvokeResponse, error) {
	document, errorValue := json.Marshal(denial)
	if errorValue != nil {
		return capabilities.ToolInvokeResponse{}, errorValue
	}
	return capabilities.ToolInvokeResponse{
		Provider: "companion",
		ToolName: denial.ToolName,
		Status:   "denied",
		Result:   document,
	}, nil
}

func sanitizeCompanionDenialText(value string) string {
	trimmedValue := strings.TrimSpace(value)
	if len(trimmedValue) > 240 {
		return trimmedValue[:240]
	}
	return trimmedValue
}

func (service *Service) saveCompanions() error {
	service.mutex.Lock()
	companions := []*CompanionRecord{}
	for _, companion := range service.companions {
		companions = append(companions, companion)
	}
	service.mutex.Unlock()
	document, errorValue := json.MarshalIndent(map[string]any{"companions": companions}, "", "  ")
	if errorValue != nil {
		return errorValue
	}
	if errorValue := os.MkdirAll(service.Configuration.StateDirectory, 0o700); errorValue != nil {
		return errorValue
	}
	path := filepath.Join(service.Configuration.StateDirectory, "companions.json")
	tempPath := path + ".tmp"
	if errorValue := os.WriteFile(tempPath, document, 0o600); errorValue != nil {
		return errorValue
	}
	return os.Rename(tempPath, path)
}

func (service *Service) loadCompanions() {
	path := filepath.Join(service.Configuration.StateDirectory, "companions.json")
	document, errorValue := os.ReadFile(path)
	if errorValue != nil {
		return
	}
	var state struct {
		Companions []*CompanionRecord `json:"companions"`
	}
	if errorValue := json.Unmarshal(document, &state); errorValue != nil {
		return
	}
	service.mutex.Lock()
	defer service.mutex.Unlock()
	for _, companion := range state.Companions {
		if companion != nil && companion.CompanionID != "" {
			service.companions[companion.CompanionID] = companion
		}
	}
}

func (service *Service) saveCompanionJobs() error {
	service.mutex.Lock()
	jobs := []*CompanionJob{}
	for _, job := range service.companionJobs {
		if job != nil {
			jobs = append(jobs, job)
		}
	}
	service.mutex.Unlock()
	document, errorValue := json.MarshalIndent(map[string]any{"jobs": jobs}, "", "  ")
	if errorValue != nil {
		return errorValue
	}
	path := service.Configuration.CompanionJobPath
	if errorValue := os.MkdirAll(filepath.Dir(path), 0o700); errorValue != nil {
		return errorValue
	}
	temporaryPath := path + ".tmp"
	if errorValue := os.WriteFile(temporaryPath, document, 0o600); errorValue != nil {
		return errorValue
	}
	return os.Rename(temporaryPath, path)
}

func (service *Service) loadCompanionJobs() {
	document, errorValue := os.ReadFile(service.Configuration.CompanionJobPath)
	if errorValue != nil {
		return
	}
	var state struct {
		Jobs []*CompanionJob `json:"jobs"`
	}
	if errorValue := json.Unmarshal(document, &state); errorValue != nil {
		return
	}
	now := time.Now().UTC()
	service.mutex.Lock()
	defer service.mutex.Unlock()
	for _, job := range state.Jobs {
		if job == nil || job.JobID == "" {
			continue
		}
		if now.After(job.ExpiresAt) && (job.Status == "pending" || job.Status == "running") {
			job.Status = "expired"
			job.Error = firstNonEmpty(job.Error, "companion job expired")
			job.UpdatedAt = now
		}
		if job.Status == "running" {
			job.Status = "pending"
			job.CompanionID = ""
			job.UpdatedAt = now
		}
		service.companionJobs[job.JobID] = job
	}
}

func (service *Service) localCompanionCapabilities() capabilities.RegistryResponse {
	statuses := service.companionStatuses()
	descriptorsByName := map[string]capabilities.Descriptor{}
	for _, status := range statuses {
		if !status.IsOnline {
			continue
		}
		for _, descriptor := range status.Capabilities {
			descriptorsByName[descriptor.Name] = descriptor
		}
	}
	descriptors := []capabilities.Descriptor{}
	for _, descriptor := range descriptorsByName {
		descriptors = append(descriptors, descriptor)
	}
	status := "unavailable"
	if len(descriptors) > 0 {
		status = "available"
	}
	return capabilities.RegistryResponse{
		CompanionStatus: status,
		Capabilities:    descriptors,
	}
}

func companionDeepLink(request *http.Request, code string) string {
	scheme := "https"
	if request.TLS == nil && strings.HasPrefix(request.Host, "127.") {
		scheme = "http"
	}
	deviceURL := scheme + "://" + request.Host
	return "internkim://pair?device_url=" + url.QueryEscape(deviceURL) + "&code=" + url.QueryEscape(code)
}

func companionTokenHash(token string) string {
	hash := sha256.Sum256([]byte(token))
	return hex.EncodeToString(hash[:])
}

func randomPairingCode() string {
	value := strings.ToUpper(randomHex(4))
	return value[:4] + "-" + value[4:]
}
