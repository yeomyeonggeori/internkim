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

	"gitlab.com/eastriver/internkim/internal/capabilities"
)

const publicAPITokenStoreFilename = "api-tokens.json"

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
	actor, ok := service.authenticatePublicAPIToolRequest(responseWriter, request)
	if !ok {
		return
	}
	switch {
	case request.Method == http.MethodGet && path == "/tools":
		service.writePublicTools(responseWriter, actor)
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
	actorEmail := service.webStaffActorEmail(request)
	if actorEmail == "" {
		http.Error(responseWriter, "staff web session required", http.StatusForbidden)
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
		Actor:  publicAPIActorContext(record, service.isFlowAdminEmail(request.Context(), actorEmail)),
		Record: publicAPITokenSummary{ID: record.ID, Label: record.Label, Email: record.Email, Scopes: record.Scopes, CreatedAt: record.CreatedAt},
	})
}

func (service *Service) issuePublicAPIToken(ctx context.Context, actorEmail string, payload publicAPITokenCreateRequest) (string, publicAPITokenRecord, error) {
	actor, found, errorValue := service.resolveUserActorByEmail(ctx, actorEmail)
	if errorValue != nil {
		return "", publicAPITokenRecord{}, errorValue
	}
	if !found || strings.TrimSpace(actor.UserID) == "" {
		return "", publicAPITokenRecord{}, fmt.Errorf("token owner is not active staff")
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
		Scopes:    normalizePublicAPITokenScopes(payload.Scopes),
		CreatedAt: now,
	}
	records := service.readPublicAPITokenRecords()
	records = append(records, record)
	return token, record, service.writePublicAPITokenRecords(ctx, records)
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
		http.Error(responseWriter, "token owner is not active staff", http.StatusForbidden)
		return publicToolGatewayActor{}, false
	}
	record.PersonID = actor.UserID
	record.Name = actor.Name
	return publicToolGatewayActor{Record: record, Actor: publicAPIActorContext(record, actor.isAdmin())}, true
}

func (service *Service) writePublicTools(responseWriter http.ResponseWriter, actor publicToolGatewayActor) {
	descriptors := publicToolDescriptorsForActor(actor)
	service.writeJSON(responseWriter, map[string]any{"tools": descriptors})
}

func (service *Service) writePublicTool(responseWriter http.ResponseWriter, request *http.Request, actor publicToolGatewayActor, rawToolName string) {
	descriptor, ok := publicToolDescriptor(strings.Trim(rawToolName, "/"))
	if !ok || !publicToolAllowedForActor(descriptor, actor) {
		http.NotFound(responseWriter, request)
		return
	}
	service.writeJSON(responseWriter, descriptor)
}

func (service *Service) invokePublicTool(responseWriter http.ResponseWriter, request *http.Request, actor publicToolGatewayActor, rawToolName string) {
	toolName := strings.Trim(rawToolName, "/")
	descriptor, ok := publicToolDescriptor(toolName)
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
	toolRequest.Context = publicToolInvokeContext(actor.Actor, descriptor)
	toolRequest.PrivacyClass = descriptor.PrivacyClass
	toolRequest.RequiresUserPresence = descriptor.RequiresUserPresence
	response, errorValue := service.invokeCapabilityTool(request.Context(), toolRequest)
	service.writeCapabilityResponse(responseWriter, response, errorValue)
}

func (service *Service) invokeCapabilityTool(ctx context.Context, request capabilities.ToolInvokeRequest) (capabilities.ToolInvokeResponse, error) {
	document, errorValue := json.Marshal(request)
	if errorValue != nil {
		return capabilities.ToolInvokeResponse{}, errorValue
	}
	httpClient := http.Client{
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, network string, address string) (net.Conn, error) {
				_ = network
				_ = address
				var dialer net.Dialer
				return dialer.DialContext(ctx, "unix", service.Configuration.CapabilitySocketPath)
			},
		},
		Timeout: 90 * time.Second,
	}
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
	service.writeJSON(responseWriter, response)
}

func decodePublicToolInvokeRequest(reader io.Reader) (capabilities.ToolInvokeRequest, error) {
	var request capabilities.ToolInvokeRequest
	if reader == nil {
		return request, nil
	}
	document, errorValue := io.ReadAll(reader)
	if errorValue != nil {
		return capabilities.ToolInvokeRequest{}, errorValue
	}
	if len(bytes.TrimSpace(document)) == 0 {
		return request, nil
	}
	if errorValue := json.Unmarshal(document, &request); errorValue != nil {
		return capabilities.ToolInvokeRequest{}, errorValue
	}
	return request, nil
}

func publicToolInvokeContext(actor capabilities.ActorContext, descriptor capabilities.Descriptor) capabilities.ToolInvokeContext {
	return capabilities.ToolInvokeContext{
		RequesterPersonID:      actor.PersonID,
		RequesterEmail:         strings.ToLower(strings.TrimSpace(actor.Email)),
		RequesterName:          actor.DisplayName,
		TaskSource:             "public_api",
		Platform:               "public_api",
		IsApprovalContinuation: publicToolScopeForDescriptor(descriptor) != "",
	}
}

func publicToolDescriptorsForActor(actor publicToolGatewayActor) []capabilities.Descriptor {
	descriptors := publicToolDescriptors()
	allowedDescriptors := make([]capabilities.Descriptor, 0, len(descriptors))
	for _, descriptor := range descriptors {
		if publicToolAllowedForActor(descriptor, actor) {
			allowedDescriptors = append(allowedDescriptors, descriptor)
		}
	}
	return allowedDescriptors
}

func publicToolDescriptor(toolName string) (capabilities.Descriptor, bool) {
	for _, descriptor := range publicToolDescriptors() {
		if descriptor.Name == toolName {
			return descriptor, true
		}
	}
	return capabilities.Descriptor{}, false
}

func publicToolDescriptors() []capabilities.Descriptor {
	names := map[string]bool{
		"flow.task.add":              true,
		"flow.task.list":             true,
		"flow.task.update":           true,
		"flow.task.delete":           true,
		"calendar.event.add":         true,
		"calendar.event.list":        true,
		"calendar.event.update":      true,
		"calendar.event.delete":      true,
		"calendar.connection.status": true,
		"calendar.connection.start":  true,
		"mail.connection.status":     true,
		"mail.connection.start":      true,
		"mail.message.list":          true,
		"mail.message.search":        true,
		"mail.message.read":          true,
		"mail.message.send":          true,
		"platform.message.context":   true,
		"platform.message.search":    true,
		"platform.message.send":      true,
	}
	descriptors := make([]capabilities.Descriptor, 0, len(names))
	for _, descriptor := range capabilities.DeviceDescriptors() {
		if names[descriptor.Name] {
			descriptors = append(descriptors, descriptor)
		}
	}
	return descriptors
}

func publicToolAllowedForActor(descriptor capabilities.Descriptor, actor publicToolGatewayActor) bool {
	requiredScope := publicToolScopeForDescriptor(descriptor)
	return requiredScope == "" || slices.Contains(actor.Actor.Scopes, requiredScope)
}

func publicToolScopeForDescriptor(descriptor capabilities.Descriptor) string {
	switch descriptor.SideEffectClass {
	case "read":
		return ""
	case "workspace_write", "workspace_calendar", "workspace_task":
		return "write"
	case "external_send":
		return "external_send"
	case "connect":
		return "connect"
	case "destructive":
		return "destructive"
	default:
		if descriptor.RequiresApproval {
			return "write"
		}
		return "write"
	}
}

func publicAPIActorContext(record publicAPITokenRecord, isAdmin bool) capabilities.ActorContext {
	return capabilities.ActorContext{
		PersonID:    strings.TrimSpace(record.PersonID),
		Email:       strings.ToLower(strings.TrimSpace(record.Email)),
		DisplayName: strings.TrimSpace(record.Name),
		Source:      "public_api_token",
		Scopes:      normalizePublicAPITokenScopes(record.Scopes),
		IsAdmin:     isAdmin,
	}
}

func normalizePublicAPITokenScopes(scopes []string) []string {
	seen := map[string]bool{}
	normalizedScopes := make([]string, 0, len(scopes)+1)
	for _, scope := range append([]string{"read"}, scopes...) {
		normalizedScope := strings.ToLower(strings.TrimSpace(scope))
		if normalizedScope == "" || seen[normalizedScope] {
			continue
		}
		seen[normalizedScope] = true
		normalizedScopes = append(normalizedScopes, normalizedScope)
	}
	return normalizedScopes
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
