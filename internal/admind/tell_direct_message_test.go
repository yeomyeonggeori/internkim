package admind

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"gitlab.com/eastriver/internkim/internal/capabilities"
)

func tellingRequest(t *testing.T, body string) *http.Request {
	t.Helper()
	request := httptest.NewRequest(http.MethodPost, tellDirectMessagePath, strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	return arrivingOnTheRequesterSocket(request)
}

func TestATellingReachesTheRecipientAsADirectMessageFromTheBot(t *testing.T) {
	service := newTaskAuthorizationTestService(t)
	var invoked capabilities.ToolInvokeRequest
	service.Configuration.CapabilitySocketPath = startPublicToolGatewayCapabilityServer(t, func(request capabilities.ToolInvokeRequest) capabilities.ToolInvokeResponse {
		invoked = request
		return capabilities.ToolInvokeResponse{ToolName: request.ToolName, Status: "sent", Result: json.RawMessage(`{"messageIDs":["post-1"],"deliveryStatus":"sent"}`)}
	})
	response := httptest.NewRecorder()

	service.handleTellDirectMessage(response, tellingRequest(t, `{"recipientEmail":"Member@Example.com","message":"결재를 기다리는 건이 있습니다"}`))

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d body = %s", response.Code, response.Body.String())
	}
	if invoked.ToolName != "message_send" {
		t.Fatalf("tool name = %q", invoked.ToolName)
	}
	var input struct {
		TargetType string `json:"targetType"`
		PersonHint string `json:"personHint"`
		Message    string `json:"message"`
	}
	if errorValue := json.Unmarshal(invoked.Input, &input); errorValue != nil {
		t.Fatal(errorValue)
	}
	if input.TargetType != "directMessage" || input.PersonHint != "member@example.com" {
		t.Fatalf("input = %+v", input)
	}
	if input.Message != "결재를 기다리는 건이 있습니다" {
		t.Fatalf("message = %q", input.Message)
	}
	if invoked.Context.RequesterPersonID != "user-member" || invoked.Context.RequesterEmail != "member@example.com" {
		t.Fatalf("context = %#v", invoked.Context)
	}
	if invoked.Actor.Source != tellDirectMessageSource {
		t.Fatalf("actor = %#v", invoked.Actor)
	}
}

func TestATellingIsNotHeldAtTheApprovalGate(t *testing.T) {
	service := newTaskAuthorizationTestService(t)
	var invoked capabilities.ToolInvokeRequest
	service.Configuration.CapabilitySocketPath = startPublicToolGatewayCapabilityServer(t, func(request capabilities.ToolInvokeRequest) capabilities.ToolInvokeResponse {
		invoked = request
		return capabilities.ToolInvokeResponse{ToolName: request.ToolName, Status: "sent", Result: json.RawMessage(`{"ok":true}`)}
	})
	response := httptest.NewRecorder()

	service.handleTellDirectMessage(response, tellingRequest(t, `{"recipientEmail":"member@example.com","message":"알려드립니다"}`))

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d body = %s", response.Code, response.Body.String())
	}
	if !invoked.Context.IsApprovalContinuation {
		t.Fatal("a system telling waits on an approver nobody asked, so it never leaves the device")
	}
}

// A telling names a recipient and nothing else. Which messenger carries it is
// capabilityd's own configuration to read, guarded there by
// TestADirectMessageWithNoConversationStillReachesTheCompanyMessenger.
func TestATellingCarriesNoConversation(t *testing.T) {
	service := newTaskAuthorizationTestService(t)
	service.Configuration.ChatdPlatform = "buzz"
	var invoked capabilities.ToolInvokeRequest
	service.Configuration.CapabilitySocketPath = startPublicToolGatewayCapabilityServer(t, func(request capabilities.ToolInvokeRequest) capabilities.ToolInvokeResponse {
		invoked = request
		return capabilities.ToolInvokeResponse{ToolName: request.ToolName, Status: "sent", Result: json.RawMessage(`{"ok":true}`)}
	})
	response := httptest.NewRecorder()

	service.handleTellDirectMessage(response, tellingRequest(t, `{"recipientEmail":"member@example.com","message":"알려드립니다"}`))

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d body = %s", response.Code, response.Body.String())
	}
	if invoked.Context.Platform != "" || invoked.Context.ConversationID != "" || invoked.Context.ReplyTargetID != "" {
		t.Fatalf("a telling is not a conversation: %#v", invoked.Context)
	}
}

func TestATellingIsRefusedOffTheRequesterSocket(t *testing.T) {
	service := newTaskAuthorizationTestService(t)
	request := httptest.NewRequest(http.MethodPost, tellDirectMessagePath, strings.NewReader(`{"recipientEmail":"member@example.com","message":"알려드립니다"}`))
	response := httptest.NewRecorder()

	service.handleTellDirectMessage(response, request)

	if response.Code != http.StatusForbidden {
		t.Fatalf("status = %d body = %s", response.Code, response.Body.String())
	}
}

func TestATellingNamingNobodyOrNothingIsRefused(t *testing.T) {
	service := newTaskAuthorizationTestService(t)
	for _, body := range []string{`{"recipientEmail":"","message":"알려드립니다"}`, `{"recipientEmail":"member@example.com","message":"   "}`} {
		response := httptest.NewRecorder()

		service.handleTellDirectMessage(response, tellingRequest(t, body))

		if response.Code != http.StatusBadRequest {
			t.Errorf("%s answered %d", body, response.Code)
		}
	}
}

func TestATellingForSomebodyOutsideTheCompanyIsRefused(t *testing.T) {
	service := newTaskAuthorizationTestService(t)
	response := httptest.NewRecorder()

	service.handleTellDirectMessage(response, tellingRequest(t, `{"recipientEmail":"stranger@example.com","message":"알려드립니다"}`))

	if response.Code != http.StatusNotFound {
		t.Fatalf("status = %d body = %s", response.Code, response.Body.String())
	}
}
