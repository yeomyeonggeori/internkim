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

func mailToolJSONResponse(document string) *http.Response {
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(document)),
	}
}
