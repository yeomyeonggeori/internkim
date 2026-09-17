package capabilityd

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"gitlab.com/eastriver/internkim/internal/capabilities"
	"gitlab.com/eastriver/internkim/internal/mail"
)

func TestMailMessageSendRequiresDescriptorApproval(t *testing.T) {
	service := Service{Configuration: Configuration{AdmindBaseURL: "http://admind.local"}}
	response, errorValue := service.invokeCapabilityTool(context.Background(), "mail_message_send", strings.NewReader(`{"input":{"to":["recipient@example.com"],"subject":"Demo","body":"Hello"}}`))
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	assertCapabilityApprovalRequired(t, response, "mail_message_send")
}

func TestMailMessageSendApprovedContinuationPostsToAdmind(t *testing.T) {
	var requesterEmail string
	var payload map[string]any
	service := Service{
		Configuration: Configuration{AdmindBaseURL: "http://admind.local"},
		HTTPClient: &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			if request.Method != http.MethodPost || request.URL.String() != "http://admind.local/mail/api/messages/send" {
				t.Fatalf("unexpected request %s %s", request.Method, request.URL.String())
			}
			requesterEmail = request.Header.Get("CF-Access-Authenticated-User-Email")
			if errorValue := json.NewDecoder(request.Body).Decode(&payload); errorValue != nil {
				t.Fatal(errorValue)
			}
			return mailToolJSONResponse(`{"sent":true}`), nil
		})},
	}

	result, errorValue := service.invokeMailMessageSend(context.Background(), capabilities.ToolInvokeRequest{
		ToolName: "mail_message_send",
		Input:    []byte(`{"to":["recipient@example.com"],"subject":"Demo","body":"Hello"}`),
		Context: capabilities.ToolInvokeContext{
			RequesterEmail:         "Member@Example.com",
			IsApprovalContinuation: true,
		},
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if requesterEmail != "member@example.com" {
		t.Fatalf("requesterEmail = %q", requesterEmail)
	}
	if payload["subject"] != "Demo" {
		t.Fatalf("payload = %#v", payload)
	}
	if strings.TrimSpace(string(result)) != `{"sent":true}` {
		t.Fatalf("result = %s", string(result))
	}
}

func TestMailMessageListForwardsCursorAndRequester(t *testing.T) {
	var requesterEmail string
	var requestPath string
	service := Service{
		Configuration: Configuration{AdmindBaseURL: "http://admind.local"},
		HTTPClient: &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			requesterEmail = request.Header.Get("CF-Access-Authenticated-User-Email")
			requestPath = request.URL.String()
			return mailToolJSONResponse(`{"messages":[],"nextCursor":""}`), nil
		})},
	}

	_, errorValue := service.invokeMailMessageList(context.Background(), capabilities.ToolInvokeRequest{
		ToolName: "mail_message_list",
		Input:    []byte(`{"mailbox":"INBOX","limit":5,"cursor":"cursor-1"}`),
		Context:  capabilities.ToolInvokeContext{RequesterEmail: "Member@Example.com"},
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if requesterEmail != "member@example.com" {
		t.Fatalf("requesterEmail = %q", requesterEmail)
	}
	if requestPath != "http://admind.local/mail/api/messages?cursor=cursor-1&limit=5&mailbox=INBOX" {
		t.Fatalf("requestPath = %q", requestPath)
	}
}

func TestMailMessageSearchRequiresQuery(t *testing.T) {
	service := Service{Configuration: Configuration{AdmindBaseURL: "http://admind.local"}}
	_, errorValue := service.invokeMailMessageSearch(context.Background(), capabilities.ToolInvokeRequest{
		ToolName: "mail_message_search",
		Input:    []byte(`{"mailbox":"INBOX"}`),
	})
	if errorValue == nil || !strings.Contains(errorValue.Error(), "query is required") {
		t.Fatalf("expected query error, got %v", errorValue)
	}
}

func TestMailMessageSearchForwardsQueryAndCursor(t *testing.T) {
	var requestPath string
	service := Service{
		Configuration: Configuration{AdmindBaseURL: "http://admind.local"},
		HTTPClient: &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			requestPath = request.URL.String()
			return mailToolJSONResponse(`{"messages":[],"nextCursor":""}`), nil
		})},
	}

	_, errorValue := service.invokeMailMessageSearch(context.Background(), capabilities.ToolInvokeRequest{
		ToolName: "mail_message_search",
		Input:    []byte(`{"mailbox":"INBOX","query":"invoice","limit":5,"cursor":"cursor-1"}`),
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if requestPath != "http://admind.local/mail/api/messages?cursor=cursor-1&limit=5&mailbox=INBOX&query=invoice" {
		t.Fatalf("requestPath = %q", requestPath)
	}
}

func TestMailMessageListRejectsTooLargeLimit(t *testing.T) {
	_, errorValue := decodeMailMessageListInput([]byte(`{"limit":51}`))
	if errorValue == nil || !strings.Contains(errorValue.Error(), "limit must be between 1 and 50") {
		t.Fatalf("expected limit error, got %v", errorValue)
	}
}

func TestMailConnectionStatusUsesAccountEndpoint(t *testing.T) {
	var requesterEmail string
	service := Service{
		Configuration: Configuration{AdmindBaseURL: "http://admind.local"},
		HTTPClient: &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			if request.Method != http.MethodGet || request.URL.String() != "http://admind.local/mail/api/account" {
				t.Fatalf("unexpected request %s %s", request.Method, request.URL.String())
			}
			requesterEmail = request.Header.Get("CF-Access-Authenticated-User-Email")
			return mailToolJSONResponse(`{"configured":true,"email":"member@example.com"}`), nil
		})},
	}

	result, errorValue := service.invokeMailConnectionStatus(context.Background(), capabilities.ToolInvokeRequest{
		ToolName: "mail_connection_status",
		Context:  capabilities.ToolInvokeContext{RequesterEmail: "Member@Example.com"},
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if requesterEmail != "member@example.com" {
		t.Fatalf("requesterEmail = %q", requesterEmail)
	}
	if !strings.Contains(string(result), `"configured":true`) {
		t.Fatalf("result = %s", string(result))
	}
}

func TestMailConnectionStartReturnsSetupURL(t *testing.T) {
	service := Service{Configuration: Configuration{AdmindBaseURL: "http://admind.local"}}
	result, errorValue := service.invokeMailConnectionStart(context.Background(), capabilities.ToolInvokeRequest{ToolName: "mail_connection_start"})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if !strings.Contains(string(result), `"setupURL":"http://admind.local/mail/"`) {
		t.Fatalf("result = %s", string(result))
	}
}

func TestMailAnswersHoldTheirResultContracts(t *testing.T) {
	cases := []struct {
		toolName string
		input    string
		answer   any
	}{
		{toolName: "mail_connection_status", input: `{}`, answer: mail.AccountToResponse(mail.DefaultAccount("member@example.com"))},
		{toolName: "mail_message_list", input: `{"mailbox":"INBOX"}`, answer: mail.MessageListResponse{Messages: []mail.MessageResponse{}}},
		{toolName: "mail_message_list", input: `{"mailbox":"INBOX"}`, answer: mail.MessageListResponse{
			Messages:    []mail.MessageResponse{{UID: 42, Mailbox: "INBOX", Subject: "Invoice", From: "Alice <alice@example.com>", Date: "2026-09-17T09:00:00Z", Preview: "Attached", IsRead: true}},
			NextCursor:  "cursor-2",
			UIDNext:     43,
			UIDValidity: 7,
		}},
		{toolName: "mail_message_search", input: `{"query":"invoice"}`, answer: mail.MessageListResponse{Messages: []mail.MessageResponse{{UID: 42, Mailbox: "INBOX"}}}},
		{toolName: "mail_message_read", input: `{"mailbox":"INBOX","uid":"42"}`, answer: mail.MessageDetailResponse{UID: 42, Mailbox: "INBOX", Body: "Hello", BodyHTML: "<p>Hello</p>"}},
		{toolName: "mail_message_send", input: `{"to":["recipient@example.com"],"subject":"Demo","body":"Hello"}`, answer: mail.SendResult{Sent: true, AppendedTo: "Sent"}},
		{toolName: "mail_message_mark", input: `{"mailbox":"INBOX","uid":"42","seen":true}`, answer: map[string]bool{"marked": true}},
		{toolName: "mail_message_move", input: `{"mailbox":"INBOX","uid":"42","targetMailbox":"Archive"}`, answer: map[string]bool{"moved": true}},
		{toolName: "mail_connection_start", input: `{}`},
	}
	for _, testCase := range cases {
		t.Run(testCase.toolName, func(t *testing.T) {
			answer, errorValue := json.Marshal(testCase.answer)
			if errorValue != nil {
				t.Fatal(errorValue)
			}
			service := Service{
				Configuration: Configuration{AdmindBaseURL: "http://admind.local"},
				HTTPClient: &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
					return mailToolJSONResponse(string(answer)), nil
				})},
			}
			response, errorValue := service.invokeMailTool(context.Background(), capabilities.ToolInvokeRequest{
				ToolName: testCase.toolName,
				Input:    []byte(testCase.input),
			})
			if errorValue != nil {
				t.Fatal(errorValue)
			}
			if response.Outcome != capabilities.ToolOutcomeSucceeded {
				t.Fatalf("response = %+v", response)
			}
		})
	}
}

func mailToolJSONResponse(document string) *http.Response {
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(document)),
	}
}
