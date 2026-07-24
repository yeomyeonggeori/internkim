package capabilityd

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"gitlab.com/eastriver/internkim/internal/capabilities"
)

func TestSiteServePublishPropagatesContextAndBundle(t *testing.T) {
	var requestBody map[string]any
	service := Service{
		Configuration: Configuration{AdmindBaseURL: "http://admind.local"},
		HTTPClient: &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			if request.Method != http.MethodPost || request.URL.String() != "http://admind.local/admin/api/sites/serve" {
				t.Fatalf("unexpected request %s %s", request.Method, request.URL.String())
			}
			if errorValue := json.NewDecoder(request.Body).Decode(&requestBody); errorValue != nil {
				t.Fatal(errorValue)
			}
			return siteToolJSONResponse(`{"siteID":"site-1","slug":"demo-site","title":"Demo Site","status":"published","publishedURL":"https://demo-site.example","currentVersionID":"version-1","owner":"internal-only"}`), nil
		})},
	}

	transport := testSiteSourceBundleTransport("~/sites/demo-site")
	response, errorValue := invokeSiteCapabilityTool(t, service, capabilities.ToolInvokeRequest{
		ToolName:  "site.serve",
		Input:     json.RawMessage(`{"title":"Demo Site","sourceWorkspacePath":"~/sites/demo-site","mode":"publish"}`),
		Transport: transport,
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
	if requestBody["title"] != "Demo Site" || requestBody["mode"] != "publish" || requestBody["sourceWorkspacePath"] != "~/sites/demo-site" {
		t.Fatalf("serve input was not propagated: %+v", requestBody)
	}
	if requestBody["sourceBundleBase64"] == "" || requestBody["sourceBundleFormat"] != "tar.gz" || requestBody["sourceSHA256"] != transport.SiteSourceBundle.SHA256 {
		t.Fatalf("source bundle was not propagated: %+v", requestBody)
	}
	if response.Outcome != capabilities.ToolOutcomeSucceeded || response.Status != "published" {
		t.Fatalf("unexpected serve response: %+v", response)
	}
	if len(response.Effects) != 1 || response.Effects[0].ObjectType != "website" || response.Effects[0].Effect != "published" || response.Effects[0].URL != "https://demo-site.example" {
		t.Fatalf("unexpected serve effects: %+v", response.Effects)
	}
	if strings.Contains(string(response.Result), `"owner"`) || !strings.Contains(string(response.Result), `"sourceSHA256":"`+transport.SiteSourceBundle.SHA256+`"`) {
		t.Fatalf("expected canonical serve projection, got %s", response.Result)
	}
	if !strings.Contains(string(response.Result), `"slug":"demo-site"`) || !strings.Contains(string(response.Result), `"mode":"publish"`) {
		t.Fatalf("expected serve result to carry the server-allocated slug and mode, got %s", response.Result)
	}
}

func TestSiteServePreviewProjectsPreviewURL(t *testing.T) {
	service := Service{
		Configuration: Configuration{AdmindBaseURL: "http://admind.local"},
		HTTPClient: &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			if request.Method != http.MethodPost || request.URL.String() != "http://admind.local/admin/api/sites/serve" {
				t.Fatalf("unexpected request %s %s", request.Method, request.URL.String())
			}
			return siteToolJSONResponse(`{"siteID":"site-1","slug":"demo-site","title":"Demo Site","status":"draft","previewID":"preview-1","previewURL":"https://demo-site.example/__preview/preview-1"}`), nil
		})},
	}

	response, errorValue := invokeSiteCapabilityTool(t, service, capabilities.ToolInvokeRequest{
		ToolName:  "site.serve",
		Input:     json.RawMessage(`{"title":"Demo Site","sourceWorkspacePath":"~/sites/demo-site","mode":"preview"}`),
		Transport: testSiteSourceBundleTransport("~/sites/demo-site"),
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if response.Status != "previewed" || len(response.Effects) != 1 || response.Effects[0].Effect != "previewed" || response.Effects[0].URL != "https://demo-site.example/__preview/preview-1" {
		t.Fatalf("unexpected preview response: %+v", response)
	}
	if strings.Contains(string(response.Result), "publishedURL") {
		t.Fatalf("preview projection must not carry publishedURL, got %s", response.Result)
	}
}

func TestSiteServeRequiresSourceBundle(t *testing.T) {
	service := Service{Configuration: Configuration{AdmindBaseURL: "http://admind.local"}}
	_, errorValue := service.invokeSiteAppTool(context.Background(), capabilities.ToolInvokeRequest{
		ToolName: "site.serve",
		Input:    json.RawMessage(`{"title":"Demo Site","sourceWorkspacePath":"~/sites/demo-site","mode":"publish"}`),
	})
	if errorValue == nil || !strings.Contains(errorValue.Error(), "source bundle is required") {
		t.Fatalf("expected source bundle requirement, got %v", errorValue)
	}
}

func TestSiteServeRejectsMismatchedReferenceResult(t *testing.T) {
	service := Service{
		Configuration: Configuration{AdmindBaseURL: "http://admind.local"},
		HTTPClient: &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			return siteToolJSONResponse(`{"siteID":"site-2","slug":"other-site","status":"published","publishedURL":"https://other-site.example"}`), nil
		})},
	}

	_, errorValue := invokeSiteCapabilityTool(t, service, capabilities.ToolInvokeRequest{
		ToolName:  "site.serve",
		Input:     json.RawMessage(`{"title":"Demo Site","sourceWorkspacePath":"~/sites/demo-site","mode":"publish","siteReference":"demo-site"}`),
		Transport: testSiteSourceBundleTransport("~/sites/demo-site"),
	})
	if errorValue == nil || !strings.Contains(errorValue.Error(), "does not match the requested siteReference") {
		t.Fatalf("expected mismatched result rejection, got %v", errorValue)
	}
}

func TestSiteServePassesThroughResolutionFailure(t *testing.T) {
	service := Service{
		Configuration: Configuration{AdmindBaseURL: "http://admind.local"},
		HTTPClient: &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			return siteToolJSONResponse(`{"status":"not_found","siteReference":"missing-site","candidates":[{"siteID":"site-1","slug":"existing"}]}`), nil
		})},
	}

	response, errorValue := invokeSiteCapabilityTool(t, service, capabilities.ToolInvokeRequest{
		ToolName:  "site.serve",
		Input:     json.RawMessage(`{"title":"Demo Site","sourceWorkspacePath":"~/sites/demo-site","mode":"publish","siteReference":"missing-site"}`),
		Transport: testSiteSourceBundleTransport("~/sites/demo-site"),
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if response.Outcome != capabilities.ToolOutcomeFailed || !response.IsError || response.ErrorCode != "site_not_found" || response.FailureStage != "resolution" {
		t.Fatalf("expected typed not_found response, got %+v", response)
	}
	if !strings.Contains(string(response.Result), `"slug":"existing"`) {
		t.Fatalf("expected candidates in resolution failure, got %s", response.Result)
	}
}

func TestSiteListProjectsCanonicalEntries(t *testing.T) {
	service := Service{
		Configuration: Configuration{AdmindBaseURL: "http://admind.local"},
		HTTPClient: &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			if request.Method != http.MethodGet || request.URL.String() != "http://admind.local/admin/api/sites" {
				t.Fatalf("unexpected request %s %s", request.Method, request.URL.String())
			}
			return siteToolJSONResponse(`{"sites":[{"siteID":"site-1","slug":"alpha","title":"Alpha","status":"published","publishedURL":"https://alpha.example","sourceBundleWorkspacePath":"~/sites/alpha","updatedAt":"2026-07-19T12:00:00Z","owner":"internal-only","hostSourcePath":"/root/sites/site-1"},{"siteID":"site-2","slug":"beta","title":"Beta","status":"draft"},{"siteID":"site-3","slug":"gone","title":"Gone","status":"deleting"}]}`), nil
		})},
	}

	response, errorValue := service.invokeSiteAppTool(context.Background(), capabilities.ToolInvokeRequest{
		ToolName: "site.list",
		Input:    json.RawMessage(`{}`),
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	result := string(response.Result)
	if response.Outcome != capabilities.ToolOutcomeSucceeded || len(response.Effects) != 0 {
		t.Fatalf("unexpected list response: %+v", response)
	}
	if !strings.Contains(result, `"slug":"alpha"`) || !strings.Contains(result, `"slug":"beta"`) {
		t.Fatalf("expected both served sites, got %s", result)
	}
	if strings.Contains(result, `"slug":"gone"`) {
		t.Fatalf("expected deleting site to be omitted, got %s", result)
	}
	if strings.Contains(result, "owner") || strings.Contains(result, "hostSourcePath") {
		t.Fatalf("expected canonical list projection, got %s", result)
	}
}

func TestSiteListFiltersByExactReference(t *testing.T) {
	service := Service{
		Configuration: Configuration{AdmindBaseURL: "http://admind.local"},
		HTTPClient: &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			return siteToolJSONResponse(`{"sites":[{"siteID":"site-1","slug":"alpha","title":"Alpha","status":"published","publishedURL":"https://alpha.example"},{"siteID":"site-2","slug":"beta","title":"Beta","status":"draft"}]}`), nil
		})},
	}

	response, errorValue := service.invokeSiteAppTool(context.Background(), capabilities.ToolInvokeRequest{
		ToolName: "site.list",
		Input:    json.RawMessage(`{"siteReference":"beta"}`),
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	result := string(response.Result)
	if !strings.Contains(result, `"slug":"beta"`) || strings.Contains(result, `"slug":"alpha"`) {
		t.Fatalf("expected exact reference filtering, got %s", result)
	}
}

func TestSiteListReturnsTypedNotFoundForUnknownReference(t *testing.T) {
	service := Service{
		Configuration: Configuration{AdmindBaseURL: "http://admind.local"},
		HTTPClient: &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			return siteToolJSONResponse(`{"sites":[{"siteID":"site-1","slug":"existing","title":"Existing","status":"draft","sourceBundleWorkspacePath":"~/sites/existing"}]}`), nil
		})},
	}

	response, errorValue := service.invokeSiteAppTool(context.Background(), capabilities.ToolInvokeRequest{
		ToolName: "site.list",
		Input:    json.RawMessage(`{"siteReference":"brand-new-site"}`),
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	result := string(response.Result)
	if !strings.Contains(result, `"status":"not_found"`) || !strings.Contains(result, `"siteReference":"brand-new-site"`) || !strings.Contains(result, `"slug":"existing"`) {
		t.Fatalf("expected not_found with candidates, got %s", result)
	}
	if !strings.Contains(result, `"sourceWorkspacePath":"~/sites/existing"`) {
		t.Fatalf("expected candidates to expose the serve source workspace path, got %s", result)
	}
	if response.Outcome != capabilities.ToolOutcomeFailed || !response.IsError || response.ErrorCode != "site_not_found" || response.FailureStage != "resolution" {
		t.Fatalf("expected typed not_found response, got %+v", response)
	}
}

func TestSiteUnserveResolvesReferenceAndDeletes(t *testing.T) {
	requests := []string{}
	service := Service{
		Configuration: Configuration{AdmindBaseURL: "http://admind.local"},
		HTTPClient: &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			requests = append(requests, request.Method+" "+request.URL.Path)
			switch {
			case request.Method == http.MethodGet && request.URL.Path == "/admin/api/sites":
				return siteToolJSONResponse(`{"sites":[{"siteID":"site-1","slug":"demo","title":"Demo","status":"published"}]}`), nil
			case request.Method == http.MethodDelete && request.URL.Path == "/admin/api/sites/site-1":
				var input map[string]any
				if errorValue := json.NewDecoder(request.Body).Decode(&input); errorValue != nil {
					t.Fatal(errorValue)
				}
				if input["confirm"] != "DELETE" || input["userConfirmed"] != true {
					t.Fatalf("expected internal approval proof, got %+v", input)
				}
				return siteToolJSONResponse(`{"siteID":"site-1","slug":"demo","status":"deleted","owner":"internal-only"}`), nil
			default:
				t.Fatalf("unexpected request %s %s", request.Method, request.URL.String())
				return nil, nil
			}
		})},
	}

	response, errorValue := service.invokeSiteAppTool(context.Background(), capabilities.ToolInvokeRequest{
		ToolName: "site.unserve",
		Input:    json.RawMessage(`{"siteReference":"demo","reason":"Remove obsolete launch page"}`),
		Context:  capabilities.ToolInvokeContext{IsApprovalContinuation: true},
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if strings.Join(requests, ",") != "GET /admin/api/sites,DELETE /admin/api/sites/site-1" {
		t.Fatalf("unexpected requests: %+v", requests)
	}
	if string(response.Result) != `{"siteID":"site-1","slug":"demo","unserved":true}` {
		t.Fatalf("unexpected unserve result %s", response.Result)
	}
	if response.Status != "deleted" || len(response.Effects) != 1 || response.Effects[0].ObjectType != "website" || response.Effects[0].Effect != "deleted" || response.Effects[0].ID != "site-1" {
		t.Fatalf("unexpected unserve effects %+v", response.Effects)
	}
}

func TestSiteUnserveReturnsTypedNotFound(t *testing.T) {
	service := Service{
		Configuration: Configuration{AdmindBaseURL: "http://admind.local"},
		HTTPClient: &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			return siteToolJSONResponse(`{"sites":[]}`), nil
		})},
	}

	response, errorValue := service.invokeSiteAppTool(context.Background(), capabilities.ToolInvokeRequest{
		ToolName: "site.unserve",
		Input:    json.RawMessage(`{"siteReference":"missing-site"}`),
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if response.Outcome != capabilities.ToolOutcomeFailed || response.ErrorCode != "site_not_found" || !strings.Contains(string(response.Result), `"siteReference":"missing-site"`) {
		t.Fatalf("expected typed not_found, got %+v", response)
	}
}

func TestSiteUnserveReturnsAmbiguousCandidates(t *testing.T) {
	service := Service{
		Configuration: Configuration{AdmindBaseURL: "http://admind.local"},
		HTTPClient: &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			return siteToolJSONResponse(`{"sites":[{"siteID":"shared","slug":"alpha","title":"Alpha","status":"draft"},{"siteID":"site-2","slug":"shared","title":"Beta","status":"draft"}]}`), nil
		})},
	}

	response, errorValue := service.invokeSiteAppTool(context.Background(), capabilities.ToolInvokeRequest{
		ToolName: "site.unserve",
		Input:    json.RawMessage(`{"siteReference":"shared"}`),
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if response.Outcome != capabilities.ToolOutcomeFailed || response.ErrorCode != "site_ambiguous" || !strings.Contains(string(response.Result), `"title":"Beta"`) {
		t.Fatalf("expected typed ambiguous response, got %+v", response)
	}
}

func TestSiteUnserveRequiresReference(t *testing.T) {
	service := Service{Configuration: Configuration{AdmindBaseURL: "http://admind.local"}}
	_, errorValue := service.invokeSiteAppTool(context.Background(), capabilities.ToolInvokeRequest{
		ToolName: "site.unserve",
		Input:    json.RawMessage(`{}`),
	})
	if errorValue == nil || !strings.Contains(errorValue.Error(), "siteReference is required") {
		t.Fatalf("expected siteReference requirement, got %v", errorValue)
	}
}

func TestSiteAppIdentityRejectsSurroundingWhitespace(t *testing.T) {
	for _, inputDocument := range []json.RawMessage{
		json.RawMessage(`{"siteReference":" demo"}`),
		json.RawMessage(`{"siteReference":"demo "}`),
	} {
		if _, errorValue := decodeSiteAppInput(inputDocument); errorValue == nil {
			t.Fatalf("expected exact identity rejection for %s", inputDocument)
		}
	}
}

func TestSiteServeRejectsPaddedSiteID(t *testing.T) {
	_, errorValue := projectCanonicalSiteAppResult(
		capabilities.ToolInvokeRequest{
			ToolName:  "site.serve",
			Input:     json.RawMessage(`{"title":"Demo","sourceWorkspacePath":"~/sites/demo","mode":"publish"}`),
			Transport: testSiteSourceBundleTransport("~/sites/demo"),
		},
		json.RawMessage(`{"siteID":" site-1","slug":"demo","status":"published","publishedURL":"https://demo.example"}`),
	)
	if errorValue == nil {
		t.Fatal("expected padded siteID result rejection")
	}
}

func TestSiteUnserveAdmindInputDoesNotTrustDirectConfirmation(t *testing.T) {
	input, errorValue := siteUnserveAdmindInput(json.RawMessage(`{"siteReference":"demo","confirm":"DELETE","userConfirmed":true}`), false)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if strings.Contains(string(input), "confirm") || strings.Contains(string(input), "userConfirmed") {
		t.Fatalf("expected direct confirmation fields to be removed, got %s", input)
	}
}

func TestSiteAppInputUsesOnlyRuntimeIdentity(t *testing.T) {
	input, errorValue := siteAppInputWithContext(
		json.RawMessage(`{
			"siteReference":"site-1",
			"requestedBy":"attacker@example.com",
			"requester":{"personID":"attacker"},
			"createdBy":{"personID":"attacker"},
			"ownerIdentity":{"personID":"attacker"}
		}`),
		capabilities.ToolInvokeContext{
			RequesterPersonID: "person-1",
			RequesterEmail:    "owner@example.com",
			Platform:          "mattermost",
			ConversationID:    "thread-1",
		},
	)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if strings.Contains(string(input), "attacker") {
		t.Fatalf("expected caller identity to be discarded, got %s", input)
	}
	for _, expected := range []string{`"requestedBy":"owner@example.com"`, `"personID":"person-1"`, `"platform":"mattermost"`, `"conversationID":"thread-1"`} {
		if !strings.Contains(string(input), expected) {
			t.Fatalf("expected %s in %s", expected, input)
		}
	}
}

func TestSiteServeRejectsInvalidSourceSHA256(t *testing.T) {
	for _, sourceSHA256 := range []string{
		"abc",
		"AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA",
	} {
		transport := testSiteSourceBundleTransport("~/sites/demo")
		transport.SiteSourceBundle.SHA256 = sourceSHA256
		errorValue := validateSiteSourceBundle(transport.SiteSourceBundle)
		if errorValue == nil || !strings.Contains(errorValue.Error(), "SHA-256") {
			t.Fatalf("expected invalid source SHA-256 rejection, got %v", errorValue)
		}
	}
}

func TestSiteAppRejectsMalformedSuccessfulResponse(t *testing.T) {
	service := Service{
		Configuration: Configuration{AdmindBaseURL: "http://admind.local"},
		HTTPClient: &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			return siteToolJSONResponse(`{}`), nil
		})},
	}

	_, errorValue := service.invokeSiteAppTool(context.Background(), capabilities.ToolInvokeRequest{
		ToolName:  "site.serve",
		Input:     json.RawMessage(`{"title":"Demo","sourceWorkspacePath":"~/sites/demo","mode":"publish"}`),
		Transport: testSiteSourceBundleTransport("~/sites/demo"),
	})
	if errorValue == nil || !strings.Contains(errorValue.Error(), "invalid response document") {
		t.Fatalf("expected malformed response rejection, got %v", errorValue)
	}
}

func siteToolJSONResponse(document string) *http.Response {
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(document)),
	}
}

func testSiteSourceBundleTransport(workspacePath string) capabilities.ToolInvokeTransport {
	content := []byte("site source bundle")
	digest := sha256.Sum256(content)
	return capabilities.ToolInvokeTransport{
		SiteSourceBundle: &capabilities.SiteSourceBundle{
			WorkspacePath: workspacePath,
			ContentBase64: base64.StdEncoding.EncodeToString(content),
			Format:        "tar.gz",
			SHA256:        hex.EncodeToString(digest[:]),
		},
	}
}

func invokeSiteCapabilityTool(t *testing.T, service Service, request capabilities.ToolInvokeRequest) (capabilities.ToolInvokeResponse, error) {
	t.Helper()
	document, errorValue := json.Marshal(request)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	return service.invokeCapabilityTool(context.Background(), request.ToolName, strings.NewReader(string(document)))
}
