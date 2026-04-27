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

	"github.com/anthropic-lab/internkim/internal/capabilities"
	companionruntime "github.com/anthropic-lab/internkim/internal/companion"
)

const companionOnlineWindow = 45 * time.Second

type CompanionPairingCode struct {
	Code      string    `json:"code"`
	CreatedAt time.Time `json:"createdAt"`
	ExpiresAt time.Time `json:"expiresAt"`
	UsedAt    time.Time `json:"usedAt,omitempty"`
}

type CompanionRecord struct {
	CompanionID  string                    `json:"companionID"`
	DisplayName  string                    `json:"displayName"`
	PublicKey    string                    `json:"publicKey"`
	TokenHash    string                    `json:"tokenHash"`
	Capabilities []capabilities.Descriptor `json:"capabilities"`
	LocalOnly    bool                      `json:"localOnly"`
	CreatedAt    time.Time                 `json:"createdAt"`
	LastSeenAt   time.Time                 `json:"lastSeenAt"`
	RevokedAt    time.Time                 `json:"revokedAt,omitempty"`
}

type CompanionStatus struct {
	CompanionID  string                    `json:"companionID"`
	DisplayName  string                    `json:"displayName"`
	Capabilities []capabilities.Descriptor `json:"capabilities"`
	LocalOnly    bool                      `json:"localOnly"`
	CreatedAt    time.Time                 `json:"createdAt"`
	LastSeenAt   time.Time                 `json:"lastSeenAt"`
	IsOnline     bool                      `json:"isOnline"`
}

type CompanionJob struct {
	JobID         string                           `json:"jobID"`
	ParentJobID   string                           `json:"parentJobID,omitempty"`
	GrantID       string                           `json:"grantID,omitempty"`
	Status        string                           `json:"status"`
	CompanionID   string                           `json:"companionID,omitempty"`
	ToolName      string                           `json:"toolName"`
	PrivacyClass  string                           `json:"privacyClass"`
	ResourceScope capabilities.ResourceScope       `json:"resourceScope,omitempty"`
	Depth         int                              `json:"depth"`
	Request       capabilities.ToolInvokeRequest   `json:"request"`
	Response      *capabilities.ToolInvokeResponse `json:"response,omitempty"`
	Denial        *capabilities.DenialResult       `json:"denial,omitempty"`
	Error         string                           `json:"error,omitempty"`
	CreatedAt     time.Time                        `json:"createdAt"`
	UpdatedAt     time.Time                        `json:"updatedAt"`
	ExpiresAt     time.Time                        `json:"expiresAt"`
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
	Capabilities []capabilities.Descriptor `json:"capabilities"`
	LocalOnly    bool                      `json:"localOnly"`
}

type companionPairingCodeResponse struct {
	Code      string    `json:"code"`
	ExpiresAt time.Time `json:"expiresAt"`
	DeepLink  string    `json:"deepLink"`
}

type companionStatusResponse struct {
	Companions []CompanionStatus `json:"companions"`
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

func (service *Service) createCompanionPairingCode(responseWriter http.ResponseWriter, request *http.Request) {
	code := randomPairingCode()
	now := time.Now().UTC()
	pairingCode := &CompanionPairingCode{
		Code:      code,
		CreatedAt: now,
		ExpiresAt: now.Add(10 * time.Minute),
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
		CompanionID:  companionID,
		DisplayName:  firstNonEmpty(payload.DisplayName, "Companion"),
		PublicKey:    payload.PublicKey,
		TokenHash:    companionTokenHash(token),
		Capabilities: payload.Capabilities,
		LocalOnly:    payload.LocalOnly,
		CreatedAt:    now,
		LastSeenAt:   now,
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
	if !service.hasOnlineCompanionForTool(request.ToolName) {
		return capabilities.ToolInvokeResponse{}, errors.New("companion unavailable for capability: " + request.ToolName)
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
		JobID:         randomHex(16),
		ParentJobID:   request.ParentJobID,
		GrantID:       request.GrantID,
		Status:        "pending",
		ToolName:      request.ToolName,
		PrivacyClass:  companionPrivacyClass(request),
		ResourceScope: companionResourceScope(request),
		Request:       normalizeCompanionJobRequest(request),
		CreatedAt:     now,
		UpdatedAt:     now,
		ExpiresAt:     now.Add(time.Duration(timeout) * time.Second),
	}
	job.Depth = service.nextCompanionJobDepth(job.ParentJobID)
	service.mutex.Lock()
	service.companionJobs[job.JobID] = job
	service.mutex.Unlock()

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
	defer service.mutex.Unlock()
	now := time.Now().UTC()
	for _, job := range service.companionJobs {
		if job.Status != "pending" || now.After(job.ExpiresAt) {
			continue
		}
		if !companionCanRunTool(companion, job.Request.ToolName) {
			continue
		}
		job.Status = "running"
		job.CompanionID = companion.CompanionID
		job.UpdatedAt = now
		return job
	}
	return nil
}

func (service *Service) finishCompanionJob(companionID string, jobID string, response *capabilities.ToolInvokeResponse, errorMessage string) error {
	service.mutex.Lock()
	defer service.mutex.Unlock()
	job := service.companionJobs[jobID]
	if job == nil {
		return errors.New("companion job not found")
	}
	if job.CompanionID != companionID {
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
	return nil
}

func (service *Service) denyCompanionJobResult(companionID string, jobID string, denial capabilities.DenialResult) error {
	service.mutex.Lock()
	defer service.mutex.Unlock()
	job := service.companionJobs[jobID]
	if job == nil {
		return errors.New("companion job not found")
	}
	if job.CompanionID != companionID {
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
			CompanionID:  companion.CompanionID,
			DisplayName:  companion.DisplayName,
			Capabilities: companion.Capabilities,
			LocalOnly:    companion.LocalOnly,
			CreatedAt:    companion.CreatedAt,
			LastSeenAt:   companion.LastSeenAt,
			IsOnline:     now.Sub(companion.LastSeenAt) <= companionOnlineWindow,
		})
	}
	return statuses
}

func (service *Service) hasOnlineCompanionForTool(toolName string) bool {
	for _, status := range service.companionStatuses() {
		if !status.IsOnline {
			continue
		}
		for _, descriptor := range status.Capabilities {
			if descriptor.Name == toolName {
				return true
			}
		}
	}
	return false
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
	case "browser.session.start", "browser.navigate", "browser.observe", "browser.screenshot":
		return capabilities.ResourceScope{Kind: "web_origin", Value: browserOriginFromInput(request.Input)}
	case "file.pick":
		return capabilities.ResourceScope{Kind: "file_root", Value: ""}
	default:
		return capabilities.ResourceScope{}
	}
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
