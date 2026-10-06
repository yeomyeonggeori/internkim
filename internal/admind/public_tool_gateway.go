package admind

import (
	"bytes"
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/yeomyeonggeori/internkim/internal/capabilities"
	"github.com/yeomyeonggeori/internkim/pkg/capabilityprotocol"
)

const publicAPITokenStoreFilename = "api-tokens.json"

const (
	publicAPIPermissionRead   = "read"
	publicAPIPermissionWrite  = "write"
	publicAPIPermissionDelete = "delete"
)

const (
	publicAPIActorSourceToken             = "public_api_token"
	publicAPIActorSourceAssertedRequester = "public_api_requester"
)

func knownPublicAPIPermissions() []string {
	return []string{
		publicAPIPermissionRead,
		publicAPIPermissionWrite,
		publicAPIPermissionDelete,
	}
}

type publicAPITokenRecord struct {
	ID        string    `json:"id"`
	Label     string    `json:"label"`
	Email     string    `json:"email"`
	Name      string    `json:"name,omitempty"`
	PersonID  string    `json:"personID,omitempty"`
	TokenHash string    `json:"tokenHash"`
	Scopes    []string  `json:"scopes"`
	CreatedAt time.Time `json:"createdAt"`
	RevokedAt time.Time `json:"revokedAt,omitempty"`
}

type publicAPITokenCreateRequest struct {
	Label  string   `json:"label"`
	Scopes []string `json:"scopes"`
}

type publicAPITokenCreateResponse struct {
	Token  string                    `json:"token"`
	Actor  capabilities.ActorContext `json:"actor"`
	Record publicAPITokenSummary     `json:"record"`
}

type publicAPITokenSummary struct {
	ID        string    `json:"id"`
	Label     string    `json:"label"`
	Email     string    `json:"email"`
	Scopes    []string  `json:"scopes"`
	CreatedAt time.Time `json:"createdAt"`
}

type publicToolGatewayActor struct {
	Record publicAPITokenRecord
	Actor  capabilities.ActorContext
}

func (service *Service) handlePublicAPI(responseWriter http.ResponseWriter, request *http.Request) {
	path := strings.TrimPrefix(request.URL.Path, "/api/v1")
	if request.Method == http.MethodPost && path == "/tokens" {
		service.createPublicAPIToken(responseWriter, request)
		return
	}
	actor, ok := service.authenticatePublicAPIRequest(responseWriter, request)
	if !ok {
		return
	}
	switch {
	case request.Method == http.MethodGet && strings.HasPrefix(path, "/diagnostics/"):
		service.readPublicHostDiagnostics(responseWriter, request, actor)
	case request.Method == http.MethodPost && path == "/agent/messages":
		service.handleAgentMessage(responseWriter, request, actor)
	case request.Method == http.MethodGet && path == "/agent/replies":
		service.handleAgentReplies(responseWriter, request, actor)
	case request.Method == http.MethodGet && path == "/tools":
		service.writePublicTools(responseWriter, request, actor)
	case request.Method == http.MethodGet && strings.HasPrefix(path, "/tools/"):
		service.writePublicTool(responseWriter, request, actor, strings.TrimPrefix(path, "/tools/"))
	case request.Method == http.MethodPost && strings.HasPrefix(path, "/tools/") && strings.HasSuffix(path, "/invoke"):
		toolName := strings.TrimSuffix(strings.TrimPrefix(path, "/tools/"), "/invoke")
		service.invokePublicTool(responseWriter, request, actor, toolName)
	default:
		http.NotFound(responseWriter, request)
	}
}

func (service *Service) createPublicAPIToken(responseWriter http.ResponseWriter, request *http.Request) {
	actorEmail := service.webMemberActorEmail(request)
	if actorEmail == "" {
		http.Error(responseWriter, "member web session required", http.StatusForbidden)
		return
	}
	var payload publicAPITokenCreateRequest
	if request.Body != nil {
		if errorValue := json.NewDecoder(request.Body).Decode(&payload); errorValue != nil && errorValue != io.EOF {
			http.Error(responseWriter, "invalid request body", http.StatusBadRequest)
			return
		}
	}
	token, record, errorValue := service.issuePublicAPIToken(request.Context(), actorEmail, payload)
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusInternalServerError)
		return
	}
	service.writeJSON(responseWriter, publicAPITokenCreateResponse{
		Token:  token,
		Actor:  publicAPIActorContext(record, service.isTaskAdminEmail(request.Context(), actorEmail), publicAPIActorSourceToken),
		Record: publicAPITokenSummary{ID: record.ID, Label: record.Label, Email: record.Email, Scopes: record.Scopes, CreatedAt: record.CreatedAt},
	})
}

func (service *Service) issuePublicAPIToken(ctx context.Context, actorEmail string, payload publicAPITokenCreateRequest) (string, publicAPITokenRecord, error) {
	actor, found, errorValue := service.resolveUserActorByEmail(ctx, actorEmail)
	if errorValue != nil {
		return "", publicAPITokenRecord{}, errorValue
	}
	if !found || strings.TrimSpace(actor.UserID) == "" {
		return "", publicAPITokenRecord{}, fmt.Errorf("token owner is not active member")
	}
	service.mutex.Lock()
	defer service.mutex.Unlock()
	token := "ik_" + randomHex(32)
	now := time.Now().UTC()
	record := publicAPITokenRecord{
		ID:        "tok_" + randomHex(12),
		Label:     firstNonEmpty(strings.TrimSpace(payload.Label), "API token"),
		Email:     actor.Email,
		Name:      actor.Name,
		PersonID:  actor.UserID,
		TokenHash: publicAPITokenHash(token),
		Scopes:    normalizePublicAPIPermissions(payload.Scopes),
		CreatedAt: now,
	}
	records := service.readPublicAPITokenRecords()
	records = append(records, record)
	return token, record, service.writePublicAPITokenRecords(ctx, records)
}

func (service *Service) authenticatePublicAPIRequest(responseWriter http.ResponseWriter, request *http.Request) (publicToolGatewayActor, bool) {
	if requesterEmail := assertedRequesterEmail(request); requesterEmail != "" {
		return service.assertedPublicAPIActor(responseWriter, request, requesterEmail)
	}
	return service.authenticatePublicAPIToolRequest(responseWriter, request)
}

func (service *Service) assertedPublicAPIActor(responseWriter http.ResponseWriter, request *http.Request, requesterEmail string) (publicToolGatewayActor, bool) {
	actor, found, errorValue := service.resolveUserActorByEmail(request.Context(), requesterEmail)
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusBadGateway)
		return publicToolGatewayActor{}, false
	}
	if !found || strings.TrimSpace(actor.UserID) == "" {
		http.Error(responseWriter, "asserted requester is not active member", http.StatusForbidden)
		return publicToolGatewayActor{}, false
	}
	record := publicAPITokenRecord{
		Email:    actor.Email,
		Name:     actor.Name,
		PersonID: actor.UserID,
		Scopes:   []string{assertedRequesterPermission(request)},
	}
	return publicToolGatewayActor{
		Record: record,
		Actor:  publicAPIActorContext(record, actor.isAdmin(), publicAPIActorSourceAssertedRequester),
	}, true
}

func (service *Service) authenticatePublicAPIToolRequest(responseWriter http.ResponseWriter, request *http.Request) (publicToolGatewayActor, bool) {
	token := publicAPIBearerToken(request)
	if token == "" {
		http.Error(responseWriter, "bearer token required", http.StatusUnauthorized)
		return publicToolGatewayActor{}, false
	}
	record, found := service.lookupPublicAPITokenRecord(token)
	if !found {
		http.Error(responseWriter, "invalid bearer token", http.StatusUnauthorized)
		return publicToolGatewayActor{}, false
	}
	actor, found, errorValue := service.resolveUserActorByEmail(request.Context(), record.Email)
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusBadGateway)
		return publicToolGatewayActor{}, false
	}
	if !found || strings.TrimSpace(actor.UserID) == "" {
		http.Error(responseWriter, "token owner is not active member", http.StatusForbidden)
		return publicToolGatewayActor{}, false
	}
	record.PersonID = actor.UserID
	record.Name = actor.Name
	return publicToolGatewayActor{
		Record: record,
		Actor:  publicAPIActorContext(record, actor.isAdmin(), publicAPIActorSourceToken),
	}, true
}

func (service *Service) writePublicTools(responseWriter http.ResponseWriter, request *http.Request, actor publicToolGatewayActor) {
	descriptors, errorValue := service.publicToolDescriptorsForActor(request.Context(), actor)
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusBadGateway)
		return
	}
	service.writeJSON(responseWriter, map[string]any{"tools": descriptors})
}

func (service *Service) writePublicTool(responseWriter http.ResponseWriter, request *http.Request, actor publicToolGatewayActor, rawToolName string) {
	descriptor, ok, errorValue := service.publicToolDescriptor(request.Context(), strings.Trim(rawToolName, "/"))
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusBadGateway)
		return
	}
	if !ok || !publicToolAllowedForActor(descriptor, actor) {
		http.NotFound(responseWriter, request)
		return
	}
	service.writeJSON(responseWriter, descriptor)
}

func (service *Service) invokePublicTool(responseWriter http.ResponseWriter, request *http.Request, actor publicToolGatewayActor, rawToolName string) {
	toolName := strings.Trim(rawToolName, "/")
	descriptor, ok, errorValue := service.publicToolDescriptor(request.Context(), toolName)
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusBadGateway)
		return
	}
	if !ok || !publicToolAllowedForActor(descriptor, actor) {
		http.Error(responseWriter, "tool is not available for this token", http.StatusForbidden)
		return
	}
	toolRequest, errorValue := decodePublicToolInvokeRequest(request.Body)
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusBadRequest)
		return
	}
	toolRequest.ToolName = toolName
	toolRequest.Actor = actor.Actor
	toolRequest.Context = publicToolInvokeContext(actor.Actor)
	toolRequest.PrivacyClass = descriptor.PrivacyClass
	toolRequest.RequiresUserPresence = descriptor.RequiresUserPresence
	carriedFiles, refusalStatus, errorValue := service.carriedWorkspaceFiles(request.Context(), actor.Actor, toolRequest.Input)
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), refusalStatus)
		return
	}
	toolRequest.Transport.WorkspaceFiles = carriedFiles
	response, errorValue := service.invokeCapabilityTool(request.Context(), toolRequest)
	service.writeCapabilityResponse(responseWriter, response, errorValue)
}

func (service *Service) capabilitySocketClient() http.Client {
	return http.Client{
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, _ string, _ string) (net.Conn, error) {
				var dialer net.Dialer
				return dialer.DialContext(ctx, "unix", service.Configuration.CapabilitySocketPath)
			},
		},
	}
}

func (service *Service) fetchCapabilityRegistry(ctx context.Context) (capabilities.RegistryResponse, error) {
	httpRequest, errorValue := http.NewRequestWithContext(ctx, http.MethodGet, "http://internkim/v1/capabilities", nil)
	if errorValue != nil {
		return capabilities.RegistryResponse{}, errorValue
	}
	httpClient := service.capabilitySocketClient()
	httpResponse, errorValue := httpClient.Do(httpRequest)
	if errorValue != nil {
		return capabilities.RegistryResponse{}, errorValue
	}
	defer httpResponse.Body.Close()
	responseDocument, readError := io.ReadAll(httpResponse.Body)
	if readError != nil {
		return capabilities.RegistryResponse{}, readError
	}
	if httpResponse.StatusCode >= http.StatusBadRequest {
		return capabilities.RegistryResponse{}, fmt.Errorf("capability registry failed: %s", strings.TrimSpace(string(responseDocument)))
	}
	var registry capabilities.RegistryResponse
	if errorValue := json.Unmarshal(responseDocument, &registry); errorValue != nil {
		return capabilities.RegistryResponse{}, errorValue
	}
	return registry, nil
}

func (service *Service) invokeCapabilityTool(ctx context.Context, request capabilities.ToolInvokeRequest) (capabilities.ToolInvokeResponse, error) {
	document, errorValue := json.Marshal(request)
	if errorValue != nil {
		return capabilities.ToolInvokeResponse{}, errorValue
	}
	httpClient := service.capabilitySocketClient()
	requestURL := "http://internkim/v1/tools/" + request.ToolName + "/invoke"
	httpRequest, errorValue := http.NewRequestWithContext(ctx, http.MethodPost, requestURL, bytes.NewReader(document))
	if errorValue != nil {
		return capabilities.ToolInvokeResponse{}, errorValue
	}
	httpRequest.Header.Set("Content-Type", "application/json")
	httpResponse, errorValue := httpClient.Do(httpRequest)
	if errorValue != nil {
		return capabilities.ToolInvokeResponse{}, errorValue
	}
	defer httpResponse.Body.Close()
	responseDocument, readError := io.ReadAll(httpResponse.Body)
	if readError != nil {
		return capabilities.ToolInvokeResponse{}, readError
	}
	if httpResponse.StatusCode >= http.StatusBadRequest {
		return capabilities.ToolInvokeResponse{}, fmt.Errorf("capability invoke failed: %s", strings.TrimSpace(string(responseDocument)))
	}
	var response capabilities.ToolInvokeResponse
	if errorValue := json.Unmarshal(responseDocument, &response); errorValue != nil {
		return capabilities.ToolInvokeResponse{}, errorValue
	}
	return response, nil
}

func (service *Service) writeCapabilityResponse(responseWriter http.ResponseWriter, response capabilities.ToolInvokeResponse, errorValue error) {
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusBadGateway)
		return
	}
	service.writeJSON(responseWriter, publicToolInvokeAnswer(response))
}

func publicToolInvokeAnswer(response capabilities.ToolInvokeResponse) map[string]any {
	answer := map[string]any{
		"toolName": response.ToolName,
		"outcome":  response.Outcome,
		"result":   publicToolResultDocument(response),
	}
	if response.SelectedBackend != "" {
		answer["selectedBackend"] = response.SelectedBackend
	}
	if len(response.Effects) > 0 {
		answer["effects"] = response.Effects
	}
	if response.Message != "" {
		answer["message"] = response.Message
	}
	if response.ErrorCode != "" {
		answer["errorCode"] = response.ErrorCode
	}
	if response.FailureStage != "" {
		answer["failureStage"] = response.FailureStage
	}
	if response.Outcome == capabilities.ToolOutcomeFailed {
		answer["retryable"] = response.Retryable
	}
	return answer
}

func publicToolResultDocument(response capabilities.ToolInvokeResponse) json.RawMessage {
	if response.Outcome != capabilities.ToolOutcomeFailed || len(response.Result) == 0 {
		return response.Result
	}
	var document map[string]json.RawMessage
	if json.Unmarshal(response.Result, &document) != nil {
		return response.Result
	}
	repeated := map[string]string{
		"message":      response.Message,
		"errorCode":    response.ErrorCode,
		"failureStage": response.FailureStage,
	}
	for key, above := range repeated {
		var inside string
		if json.Unmarshal(document[key], &inside) == nil && inside == above {
			delete(document, key)
		}
	}
	delete(document, "retryable")
	delete(document, "safeRetry")
	deduped, errorValue := json.Marshal(document)
	if errorValue != nil {
		return response.Result
	}
	return deduped
}

func decodePublicToolInvokeRequest(reader io.Reader) (capabilities.ToolInvokeRequest, error) {
	var request capabilities.ToolInvokeRequest
	errorValue := decodeOptionalJSONBody(reader, &request)
	return request, errorValue
}

func decodeOptionalJSONBody(reader io.Reader, target any) error {
	if reader == nil {
		return nil
	}
	document, errorValue := io.ReadAll(reader)
	if errorValue != nil {
		return errorValue
	}
	if len(bytes.TrimSpace(document)) == 0 {
		return nil
	}
	return json.Unmarshal(document, target)
}

func publicToolInvokeContext(actor capabilities.ActorContext) capabilities.ToolInvokeContext {
	return capabilities.ToolInvokeContext{
		RequesterPersonID: actor.PersonID,
		RequesterEmail:    strings.ToLower(strings.TrimSpace(actor.Email)),
		RequesterName:     actor.DisplayName,
		TaskSource:        capabilities.TaskSourcePublicAPI,
	}
}

func (service *Service) exposableCapabilityDescriptors(ctx context.Context) ([]capabilities.Descriptor, error) {
	registry, errorValue := service.fetchCapabilityRegistry(ctx)
	if errorValue != nil {
		return nil, errorValue
	}
	return registry.DeviceCapabilities, nil
}

func (service *Service) publicToolDescriptorsForActor(ctx context.Context, actor publicToolGatewayActor) ([]capabilities.Descriptor, error) {
	descriptors, errorValue := service.exposableCapabilityDescriptors(ctx)
	if errorValue != nil {
		return nil, errorValue
	}
	allowedDescriptors := make([]capabilities.Descriptor, 0, len(descriptors))
	for _, descriptor := range descriptors {
		if publicToolAllowedForActor(descriptor, actor) {
			allowedDescriptors = append(allowedDescriptors, descriptor)
		}
	}
	return allowedDescriptors, nil
}

func (service *Service) publicToolDescriptor(ctx context.Context, toolName string) (capabilities.Descriptor, bool, error) {
	descriptors, errorValue := service.exposableCapabilityDescriptors(ctx)
	if errorValue != nil {
		return capabilities.Descriptor{}, false, errorValue
	}
	for _, descriptor := range descriptors {
		if descriptor.Name == toolName {
			return descriptor, true, nil
		}
	}
	return capabilities.Descriptor{}, false, nil
}

func publicToolAllowedForActor(descriptor capabilities.Descriptor, actor publicToolGatewayActor) bool {
	requiredPermission := publicToolPermissionForDescriptor(descriptor)
	return actorPermissionRank(actor) >= publicAPIPermissionRank(requiredPermission)
}

func actorPermissionRank(actor publicToolGatewayActor) int {
	highestRank := 0
	for _, permission := range actor.Actor.Scopes {
		if rank := publicAPIPermissionRank(permission); rank > highestRank {
			highestRank = rank
		}
	}
	return highestRank
}

func publicAPIPermissionRank(permission string) int {
	switch permission {
	case publicAPIPermissionDelete:
		return 3
	case publicAPIPermissionWrite:
		return 2
	default:
		return 1
	}
}

func publicToolPermissionForDescriptor(descriptor capabilities.Descriptor) string {
	switch descriptor.SideEffectClass {
	case capabilityprotocol.SideEffectRead, capabilityprotocol.SideEffectComputation:
		return ""
	case capabilityprotocol.SideEffectDestructive:
		return publicAPIPermissionDelete
	}
	if capabilityprotocol.IsSideEffectClass(descriptor.SideEffectClass) {
		return publicAPIPermissionWrite
	}
	return publicAPIPermissionDelete
}

func publicAPIActorContext(record publicAPITokenRecord, isAdmin bool, source string) capabilities.ActorContext {
	return capabilities.ActorContext{
		PersonID:    strings.TrimSpace(record.PersonID),
		Email:       strings.ToLower(strings.TrimSpace(record.Email)),
		DisplayName: strings.TrimSpace(record.Name),
		Source:      source,
		Scopes:      normalizePublicAPIPermissions(record.Scopes),
		IsAdmin:     isAdmin,
	}
}

func normalizePublicAPIPermissions(permissions []string) []string {
	known := knownPublicAPIPermissions()
	seen := map[string]bool{}
	normalizedPermissions := make([]string, 0, len(permissions)+1)
	for _, permission := range append([]string{publicAPIPermissionRead}, permissions...) {
		normalizedPermission := strings.ToLower(strings.TrimSpace(permission))
		if normalizedPermission == "" || seen[normalizedPermission] || !slices.Contains(known, normalizedPermission) {
			continue
		}
		seen[normalizedPermission] = true
		normalizedPermissions = append(normalizedPermissions, normalizedPermission)
	}
	return normalizedPermissions
}

func publicAPIBearerToken(request *http.Request) string {
	value := strings.TrimSpace(request.Header.Get("Authorization"))
	if !strings.HasPrefix(strings.ToLower(value), "bearer ") {
		return ""
	}
	return strings.TrimSpace(value[len("Bearer "):])
}

func (service *Service) lookupPublicAPITokenRecord(token string) (publicAPITokenRecord, bool) {
	tokenHash := publicAPITokenHash(token)
	for _, record := range service.readPublicAPITokenRecords() {
		if !record.RevokedAt.IsZero() {
			continue
		}
		if subtle.ConstantTimeCompare([]byte(record.TokenHash), []byte(tokenHash)) == 1 {
			return record, true
		}
	}
	return publicAPITokenRecord{}, false
}

func (service *Service) readPublicAPITokenRecords() []publicAPITokenRecord {
	document, errorValue := os.ReadFile(service.publicAPITokenStorePath())
	if errorValue != nil {
		return nil
	}
	var records []publicAPITokenRecord
	if errorValue := json.Unmarshal(document, &records); errorValue != nil {
		return nil
	}
	return records
}

func (service *Service) writePublicAPITokenRecords(ctx context.Context, records []publicAPITokenRecord) error {
	_ = ctx
	document, errorValue := json.MarshalIndent(records, "", "  ")
	if errorValue != nil {
		return errorValue
	}
	path := service.publicAPITokenStorePath()
	if errorValue := os.MkdirAll(filepath.Dir(path), 0o700); errorValue != nil {
		return errorValue
	}
	return writeFileAtomically(path, append(document, '\n'), 0o600)
}

func (service *Service) publicAPITokenStorePath() string {
	return filepath.Join(service.Configuration.StateDirectory, publicAPITokenStoreFilename)
}

func publicAPITokenHash(token string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(token)))
	return hex.EncodeToString(sum[:])
}
