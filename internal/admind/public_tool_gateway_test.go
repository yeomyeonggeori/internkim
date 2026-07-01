package admind

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"gitlab.com/eastriver/internkim/internal/capabilities"
)

func TestPublicToolGatewayOverridesActorFromBearerToken(t *testing.T) {
	service := newFlowAuthorizationTestService(t)
	service.Configuration.CapabilitySocketPath = startPublicToolGatewayCapabilityServer(t, func(request capabilities.ToolInvokeRequest) capabilities.ToolInvokeResponse {
		if request.ToolName != "task.add" {
			t.Fatalf("tool name = %q", request.ToolName)
		}
		if request.Context.RequesterEmail != "staff@example.com" {
			t.Fatalf("requester email = %q", request.Context.RequesterEmail)
		}
		if request.Actor.Email != "staff@example.com" || request.Actor.Source != "public_api_token" {
			t.Fatalf("actor = %#v", request.Actor)
		}
		if strings.Contains(string(request.Input), "other@example.com") {
			t.Fatalf("input should not contain overridden actor: %s", string(request.Input))
		}
		return capabilities.ToolInvokeResponse{Provider: "internkim", SelectedBackend: "device", ToolName: request.ToolName, Status: "ok", Result: json.RawMessage(`{"ok":true}`)}
	})
	token, _, errorValue := service.issuePublicAPIToken(context.Background(), "staff@example.com", publicAPITokenCreateRequest{Scopes: []string{"write"}})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	requestBody := `{"input":{"prompt":"업무 추가"},"context":{"requesterEmail":"other@example.com"},"actor":{"email":"other@example.com"}}`
	request := httptest.NewRequest(http.MethodPost, "/api/v1/tools/task.add/invoke", strings.NewReader(requestBody))
	request.Header.Set("Authorization", "Bearer "+token)
	response := httptest.NewRecorder()

	service.handlePublicAPI(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d body = %s", response.Code, response.Body.String())
	}
}

func TestPublicToolGatewayRequiresExplicitWriteScope(t *testing.T) {
	service := newFlowAuthorizationTestService(t)
	service.Configuration.CapabilitySocketPath = startPublicToolGatewayCapabilityServer(t, denyScopeCapabilityHandler(t))
	token, _, errorValue := service.issuePublicAPIToken(context.Background(), "staff@example.com", publicAPITokenCreateRequest{})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	request := httptest.NewRequest(http.MethodPost, "/api/v1/tools/task.add/invoke", strings.NewReader(`{"input":{"prompt":"업무 추가"}}`))
	request.Header.Set("Authorization", "Bearer "+token)
	response := httptest.NewRecorder()

	service.handlePublicAPI(response, request)

	if response.Code != http.StatusForbidden {
		t.Fatalf("status = %d body = %s", response.Code, response.Body.String())
	}
}

func TestPublicToolGatewayDeniesConnectWithoutWriteScope(t *testing.T) {
	service := newFlowAuthorizationTestService(t)
	service.Configuration.CapabilitySocketPath = startPublicToolGatewayCapabilityServer(t, denyScopeCapabilityHandler(t))
	token, _, errorValue := service.issuePublicAPIToken(context.Background(), "staff@example.com", publicAPITokenCreateRequest{})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	request := httptest.NewRequest(http.MethodPost, "/api/v1/tools/mail.connection.start/invoke", strings.NewReader(`{}`))
	request.Header.Set("Authorization", "Bearer "+token)
	response := httptest.NewRecorder()

	service.handlePublicAPI(response, request)

	if response.Code != http.StatusForbidden {
		t.Fatalf("status = %d body = %s", response.Code, response.Body.String())
	}
}

func TestPublicToolGatewayAllowsConnectScope(t *testing.T) {
	service := newFlowAuthorizationTestService(t)
	service.Configuration.CapabilitySocketPath = startPublicToolGatewayCapabilityServer(t, func(request capabilities.ToolInvokeRequest) capabilities.ToolInvokeResponse {
		if request.ToolName != "mail.connection.start" {
			t.Fatalf("tool name = %q", request.ToolName)
		}
		if !request.Context.IsApprovalContinuation {
			t.Fatal("connect tool should be treated as approved continuation for public token scope")
		}
		return capabilities.ToolInvokeResponse{Provider: "internkim", SelectedBackend: "device", ToolName: request.ToolName, Status: "connection_required", Result: json.RawMessage(`{"authorizationURL":"https://example.com/oauth"}`)}
	})
	token, _, errorValue := service.issuePublicAPIToken(context.Background(), "staff@example.com", publicAPITokenCreateRequest{Scopes: []string{"connect"}})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	request := httptest.NewRequest(http.MethodPost, "/api/v1/tools/mail.connection.start/invoke", strings.NewReader(`{}`))
	request.Header.Set("Authorization", "Bearer "+token)
	response := httptest.NewRecorder()

	service.handlePublicAPI(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d body = %s", response.Code, response.Body.String())
	}
}

func TestPublicToolGatewayRequiresExplicitDestructiveScope(t *testing.T) {
	service := newFlowAuthorizationTestService(t)
	service.Configuration.CapabilitySocketPath = startPublicToolGatewayCapabilityServer(t, denyScopeCapabilityHandler(t))
	token, _, errorValue := service.issuePublicAPIToken(context.Background(), "staff@example.com", publicAPITokenCreateRequest{Scopes: []string{"write"}})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	request := httptest.NewRequest(http.MethodPost, "/api/v1/tools/task.delete/invoke", strings.NewReader(`{"input":{"taskID":"task-1"}}`))
	request.Header.Set("Authorization", "Bearer "+token)
	response := httptest.NewRecorder()

	service.handlePublicAPI(response, request)

	if response.Code != http.StatusForbidden {
		t.Fatalf("status = %d body = %s", response.Code, response.Body.String())
	}
}

func TestPublicToolGatewayAllowsDestructiveScope(t *testing.T) {
	service := newFlowAuthorizationTestService(t)
	service.Configuration.CapabilitySocketPath = startPublicToolGatewayCapabilityServer(t, func(request capabilities.ToolInvokeRequest) capabilities.ToolInvokeResponse {
		if request.ToolName != "task.delete" {
			t.Fatalf("tool name = %q", request.ToolName)
		}
		if !request.Context.IsApprovalContinuation {
			t.Fatal("delete tool should be treated as approved continuation for public token scope")
		}
		return capabilities.ToolInvokeResponse{Provider: "internkim", SelectedBackend: "device", ToolName: request.ToolName, Status: "deleted", Result: json.RawMessage(`{"status":"deleted"}`)}
	})
	token, _, errorValue := service.issuePublicAPIToken(context.Background(), "staff@example.com", publicAPITokenCreateRequest{Scopes: []string{"destructive"}})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	request := httptest.NewRequest(http.MethodPost, "/api/v1/tools/task.delete/invoke", strings.NewReader(`{"input":{"taskID":"task-1"}}`))
	request.Header.Set("Authorization", "Bearer "+token)
	response := httptest.NewRecorder()

	service.handlePublicAPI(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d body = %s", response.Code, response.Body.String())
	}
}

func TestPublicToolGatewayLegacyScopeStillGrantsWriteTier(t *testing.T) {
	service := newFlowAuthorizationTestService(t)
	service.Configuration.CapabilitySocketPath = startPublicToolGatewayCapabilityServer(t, func(request capabilities.ToolInvokeRequest) capabilities.ToolInvokeResponse {
		return capabilities.ToolInvokeResponse{Provider: "internkim", SelectedBackend: "device", ToolName: request.ToolName, Status: "ok", Result: json.RawMessage(`{"ok":true}`)}
	})
	token, _, errorValue := service.issuePublicAPIToken(context.Background(), "staff@example.com", publicAPITokenCreateRequest{Scopes: []string{"external_send"}})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	request := httptest.NewRequest(http.MethodPost, "/api/v1/tools/task.add/invoke", strings.NewReader(`{"input":{"prompt":"업무 추가"}}`))
	request.Header.Set("Authorization", "Bearer "+token)
	response := httptest.NewRecorder()

	service.handlePublicAPI(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d body = %s", response.Code, response.Body.String())
	}
}

func TestPublicToolGatewayDoesNotListCalendarConnectionStart(t *testing.T) {
	service := newFlowAuthorizationTestService(t)
	service.Configuration.CapabilitySocketPath = startPublicToolGatewayCapabilityServer(t, denyScopeCapabilityHandler(t))
	token, _, errorValue := service.issuePublicAPIToken(context.Background(), "staff@example.com", publicAPITokenCreateRequest{Scopes: []string{"read", "write", "connect"}})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	request := httptest.NewRequest(http.MethodGet, "/api/v1/tools", nil)
	request.Header.Set("Authorization", "Bearer "+token)
	response := httptest.NewRecorder()

	service.handlePublicAPI(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d body = %s", response.Code, response.Body.String())
	}
	var payload struct {
		Tools []struct {
			Name string `json:"name"`
		} `json:"tools"`
	}
	if errorValue := json.Unmarshal(response.Body.Bytes(), &payload); errorValue != nil {
		t.Fatal(errorValue)
	}
	toolNames := make([]string, 0, len(payload.Tools))
	for _, tool := range payload.Tools {
		toolNames = append(toolNames, tool.Name)
	}
	if containsString(toolNames, "calendar.connection.start") {
		t.Fatalf("calendar connection start should not be listed: %+v", toolNames)
	}
	if !containsString(toolNames, "calendar.connection.status") {
		t.Fatalf("calendar connection status should remain listed: %+v", toolNames)
	}
}

func TestPublicToolGatewayRejectsRemovedTokenOwner(t *testing.T) {
	service := newFlowAuthorizationTestService(t)
	token := "ik_removed_owner"
	errorValue := service.writePublicAPITokenRecords(context.Background(), []publicAPITokenRecord{{
		ID:        "tok-removed",
		Label:     "Removed owner",
		Email:     "removed@example.com",
		PersonID:  "user-removed",
		TokenHash: publicAPITokenHash(token),
		Scopes:    []string{"read"},
		CreatedAt: time.Now().UTC(),
	}})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	request := httptest.NewRequest(http.MethodGet, "/api/v1/tools", nil)
	request.Header.Set("Authorization", "Bearer "+token)
	response := httptest.NewRecorder()

	service.handlePublicAPI(response, request)

	if response.Code != http.StatusForbidden {
		t.Fatalf("status = %d body = %s", response.Code, response.Body.String())
	}
}

func TestFlowCreateDefaultsBusinessAndUpdatePreservesOmittedBusiness(t *testing.T) {
	service := newFlowAuthorizationTestService(t)
	if errorValue := service.writeFlowDefinitions(context.Background(), flowDefinitions{
		Categories: []string{"제품"},
		Types:      []string{"회의"},
		Sizes:      defaultFlowSizeDefinitions(),
	}); errorValue != nil {
		t.Fatal(errorValue)
	}
	createRequest := newFlowTaskRequest("staff@example.com", "staff@example.com")
	createResponse := httptest.NewRecorder()
	service.createFlowTask(createResponse, createRequest)
	if createResponse.Code != http.StatusOK {
		t.Fatalf("create status = %d body = %s", createResponse.Code, createResponse.Body.String())
	}
	var createdTask flowTask
	if errorValue := json.Unmarshal(createResponse.Body.Bytes(), &createdTask); errorValue != nil {
		t.Fatal(errorValue)
	}
	if createdTask.Business != "제품" {
		t.Fatalf("created business = %q", createdTask.Business)
	}
	updateDocument := `{"ownerID":"` + createdTask.OwnerID + `","participantIDs":["` + createdTask.OwnerID + `"],"type":"회의","content":"10분 회의","size":"XS","status":"진행","weekCode":"26W18"}`
	updateRequest := httptest.NewRequest(http.MethodPut, "/flow/api/tasks/"+createdTask.ID, strings.NewReader(updateDocument))
	updateRequest.RemoteAddr = "198.51.100.10:443"
	updateRequest.Header.Set("Cf-Access-Authenticated-User-Email", "staff@example.com")
	updateResponse := httptest.NewRecorder()
	service.updateFlowTask(updateResponse, updateRequest, createdTask.ID)
	if updateResponse.Code != http.StatusOK {
		t.Fatalf("update status = %d body = %s", updateResponse.Code, updateResponse.Body.String())
	}
	var updatedTask flowTask
	if errorValue := json.Unmarshal(updateResponse.Body.Bytes(), &updatedTask); errorValue != nil {
		t.Fatal(errorValue)
	}
	if updatedTask.Business != "제품" {
		t.Fatalf("updated business = %q", updatedTask.Business)
	}
}

func denyScopeCapabilityHandler(t *testing.T) func(capabilities.ToolInvokeRequest) capabilities.ToolInvokeResponse {
	return func(request capabilities.ToolInvokeRequest) capabilities.ToolInvokeResponse {
		t.Fatalf("scope-denied request must not reach capability invoke: %s", request.ToolName)
		return capabilities.ToolInvokeResponse{}
	}
}

func TestPublicToolScopeForDescriptorClosedByDefault(t *testing.T) {
	cases := map[string]string{
		"read":             "",
		"workspace_write":  publicAPIScopeWrite,
		"workspace_task":   publicAPIScopeWrite,
		"external_write":   publicAPIScopeWrite,
		"external_send":    publicAPIScopeWrite,
		"external_publish": publicAPIScopeWrite,
		"site_publish":     publicAPIScopeWrite,
		"connect":          publicAPIScopeWrite,
		"browser_write":    publicAPIScopeWrite,
		"destructive":      publicAPIScopeDestructive,
		"mystery_class":    publicAPIScopeDestructive,
	}
	for sideEffectClass, expectedScope := range cases {
		scope := publicToolScopeForDescriptor(capabilities.Descriptor{SideEffectClass: sideEffectClass})
		if scope != expectedScope {
			t.Fatalf("%s: scope = %q, want %q", sideEffectClass, scope, expectedScope)
		}
	}
}

func startPublicToolGatewayCapabilityServer(t *testing.T, handler func(capabilities.ToolInvokeRequest) capabilities.ToolInvokeResponse) string {
	t.Helper()
	directoryPath, errorValue := os.MkdirTemp("/tmp", "ik-capability-*")
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	t.Cleanup(func() {
		_ = os.RemoveAll(directoryPath)
	})
	socketPath := filepath.Join(directoryPath, "capability.sock")
	listener, errorValue := net.Listen("unix", socketPath)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	server := &http.Server{Handler: http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		if request.URL.Path == "/v1/capabilities" {
			_ = json.NewEncoder(responseWriter).Encode(capabilities.RegistryResponse{DeviceCapabilities: capabilities.DeviceDescriptors()})
			return
		}
		var toolRequest capabilities.ToolInvokeRequest
		if errorValue := json.NewDecoder(request.Body).Decode(&toolRequest); errorValue != nil {
			t.Fatal(errorValue)
		}
		_ = json.NewEncoder(responseWriter).Encode(handler(toolRequest))
	})}
	t.Cleanup(func() {
		shutdownContext, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_ = server.Shutdown(shutdownContext)
		_ = listener.Close()
	})
	go func() {
		if errorValue := server.Serve(listener); errorValue != nil && errorValue != http.ErrServerClosed {
			t.Errorf("capability server failed: %v", errorValue)
		}
	}()
	return socketPath
}
