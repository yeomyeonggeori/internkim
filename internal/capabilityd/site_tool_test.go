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
		Input:    json.RawMessage(`{"slug":"demo","title":"Demo","prompt":"Build a booking site","designBrief":"Editorial restaurant style","prototypeScope":"booking request flow"}`),
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
	if requestBody["prompt"] != "Build a booking site" || requestBody["designBrief"] != "Editorial restaurant style" || requestBody["prototypeScope"] != "booking request flow" {
		t.Fatalf("prototype creation context was not propagated: %+v", requestBody)
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

func TestSiteAppStatusResolvesConversationSite(t *testing.T) {
	requests := []string{}
	service := Service{
		Configuration: Configuration{AdmindBaseURL: "http://admind.local"},
		HTTPClient: &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			requests = append(requests, request.Method+" "+request.URL.Path)
			switch {
			case request.Method == http.MethodGet && request.URL.String() == "http://admind.local/admin/api/sites":
				return siteToolJSONResponse(`{"sites":[{"siteID":"site-1","slug":"demo","platform":"mattermost","conversationID":"thread-1","status":"draft"}]}`), nil
			case request.Method == http.MethodGet && request.URL.String() == "http://admind.local/admin/api/sites/site-1":
				return siteToolJSONResponse(`{"siteID":"site-1","status":"draft","sourceWorkspacePath":"home/sites/site-1","appWorkspacePath":"home/sites/site-1/app"}`), nil
			default:
				t.Fatalf("unexpected request %s %s", request.Method, request.URL.String())
				return nil, nil
			}
		})},
	}

	response, errorValue := service.invokeSiteAppTool(context.Background(), capabilities.ToolInvokeRequest{
		ToolName: "site.app.status",
		Input:    json.RawMessage(`{}`),
		Context: capabilities.ToolInvokeContext{
			Platform:       "mattermost",
			ConversationID: "thread-1",
		},
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if !strings.Contains(string(response.Result), `"appWorkspacePath":"home/sites/site-1/app"`) {
		t.Fatalf("expected status result to include appWorkspacePath, got %s", response.Result)
	}
	if strings.Join(requests, ",") != "GET /admin/api/sites,GET /admin/api/sites/site-1" {
		t.Fatalf("unexpected requests: %+v", requests)
	}
}

func TestSiteAppStatusReturnsAmbiguousCandidates(t *testing.T) {
	service := Service{
		Configuration: Configuration{AdmindBaseURL: "http://admind.local"},
		HTTPClient: &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			switch {
			case request.Method == http.MethodGet && request.URL.String() == "http://admind.local/admin/api/sites":
				return siteToolJSONResponse(`{"sites":[{"siteID":"site-1","slug":"one","title":"One","description":"First site","purpose":"portfolio","archetype":"portfolio","publishedURL":"https://one.example","platform":"mattermost","conversationID":"thread-1","ownerIdentity":{"personID":"person-1","displayName":"Owner"},"status":"draft"},{"siteID":"site-2","slug":"two","title":"Two","description":"Second site","purpose":"booking","archetype":"booking","publishedURL":"https://two.example","platform":"mattermost","conversationID":"thread-1","ownerIdentity":{"personID":"person-1","displayName":"Owner"},"status":"draft"}]}`), nil
			default:
				t.Fatalf("unexpected request %s %s", request.Method, request.URL.String())
				return nil, nil
			}
		})},
	}

	response, errorValue := service.invokeSiteAppTool(context.Background(), capabilities.ToolInvokeRequest{
		ToolName: "site.app.status",
		Input:    json.RawMessage(`{}`),
		Context: capabilities.ToolInvokeContext{
			RequesterPersonID: "person-1",
			Platform:          "mattermost",
			ConversationID:    "thread-1",
		},
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	result := string(response.Result)
	if !strings.Contains(result, `"status":"ambiguous"`) || !strings.Contains(result, `"description":"First site"`) || !strings.Contains(result, `"archetype":"booking"`) {
		t.Fatalf("expected ambiguous candidate summaries, got %s", result)
	}
}

func TestSiteAppStatusIgnoresConversationSitesRequesterCannotEdit(t *testing.T) {
	service := Service{
		Configuration: Configuration{AdmindBaseURL: "http://admind.local"},
		HTTPClient: &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			switch {
			case request.Method == http.MethodGet && request.URL.String() == "http://admind.local/admin/api/sites":
				return siteToolJSONResponse(`{"sites":[{"siteID":"site-1","slug":"demo","platform":"mattermost","conversationID":"thread-1","ownerIdentity":{"personID":"owner-person"},"status":"draft"}]}`), nil
			default:
				t.Fatalf("unexpected request %s %s", request.Method, request.URL.String())
				return nil, nil
			}
		})},
	}

	response, errorValue := service.invokeSiteAppTool(context.Background(), capabilities.ToolInvokeRequest{
		ToolName: "site.app.status",
		Input:    json.RawMessage(`{}`),
		Context: capabilities.ToolInvokeContext{
			RequesterPersonID: "other-person",
			Platform:          "mattermost",
			ConversationID:    "thread-1",
		},
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if !strings.Contains(string(response.Result), `"status":"not_found"`) {
		t.Fatalf("expected inaccessible site not to resolve, got %s", response.Result)
	}
}

func TestSiteAppStatusBySlugHidesSourcePathsForNonEditor(t *testing.T) {
	service := Service{
		Configuration: Configuration{AdmindBaseURL: "http://admind.local"},
		HTTPClient: &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			switch {
			case request.Method == http.MethodGet && request.URL.String() == "http://admind.local/admin/api/sites":
				return siteToolJSONResponse(`{"sites":[{"siteID":"site-1","slug":"demo","platform":"mattermost","conversationID":"thread-1","ownerIdentity":{"personID":"owner-person"},"status":"draft"}]}`), nil
			case request.Method == http.MethodGet && request.URL.String() == "http://admind.local/admin/api/sites/site-1":
				return siteToolJSONResponse(`{"siteID":"site-1","slug":"demo","title":"Demo","description":"Public description","sourceWorkspacePath":"home/sites/site-1","appWorkspacePath":"home/sites/site-1/app","hostSourcePath":"/root/sites/site-1","ownerIdentity":{"personID":"owner-person"},"status":"draft"}`), nil
			default:
				t.Fatalf("unexpected request %s %s", request.Method, request.URL.String())
				return nil, nil
			}
		})},
	}

	response, errorValue := service.invokeSiteAppTool(context.Background(), capabilities.ToolInvokeRequest{
		ToolName: "site.app.status",
		Input:    json.RawMessage(`{"slug":"demo"}`),
		Context: capabilities.ToolInvokeContext{
			RequesterPersonID: "other-person",
			Platform:          "mattermost",
			ConversationID:    "thread-1",
		},
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	result := string(response.Result)
	if strings.Contains(result, "sourceWorkspacePath") || strings.Contains(result, "appWorkspacePath") || !strings.Contains(result, `"canEdit":false`) {
		t.Fatalf("expected public status without source paths, got %s", result)
	}
}

func TestSiteAppHistoryResolvesConversationSite(t *testing.T) {
	requests := []string{}
	service := Service{
		Configuration: Configuration{AdmindBaseURL: "http://admind.local"},
		HTTPClient: &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			requests = append(requests, request.Method+" "+request.URL.Path)
			switch {
			case request.Method == http.MethodGet && request.URL.String() == "http://admind.local/admin/api/sites":
				return siteToolJSONResponse(`{"sites":[{"siteID":"site-1","slug":"demo","platform":"mattermost","conversationID":"thread-1","status":"published"}]}`), nil
			case request.Method == http.MethodGet && request.URL.String() == "http://admind.local/admin/api/sites/site-1/history":
				return siteToolJSONResponse(`{"siteID":"site-1","revisions":[]}`), nil
			default:
				t.Fatalf("unexpected request %s %s", request.Method, request.URL.String())
				return nil, nil
			}
		})},
	}

	_, errorValue := service.invokeSiteAppTool(context.Background(), capabilities.ToolInvokeRequest{
		ToolName: "site.app.history",
		Input:    json.RawMessage(`{}`),
		Context: capabilities.ToolInvokeContext{
			Platform:       "mattermost",
			ConversationID: "thread-1",
		},
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if strings.Join(requests, ",") != "GET /admin/api/sites,GET /admin/api/sites/site-1/history" {
		t.Fatalf("unexpected requests: %+v", requests)
	}
}

func TestSiteAppDiffPassesRevisionQuery(t *testing.T) {
	requests := []string{}
	service := Service{
		Configuration: Configuration{AdmindBaseURL: "http://admind.local"},
		HTTPClient: &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			requests = append(requests, request.Method+" "+request.URL.String())
			switch {
			case request.Method == http.MethodGet && request.URL.String() == "http://admind.local/admin/api/sites":
				return siteToolJSONResponse(`{"sites":[{"siteID":"site-1","slug":"demo","platform":"mattermost","conversationID":"thread-1","status":"published"}]}`), nil
			case request.Method == http.MethodGet && request.URL.String() == "http://admind.local/admin/api/sites/site-1/diff?fromRevision=abc&toRevision=def":
				return siteToolJSONResponse(`{"siteID":"site-1","summary":"changed"}`), nil
			default:
				t.Fatalf("unexpected request %s %s", request.Method, request.URL.String())
				return nil, nil
			}
		})},
	}

	_, errorValue := service.invokeSiteAppTool(context.Background(), capabilities.ToolInvokeRequest{
		ToolName: "site.app.diff",
		Input:    json.RawMessage(`{"fromRevision":"abc","toRevision":"def"}`),
		Context: capabilities.ToolInvokeContext{
			Platform:       "mattermost",
			ConversationID: "thread-1",
		},
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if len(requests) != 2 {
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
