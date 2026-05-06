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

func TestSiteAppCreatePropagatesConversationContext(t *testing.T) {
	var requestBody map[string]any
	service := Service{
		Configuration: Configuration{AdmindBaseURL: "http://admind.local"},
		HTTPClient: &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			if request.Method != http.MethodPost || request.URL.String() != "http://admind.local/admin/api/sites" {
				t.Fatalf("unexpected request %s %s", request.Method, request.URL.String())
			}
			if errorValue := json.NewDecoder(request.Body).Decode(&requestBody); errorValue != nil {
				t.Fatal(errorValue)
			}
			return siteToolJSONResponse(`{"siteID":"site-1"}`), nil
		})},
	}

	_, errorValue := service.invokeSiteAppTool(context.Background(), capabilities.ToolInvokeRequest{
		ToolName: "site.app.create",
		Input:    json.RawMessage(`{"slug":"demo","title":"Demo"}`),
		Context: capabilities.ToolInvokeContext{
			RequesterEmail: "owner@example.com",
			Platform:       "mattermost",
			ConversationID: "thread-1",
		},
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if requestBody["requestedBy"] != "owner@example.com" || requestBody["platform"] != "mattermost" || requestBody["conversationID"] != "thread-1" {
		t.Fatalf("context was not propagated: %+v", requestBody)
	}
}

func TestSiteAppPublishRequiresExistingSiteID(t *testing.T) {
	service := Service{Configuration: Configuration{AdmindBaseURL: "http://admind.local"}}
	_, errorValue := service.invokeSiteAppTool(context.Background(), capabilities.ToolInvokeRequest{
		ToolName: "site.app.publish",
		Input:    json.RawMessage(`{"slug":"demo"}`),
	})
	if errorValue == nil {
		t.Fatal("expected siteID requirement")
	}
}

func TestSiteAppPublishResolvesConversationSite(t *testing.T) {
	requests := []string{}
	service := Service{
		Configuration: Configuration{AdmindBaseURL: "http://admind.local"},
		HTTPClient: &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			requests = append(requests, request.Method+" "+request.URL.Path)
			switch {
			case request.Method == http.MethodGet && request.URL.String() == "http://admind.local/admin/api/sites":
				return siteToolJSONResponse(`{"sites":[{"siteID":"site-1","slug":"demo","platform":"mattermost","conversationID":"thread-1","status":"published"}]}`), nil
			case request.Method == http.MethodPost && request.URL.String() == "http://admind.local/admin/api/sites/site-1/publish":
				return siteToolJSONResponse(`{"siteID":"site-1","status":"published"}`), nil
			default:
				t.Fatalf("unexpected request %s %s", request.Method, request.URL.String())
				return nil, nil
			}
		})},
	}

	_, errorValue := service.invokeSiteAppTool(context.Background(), capabilities.ToolInvokeRequest{
		ToolName: "site.app.publish",
		Input:    json.RawMessage(`{"message":"Update prototype"}`),
		Context: capabilities.ToolInvokeContext{
			Platform:       "mattermost",
			ConversationID: "thread-1",
		},
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if strings.Join(requests, ",") != "GET /admin/api/sites,POST /admin/api/sites/site-1/publish" {
		t.Fatalf("unexpected requests: %+v", requests)
	}
}

func siteToolJSONResponse(document string) *http.Response {
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(document)),
	}
}
