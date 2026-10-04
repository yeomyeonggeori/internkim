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

	"github.com/yeomyeonggeori/internkim/internal/capabilities"
)

func TestPublicToolGatewayOverridesActorFromBearerToken(t *testing.T) {
	service := newTaskAuthorizationTestService(t)
	service.Configuration.CapabilitySocketPath = startPublicToolGatewayCapabilityServer(t, func(request capabilities.ToolInvokeRequest) capabilities.ToolInvokeResponse {
		if request.ToolName != "task_add" {
			t.Fatalf("tool name = %q", request.ToolName)
		}
		if request.Context.RequesterEmail != "member@example.com" {
			t.Fatalf("requester email = %q", request.Context.RequesterEmail)
		}
		if request.Actor.Email != "member@example.com" || request.Actor.Source != "public_api_token" {
			t.Fatalf("actor = %#v", request.Actor)
		}
		if strings.Contains(string(request.Input), "other@example.com") {
			t.Fatalf("input should not contain overridden actor: %s", string(request.Input))
		}
		return capabilities.ToolInvokeResponse{Provider: "internkim", SelectedBackend: "device", ToolName: request.ToolName, Status: "ok", Result: json.RawMessage(`{"ok":true}`)}
	})
	token, _, errorValue := service.issuePublicAPIToken(context.Background(), "member@example.com", publicAPITokenCreateRequest{Scopes: []string{"write"}})
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
	token, _, errorValue := service.issuePublicAPIToken(context.Background(), "member@example.com", publicAPITokenCreateRequest{})
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
	token, _, errorValue := service.issuePublicAPIToken(context.Background(), "member@example.com", publicAPITokenCreateRequest{})
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
	token, _, errorValue := service.issuePublicAPIToken(context.Background(), "member@example.com", publicAPITokenCreateRequest{Scopes: []string{"write"}})
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
	token, _, errorValue := service.issuePublicAPIToken(context.Background(), "member@example.com", publicAPITokenCreateRequest{Scopes: []string{"write"}})
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
	token, _, errorValue := service.issuePublicAPIToken(context.Background(), "member@example.com", publicAPITokenCreateRequest{Scopes: []string{"delete"}})
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
	token, _, errorValue := service.issuePublicAPIToken(context.Background(), "member@example.com", publicAPITokenCreateRequest{Scopes: []string{"external_send", "destructive", "admin"}})
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
	token, _, errorValue := service.issuePublicAPIToken(context.Background(), "member@example.com", publicAPITokenCreateRequest{Scopes: []string{"read", "write", "delete"}})
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

func TestPublicAPIRunsAsTheRequesterTheSocketAsserts(t *testing.T) {
	service := newTaskAuthorizationTestService(t)
	service.Configuration.CapabilitySocketPath = startPublicToolGatewayCapabilityServer(t, func(request capabilities.ToolInvokeRequest) capabilities.ToolInvokeResponse {
		if request.Actor.Email != "member@example.com" || request.Actor.PersonID != "user-member" {
			t.Fatalf("actor = %#v", request.Actor)
		}
		if request.Actor.Source != publicAPIActorSourceAssertedRequester {
			t.Fatalf("actor source = %q", request.Actor.Source)
		}
		return capabilities.ToolInvokeResponse{Provider: "internkim", SelectedBackend: "device", ToolName: request.ToolName, Status: "deleted", Result: json.RawMessage(`{"status":"deleted"}`)}
	})
	request := httptest.NewRequest(http.MethodPost, "/api/v1/tools/task_delete/invoke", strings.NewReader(`{"input":{"taskID":"task-1"}}`))
	request.Header.Set(requesterEmailHeader, "member@example.com")
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
	request.Header.Set(requesterEmailHeader, "member@example.com")
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
	request.Header.Set(requesterEmailHeader, "member@example.com")
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
		"computation":      "",
		"workspace_write":  publicAPIPermissionWrite,
		"external_write":   publicAPIPermissionWrite,
		"external_send":    publicAPIPermissionWrite,
		"external_publish": publicAPIPermissionWrite,
		"connect":          publicAPIPermissionWrite,
		"approval":         publicAPIPermissionWrite,
		"local_file":       publicAPIPermissionWrite,
		"platform_reply":   publicAPIPermissionWrite,
		"destructive":      publicAPIPermissionDelete,
		"mystery_class":    publicAPIPermissionDelete,
		"workspace_task":   publicAPIPermissionDelete,
		"browser_write":    publicAPIPermissionDelete,
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
			_ = json.NewEncoder(responseWriter).Encode(capabilities.RegistryResponse{DeviceCapabilities: capabilities.DefaultToolDescriptors()})
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

func TestPublicAPICarriesNoConversationAtAll(t *testing.T) {
	service := newTaskAuthorizationTestService(t)
	service.Configuration.ChatdPlatform = "buzz"
	var invoked capabilities.ToolInvokeRequest
	service.Configuration.CapabilitySocketPath = startPublicToolGatewayCapabilityServer(t, func(request capabilities.ToolInvokeRequest) capabilities.ToolInvokeResponse {
		invoked = request
		return capabilities.ToolInvokeResponse{Provider: "internkim", SelectedBackend: "device", ToolName: request.ToolName, Status: "sent", Result: json.RawMessage(`{"messageIDs":["message-1"],"deliveryStatus":"sent"}`)}
	})
	request := httptest.NewRequest(http.MethodPost, "/api/v1/tools/message_send/invoke", strings.NewReader(`{"input":{"targetType":"directMessage","personHint":"이샘플","message":"확인 부탁드립니다"}}`))
	request.Header.Set(requesterEmailHeader, "member@example.com")
	request.Header.Set(requesterPermissionHeader, publicAPIPermissionWrite)
	response := httptest.NewRecorder()

	service.handlePublicAPI(response, arrivingOnTheRequesterSocket(request))

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d body = %s", response.Code, response.Body.String())
	}
	if invoked.Context.Platform != "" || invoked.Context.ConversationID != "" || invoked.Context.ChannelID != "" || invoked.Context.ReplyTargetID != "" {
		t.Fatalf("the public API is a door, not a conversation: %#v", invoked.Context)
	}
	if invoked.Context.TaskSource != "public_api" {
		t.Fatalf("task source = %q", invoked.Context.TaskSource)
	}
}

func TestThePublicAPICallIsTheAdministratorsConfirmationOfAHostUpdate(t *testing.T) {
	for _, testCase := range []struct {
		toolName       string
		permission     string
		isConfirmation bool
	}{
		{"host_update", publicAPIPermissionDelete, true},
		{"host_version_get", publicAPIPermissionRead, false},
	} {
		t.Run(testCase.toolName, func(t *testing.T) {
			service := newTaskAuthorizationTestService(t)
			var invoked capabilities.ToolInvokeRequest
			service.Configuration.CapabilitySocketPath = startPublicToolGatewayCapabilityServer(t, func(request capabilities.ToolInvokeRequest) capabilities.ToolInvokeResponse {
				invoked = request
				return capabilities.ToolInvokeResponse{Provider: "internkim", SelectedBackend: "device", ToolName: request.ToolName, Outcome: capabilities.ToolOutcomeSucceeded, Result: json.RawMessage(`{}`)}
			})
			request := httptest.NewRequest(http.MethodPost, "/api/v1/tools/"+testCase.toolName+"/invoke", strings.NewReader(`{"input":{}}`))
			request.Header.Set(requesterEmailHeader, "admin@example.com")
			request.Header.Set(requesterPermissionHeader, testCase.permission)
			response := httptest.NewRecorder()

			service.handlePublicAPI(response, arrivingOnTheRequesterSocket(request))

			if response.Code != http.StatusOK {
				t.Fatalf("status = %d body = %s", response.Code, response.Body.String())
			}
			if invoked.Context.IsApprovalContinuation != testCase.isConfirmation || invoked.Context.TaskSource != capabilities.TaskSourcePublicAPI {
				t.Fatalf("the call reached the capability daemon as %#v", invoked.Context)
			}
			if invoked.Context.RequesterEmail != "admin@example.com" || invoked.Context.RequesterPersonID != "user-admin" || invoked.Context.ConversationID != "" {
				t.Fatalf("the call reached the capability daemon for %#v", invoked.Context)
			}
		})
	}
}

func TestAHostUpdateNeedsADeletingPermissionAtThePublicAPI(t *testing.T) {
	service := newTaskAuthorizationTestService(t)
	service.Configuration.CapabilitySocketPath = startPublicToolGatewayCapabilityServer(t, denyPermissionCapabilityHandler(t))
	request := httptest.NewRequest(http.MethodPost, "/api/v1/tools/host_update/invoke", strings.NewReader(`{"input":{}}`))
	request.Header.Set(requesterEmailHeader, "admin@example.com")
	request.Header.Set(requesterPermissionHeader, publicAPIPermissionWrite)
	response := httptest.NewRecorder()

	service.handlePublicAPI(response, arrivingOnTheRequesterSocket(request))

	if response.Code != http.StatusForbidden {
		t.Fatalf("status = %d body = %s", response.Code, response.Body.String())
	}
}
