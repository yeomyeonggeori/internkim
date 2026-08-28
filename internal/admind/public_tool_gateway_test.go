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
	service := newTaskAuthorizationTestService(t)
	service.Configuration.CapabilitySocketPath = startPublicToolGatewayCapabilityServer(t, func(request capabilities.ToolInvokeRequest) capabilities.ToolInvokeResponse {
		if request.ToolName != "task_add" {
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
	requestBody := `{"input":{"title":"업무 추가"},"context":{"requesterEmail":"other@example.com"},"actor":{"email":"other@example.com"}}`
	request := httptest.NewRequest(http.MethodPost, "/api/v1/tools/task_add/invoke", strings.NewReader(requestBody))
	request.Header.Set("Authorization", "Bearer "+token)
	response := httptest.NewRecorder()

	service.handlePublicAPI(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d body = %s", response.Code, response.Body.String())
	}
}

func TestPublicToolGatewayRequiresExplicitWritePermission(t *testing.T) {
	service := newTaskAuthorizationTestService(t)
	service.Configuration.CapabilitySocketPath = startPublicToolGatewayCapabilityServer(t, denyPermissionCapabilityHandler(t))
	token, _, errorValue := service.issuePublicAPIToken(context.Background(), "staff@example.com", publicAPITokenCreateRequest{})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	request := httptest.NewRequest(http.MethodPost, "/api/v1/tools/task_add/invoke", strings.NewReader(`{"input":{"title":"업무 추가"}}`))
	request.Header.Set("Authorization", "Bearer "+token)
	response := httptest.NewRecorder()

	service.handlePublicAPI(response, request)

	if response.Code != http.StatusForbidden {
		t.Fatalf("status = %d body = %s", response.Code, response.Body.String())
	}
}

func TestPublicToolGatewayDeniesConnectWithoutWritePermission(t *testing.T) {
	service := newTaskAuthorizationTestService(t)
	service.Configuration.CapabilitySocketPath = startPublicToolGatewayCapabilityServer(t, denyPermissionCapabilityHandler(t))
	token, _, errorValue := service.issuePublicAPIToken(context.Background(), "staff@example.com", publicAPITokenCreateRequest{})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	request := httptest.NewRequest(http.MethodPost, "/api/v1/tools/mail_connection_start/invoke", strings.NewReader(`{}`))
	request.Header.Set("Authorization", "Bearer "+token)
	response := httptest.NewRecorder()

	service.handlePublicAPI(response, request)

	if response.Code != http.StatusForbidden {
		t.Fatalf("status = %d body = %s", response.Code, response.Body.String())
	}
}

func TestPublicToolGatewayAllowsConnectWithWritePermission(t *testing.T) {
	service := newTaskAuthorizationTestService(t)
	service.Configuration.CapabilitySocketPath = startPublicToolGatewayCapabilityServer(t, func(request capabilities.ToolInvokeRequest) capabilities.ToolInvokeResponse {
		if request.ToolName != "mail_connection_start" {
			t.Fatalf("tool name = %q", request.ToolName)
		}
		if !request.Context.IsApprovalContinuation {
			t.Fatal("connect tool should be treated as approved continuation for a public token")
		}
		return capabilities.ToolInvokeResponse{Provider: "internkim", SelectedBackend: "device", ToolName: request.ToolName, Status: "connection_required", Result: json.RawMessage(`{"authorizationURL":"https://example.com/oauth"}`)}
	})
	token, _, errorValue := service.issuePublicAPIToken(context.Background(), "staff@example.com", publicAPITokenCreateRequest{Scopes: []string{"write"}})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	request := httptest.NewRequest(http.MethodPost, "/api/v1/tools/mail_connection_start/invoke", strings.NewReader(`{}`))
	request.Header.Set("Authorization", "Bearer "+token)
	response := httptest.NewRecorder()

	service.handlePublicAPI(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d body = %s", response.Code, response.Body.String())
	}
}

func TestPublicToolGatewayRequiresExplicitDeletePermission(t *testing.T) {
	service := newTaskAuthorizationTestService(t)
	service.Configuration.CapabilitySocketPath = startPublicToolGatewayCapabilityServer(t, denyPermissionCapabilityHandler(t))
	token, _, errorValue := service.issuePublicAPIToken(context.Background(), "staff@example.com", publicAPITokenCreateRequest{Scopes: []string{"write"}})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	request := httptest.NewRequest(http.MethodPost, "/api/v1/tools/task_delete/invoke", strings.NewReader(`{"input":{"taskID":"task-1"}}`))
	request.Header.Set("Authorization", "Bearer "+token)
	response := httptest.NewRecorder()

	service.handlePublicAPI(response, request)

	if response.Code != http.StatusForbidden {
		t.Fatalf("status = %d body = %s", response.Code, response.Body.String())
	}
}

func TestPublicToolGatewayAllowsDeletePermission(t *testing.T) {
	service := newTaskAuthorizationTestService(t)
	service.Configuration.CapabilitySocketPath = startPublicToolGatewayCapabilityServer(t, func(request capabilities.ToolInvokeRequest) capabilities.ToolInvokeResponse {
		if request.ToolName != "task_delete" {
			t.Fatalf("tool name = %q", request.ToolName)
		}
		if !request.Context.IsApprovalContinuation {
			t.Fatal("delete tool should be treated as approved continuation for a public token")
		}
		return capabilities.ToolInvokeResponse{Provider: "internkim", SelectedBackend: "device", ToolName: request.ToolName, Status: "deleted", Result: json.RawMessage(`{"status":"deleted"}`)}
	})
	token, _, errorValue := service.issuePublicAPIToken(context.Background(), "staff@example.com", publicAPITokenCreateRequest{Scopes: []string{"delete"}})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	request := httptest.NewRequest(http.MethodPost, "/api/v1/tools/task_delete/invoke", strings.NewReader(`{"input":{"taskID":"task-1"}}`))
	request.Header.Set("Authorization", "Bearer "+token)
	response := httptest.NewRecorder()

	service.handlePublicAPI(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d body = %s", response.Code, response.Body.String())
	}
}

func TestPublicToolGatewayRetiredPermissionNameReachesNothingButReads(t *testing.T) {
	service := newTaskAuthorizationTestService(t)
	service.Configuration.CapabilitySocketPath = startPublicToolGatewayCapabilityServer(t, denyPermissionCapabilityHandler(t))
	token, _, errorValue := service.issuePublicAPIToken(context.Background(), "staff@example.com", publicAPITokenCreateRequest{Scopes: []string{"external_send", "destructive", "admin"}})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	request := httptest.NewRequest(http.MethodPost, "/api/v1/tools/task_add/invoke", strings.NewReader(`{"input":{"title":"업무 추가"}}`))
	request.Header.Set("Authorization", "Bearer "+token)
	response := httptest.NewRecorder()

	service.handlePublicAPI(response, request)

	if response.Code != http.StatusForbidden {
		t.Fatalf("status = %d body = %s", response.Code, response.Body.String())
	}
}

func TestPublicToolGatewayDoesNotListCalendarConnectionStart(t *testing.T) {
	service := newTaskAuthorizationTestService(t)
	service.Configuration.CapabilitySocketPath = startPublicToolGatewayCapabilityServer(t, denyPermissionCapabilityHandler(t))
	token, _, errorValue := service.issuePublicAPIToken(context.Background(), "staff@example.com", publicAPITokenCreateRequest{Scopes: []string{"read", "write", "delete"}})
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
	if containsString(toolNames, "calendar.connection.status") {
		t.Fatalf("calendar connection status should not be listed: %+v", toolNames)
	}
}

func TestPublicToolGatewayRejectsRemovedTokenOwner(t *testing.T) {
	service := newTaskAuthorizationTestService(t)
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

func TestTaskCreateDefaultsBusinessAndUpdatePreservesOmittedBusiness(t *testing.T) {
	service := newTaskAuthorizationTestService(t)
	if errorValue := service.writeTaskDefinitions(context.Background(), taskDefinitions{
		Categories: []string{"제품"},
		Types:      []string{"회의"},
		Sizes:      defaultTaskSizeDefinitions(),
	}); errorValue != nil {
		t.Fatal(errorValue)
	}
	createRequest := newTaskRequest("staff@example.com", "staff@example.com")
	createResponse := httptest.NewRecorder()
	service.createTask(createResponse, createRequest)
	if createResponse.Code != http.StatusOK {
		t.Fatalf("create status = %d body = %s", createResponse.Code, createResponse.Body.String())
	}
	var createdTask Task
	if errorValue := json.Unmarshal(createResponse.Body.Bytes(), &createdTask); errorValue != nil {
		t.Fatal(errorValue)
	}
	if createdTask.Business != "제품" {
		t.Fatalf("created business = %q", createdTask.Business)
	}
	updateDocument := `{"ownerID":"` + createdTask.OwnerID + `","participantIDs":["` + createdTask.OwnerID + `"],"type":"회의","content":"10분 회의","size":"XS","status":"in_progress","weekCode":"26W18"}`
	updateRequest := httptest.NewRequest(http.MethodPut, "/flow/api/tasks/"+createdTask.ID, strings.NewReader(updateDocument))
	updateRequest.RemoteAddr = "198.51.100.10:443"
	updateRequest.Header.Set("Cf-Access-Authenticated-User-Email", "staff@example.com")
	updateResponse := httptest.NewRecorder()
	service.updateTask(updateResponse, updateRequest, createdTask.ID)
	if updateResponse.Code != http.StatusOK {
		t.Fatalf("update status = %d body = %s", updateResponse.Code, updateResponse.Body.String())
	}
	var updatedTask Task
	if errorValue := json.Unmarshal(updateResponse.Body.Bytes(), &updatedTask); errorValue != nil {
		t.Fatal(errorValue)
	}
	if updatedTask.Business != "제품" {
		t.Fatalf("updated business = %q", updatedTask.Business)
	}
}

func TestPublicAPIRunsAsTheRequesterTheSocketAsserts(t *testing.T) {
	service := newTaskAuthorizationTestService(t)
	service.Configuration.CapabilitySocketPath = startPublicToolGatewayCapabilityServer(t, func(request capabilities.ToolInvokeRequest) capabilities.ToolInvokeResponse {
		if request.Actor.Email != "staff@example.com" || request.Actor.PersonID != "user-staff" {
			t.Fatalf("actor = %#v", request.Actor)
		}
		if request.Actor.Source != publicAPIActorSourceAssertedRequester {
			t.Fatalf("actor source = %q", request.Actor.Source)
		}
		return capabilities.ToolInvokeResponse{Provider: "internkim", SelectedBackend: "device", ToolName: request.ToolName, Status: "deleted", Result: json.RawMessage(`{"status":"deleted"}`)}
	})
	request := httptest.NewRequest(http.MethodPost, "/api/v1/tools/task_delete/invoke", strings.NewReader(`{"input":{"taskID":"task-1"}}`))
	request.Header.Set(requesterEmailHeader, "staff@example.com")
	request.Header.Set(requesterPermissionHeader, publicAPIPermissionDelete)
	response := httptest.NewRecorder()

	service.handlePublicAPI(response, arrivingOnTheRequesterSocket(request))

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d body = %s", response.Code, response.Body.String())
	}
}

func TestPublicAPIHoldsAnAssertedRequesterToItsPermission(t *testing.T) {
	service := newTaskAuthorizationTestService(t)
	service.Configuration.CapabilitySocketPath = startPublicToolGatewayCapabilityServer(t, denyPermissionCapabilityHandler(t))
	request := httptest.NewRequest(http.MethodPost, "/api/v1/tools/task_delete/invoke", strings.NewReader(`{"input":{"taskID":"task-1"}}`))
	request.Header.Set(requesterEmailHeader, "staff@example.com")
	request.Header.Set(requesterPermissionHeader, publicAPIPermissionWrite)
	response := httptest.NewRecorder()

	service.handlePublicAPI(response, arrivingOnTheRequesterSocket(request))

	if response.Code != http.StatusForbidden {
		t.Fatalf("status = %d body = %s", response.Code, response.Body.String())
	}
}

func TestPublicAPIIgnoresAnAssertedRequesterOnTheTCPListener(t *testing.T) {
	service := newTaskAuthorizationTestService(t)
	service.Configuration.CapabilitySocketPath = startPublicToolGatewayCapabilityServer(t, denyPermissionCapabilityHandler(t))
	request := httptest.NewRequest(http.MethodPost, "/api/v1/tools/task_delete/invoke", strings.NewReader(`{"input":{"taskID":"task-1"}}`))
	request.RemoteAddr = "127.0.0.1:12345"
	request.Header.Set(requesterEmailHeader, "staff@example.com")
	request.Header.Set(requesterPermissionHeader, publicAPIPermissionDelete)
	response := httptest.NewRecorder()

	service.handlePublicAPI(response, request)

	if response.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d body = %s", response.Code, response.Body.String())
	}
}

func TestPublicAPIPermissionsAreALadder(t *testing.T) {
	cases := map[string]int{
		"":                        1,
		publicAPIPermissionRead:   1,
		publicAPIPermissionWrite:  2,
		publicAPIPermissionDelete: 3,
		"destructive":             1,
		"admin":                   1,
	}
	for permission, expectedRank := range cases {
		if rank := publicAPIPermissionRank(permission); rank != expectedRank {
			t.Fatalf("%q ranks %d, want %d", permission, rank, expectedRank)
		}
	}
}

func denyPermissionCapabilityHandler(t *testing.T) func(capabilities.ToolInvokeRequest) capabilities.ToolInvokeResponse {
	return func(request capabilities.ToolInvokeRequest) capabilities.ToolInvokeResponse {
		t.Fatalf("permission-denied request must not reach capability invoke: %s", request.ToolName)
		return capabilities.ToolInvokeResponse{}
	}
}

func TestPublicToolPermissionForDescriptorClosedByDefault(t *testing.T) {
	cases := map[string]string{
		"read":             "",
		"workspace_write":  publicAPIPermissionWrite,
		"workspace_task":   publicAPIPermissionWrite,
		"external_write":   publicAPIPermissionWrite,
		"external_send":    publicAPIPermissionWrite,
		"external_publish": publicAPIPermissionWrite,
		"site_publish":     publicAPIPermissionWrite,
		"connect":          publicAPIPermissionWrite,
		"browser_write":    publicAPIPermissionWrite,
		"destructive":      publicAPIPermissionDelete,
		"mystery_class":    publicAPIPermissionDelete,
	}
	for sideEffectClass, expectedPermission := range cases {
		permission := publicToolPermissionForDescriptor(capabilities.Descriptor{SideEffectClass: sideEffectClass})
		if permission != expectedPermission {
			t.Fatalf("%s: permission = %q, want %q", sideEffectClass, permission, expectedPermission)
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
