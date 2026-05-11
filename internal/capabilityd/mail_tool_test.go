package capabilityd

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"gitlab.com/eastriver/internkim/internal/capabilities"
)

func TestMailMessageSendRequiresApprovalContinuation(t *testing.T) {
	service := Service{Configuration: Configuration{AdmindBaseURL: "http://admind.local"}}
	_, errorValue := service.invokeMailMessageSend(context.Background(), capabilities.ToolInvokeRequest{
		ToolName: "mail.message.send",
		Input:    []byte(`{"to":["recipient@example.com"],"subject":"Demo"}`),
	})
	if errorValue == nil || !strings.Contains(errorValue.Error(), "requires approval") {
		t.Fatalf("expected approval error, got %v", errorValue)
	}
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
		ToolName: "mail.message.send",
		Input:    []byte(`{"to":["recipient@example.com"],"subject":"Demo","body":"Hello"}`),
		Context: capabilities.ToolInvokeContext{
			RequesterEmail:         "Staff@Example.com",
			IsApprovalContinuation: true,
		},
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if requesterEmail != "staff@example.com" {
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
		ToolName: "mail.message.list",
		Input:    []byte(`{"mailbox":"INBOX","limit":5,"cursor":"cursor-1"}`),
		Context:  capabilities.ToolInvokeContext{RequesterEmail: "Staff@Example.com"},
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if requesterEmail != "staff@example.com" {
		t.Fatalf("requesterEmail = %q", requesterEmail)
	}
	if requestPath != "http://admind.local/mail/api/messages?cursor=cursor-1&limit=5&mailbox=INBOX" {
		t.Fatalf("requestPath = %q", requestPath)
	}
}

func TestMailMessageSearchRequiresQuery(t *testing.T) {
	service := Service{Configuration: Configuration{AdmindBaseURL: "http://admind.local"}}
	_, errorValue := service.invokeMailMessageSearch(context.Background(), capabilities.ToolInvokeRequest{
		ToolName: "mail.message.search",
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
		ToolName: "mail.message.search",
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

func mailToolJSONResponse(document string) *http.Response {
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(document)),
	}
}
