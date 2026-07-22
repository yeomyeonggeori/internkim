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
			return siteToolJSONResponse(`{"siteID":"site-1","slug":"demo","title":"Demo","status":"draft","sourceWorkspacePath":"/workspace/circles/staff/sites/demo/draft","appWorkspacePath":"/workspace/circles/staff/sites/demo/draft/app","sourceFiles":[{"path":"app/index.ts","content":"export {}"}],"owner":"internal-only"}`), nil
		})},
	}

	response, errorValue := service.invokeSiteAppTool(context.Background(), capabilities.ToolInvokeRequest{
		ToolName: "site.create",
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
	if response.Outcome != capabilities.ToolOutcomeSucceeded || response.Status != "created" || len(response.Effects) != 1 {
		t.Fatalf("unexpected create response: %+v", response)
	}
	if response.Effects[0].ObjectType != "website" || response.Effects[0].Effect != "created" || response.Effects[0].ID != "site-1" {
		t.Fatalf("unexpected create effect: %+v", response.Effects)
	}
	if strings.Contains(string(response.Result), `"owner"`) {
		t.Fatalf("expected canonical create result without owner, got %s", response.Result)
	}
	if !strings.Contains(string(response.Result), `"sourceFiles"`) {
		t.Fatalf("expected create result to carry sourceFiles for guest materialization, got %s", response.Result)
	}
}

func TestSiteAppPublishRequiresExistingSiteID(t *testing.T) {
	service := Service{Configuration: Configuration{AdmindBaseURL: "http://admind.local"}}
	_, errorValue := service.invokeSiteAppTool(context.Background(), capabilities.ToolInvokeRequest{
		ToolName: "site.publish",
		Input:    json.RawMessage(`{"slug":"demo"}`),
	})
	if errorValue == nil {
		t.Fatal("expected siteID requirement")
	}
}

func TestSiteAppIdentityRejectsSurroundingWhitespace(t *testing.T) {
	for _, inputDocument := range []json.RawMessage{
		json.RawMessage(`{"siteID":" site-1"}`),
		json.RawMessage(`{"siteID":"site-1 "}`),
		json.RawMessage(`{"siteReference":" demo"}`),
		json.RawMessage(`{"siteReference":"demo "}`),
	} {
		if _, errorValue := decodeSiteAppInput(inputDocument); errorValue == nil {
			t.Fatalf("expected exact identity rejection for %s", inputDocument)
		}
	}
}

func TestSiteAppResultRejectsPaddedSiteID(t *testing.T) {
	_, errorValue := projectCanonicalSiteAppResult(
		capabilities.ToolInvokeRequest{
			ToolName:  "site.publish",
			Input:     json.RawMessage(`{"siteID":"site-1"}`),
			Transport: testSiteSourceBundleTransport("/workspace/site"),
		},
		json.RawMessage(`{"siteID":" site-1","status":"published","sourceWorkspacePath":"/workspace/site","sourceSHA256":"254cc09182b94752e96474af9ba307f74dcfff4e8dfa5b0c4a76f97e634c1c28","publishedURL":"https://example.com","currentVersionID":"version-1"}`),
	)
	if errorValue == nil {
		t.Fatal("expected padded siteID result rejection")
	}
}

func TestSiteAppPublishUsesExactSiteID(t *testing.T) {
	requests := []string{}
	service := Service{
		Configuration: Configuration{AdmindBaseURL: "http://admind.local"},
		HTTPClient: &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			requests = append(requests, request.Method+" "+request.URL.Path)
			switch {
			case request.Method == http.MethodPost && request.URL.String() == "http://admind.local/admin/api/sites/site-1/publish":
				return siteToolJSONResponse(`{"siteID":"site-1","status":"published","sourceWorkspacePath":"/workspace/circles/staff/sites/demo/draft","publishedURL":"https://demo.example","currentVersionID":"version-1","owner":"internal-only"}`), nil
			default:
				t.Fatalf("unexpected request %s %s", request.Method, request.URL.String())
				return nil, nil
			}
		})},
	}

	transport := testSiteSourceBundleTransport("/workspace/circles/staff/sites/demo/draft")
	response, errorValue := invokeSiteCapabilityTool(t, service, capabilities.ToolInvokeRequest{
		ToolName:  "site.publish",
		Input:     json.RawMessage(`{"siteID":"site-1","message":"Update prototype"}`),
		Transport: transport,
		Context: capabilities.ToolInvokeContext{
			Platform:       "mattermost",
			ConversationID: "thread-1",
		},
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if strings.Join(requests, ",") != "POST /admin/api/sites/site-1/publish" {
		t.Fatalf("unexpected requests: %+v", requests)
	}
	if response.Status != "published" ||
		len(response.Effects) != 2 ||
		response.Effects[0].ID != "site-1" ||
		response.Effects[1].URL != "https://demo.example" {
		t.Fatalf("unexpected publish response: %+v", response)
	}
	if strings.Contains(string(response.Result), `"owner"`) || !strings.Contains(string(response.Result), `"sourceSHA256":"`+transport.SiteSourceBundle.SHA256+`"`) {
		t.Fatalf("expected canonical publish projection, got %s", response.Result)
	}
}

func TestSiteAppPreviewUsesExactSiteID(t *testing.T) {
	requests := []string{}
	service := Service{
		Configuration: Configuration{AdmindBaseURL: "http://admind.local"},
		HTTPClient: &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			requests = append(requests, request.Method+" "+request.URL.Path)
			switch {
			case request.Method == http.MethodPost && request.URL.String() == "http://admind.local/admin/api/sites/site-1/preview":
				return siteToolJSONResponse(`{"siteID":"site-1","status":"draft","sourceWorkspacePath":"/workspace/circles/staff/sites/demo/draft","previewID":"preview-1","previewURL":"https://demo.example/__preview/preview-1","previewExpiresAt":"2026-07-19T00:00:00Z","owner":"internal-only"}`), nil
			default:
				t.Fatalf("unexpected request %s %s", request.Method, request.URL.String())
				return nil, nil
			}
		})},
	}

	_, errorValue := invokeSiteCapabilityTool(t, service, capabilities.ToolInvokeRequest{
		ToolName:  "site.preview",
		Input:     json.RawMessage(`{"siteID":"site-1"}`),
		Transport: testSiteSourceBundleTransport("/workspace/circles/staff/sites/demo/draft"),
		Context: capabilities.ToolInvokeContext{
			Platform:       "mattermost",
			ConversationID: "thread-1",
		},
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if strings.Join(requests, ",") != "POST /admin/api/sites/site-1/preview" {
		t.Fatalf("unexpected requests: %+v", requests)
	}
}

func TestSiteAppStatusResolvesExactReference(t *testing.T) {
	requests := []string{}
	service := Service{
		Configuration: Configuration{AdmindBaseURL: "http://admind.local"},
		HTTPClient: &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			requests = append(requests, request.Method+" "+request.URL.Path)
			switch {
			case request.Method == http.MethodGet && request.URL.String() == "http://admind.local/admin/api/sites":
				return siteToolJSONResponse(`{"sites":[{"siteID":"site-1","slug":"demo","platform":"mattermost","conversationID":"thread-1","ownerIdentity":{"personID":"person-1"},"status":"draft"}]}`), nil
			case request.Method == http.MethodGet && request.URL.String() == "http://admind.local/admin/api/sites/site-1":
				return siteToolJSONResponse(`{"siteID":"site-1","slug":"demo","title":"Demo","status":"draft","sourceWorkspacePath":"home/sites/site-1","appWorkspacePath":"home/sites/site-1/app","ownerIdentity":{"personID":"person-1"}}`), nil
			default:
				t.Fatalf("unexpected request %s %s", request.Method, request.URL.String())
				return nil, nil
			}
		})},
	}

	response, errorValue := service.invokeSiteAppTool(context.Background(), capabilities.ToolInvokeRequest{
		ToolName: "site.status",
		Input:    json.RawMessage(`{"siteReference":"site-1"}`),
		Context: capabilities.ToolInvokeContext{
			RequesterPersonID: "person-1",
			Platform:          "mattermost",
			ConversationID:    "thread-1",
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

func TestSiteAppStatusReturnsNotFoundForUnknownSlug(t *testing.T) {
	service := Service{
		Configuration: Configuration{AdmindBaseURL: "http://admind.local"},
		HTTPClient: &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			switch {
			case request.Method == http.MethodGet && request.URL.String() == "http://admind.local/admin/api/sites":
				return siteToolJSONResponse(`{"sites":[{"siteID":"site-1","slug":"existing","platform":"mattermost","conversationID":"thread-1","status":"draft"}]}`), nil
			default:
				t.Fatalf("unexpected request %s %s", request.Method, request.URL.String())
				return nil, nil
			}
		})},
	}

	response, errorValue := service.invokeSiteAppTool(context.Background(), capabilities.ToolInvokeRequest{
		ToolName: "site.status",
		Input:    json.RawMessage(`{"siteReference":"brand-new-site"}`),
		Context: capabilities.ToolInvokeContext{
			Platform:       "mattermost",
			ConversationID: "thread-1",
		},
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	result := string(response.Result)
	if !strings.Contains(result, `"status":"not_found"`) || !strings.Contains(result, `"siteReference":"brand-new-site"`) {
		t.Fatalf("expected not_found status for unknown slug, got %s", result)
	}
	if response.Outcome != capabilities.ToolOutcomeFailed || !response.IsError || response.ErrorCode != "site_not_found" || response.FailureStage != "resolution" {
		t.Fatalf("expected typed not_found response, got %+v", response)
	}
}

func TestSiteAppStatusReturnsAmbiguousCandidates(t *testing.T) {
	service := Service{
		Configuration: Configuration{AdmindBaseURL: "http://admind.local"},
		HTTPClient: &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			switch {
			case request.Method == http.MethodGet && request.URL.String() == "http://admind.local/admin/api/sites":
				return siteToolJSONResponse(`{"sites":[{"siteID":"site-1","slug":"shared","title":"One","description":"First site","purpose":"portfolio","archetype":"portfolio","publishedURL":"https://one.example","platform":"mattermost","conversationID":"thread-1","ownerIdentity":{"personID":"person-1","displayName":"Owner"},"status":"draft"},{"siteID":"site-2","slug":"shared","title":"Two","description":"Second site","purpose":"booking","archetype":"booking","publishedURL":"https://two.example","platform":"mattermost","conversationID":"thread-1","ownerIdentity":{"personID":"person-1","displayName":"Owner"},"status":"draft"}]}`), nil
			default:
				t.Fatalf("unexpected request %s %s", request.Method, request.URL.String())
				return nil, nil
			}
		})},
	}

	response, errorValue := service.invokeSiteAppTool(context.Background(), capabilities.ToolInvokeRequest{
		ToolName: "site.status",
		Input:    json.RawMessage(`{"siteReference":"shared"}`),
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
	if response.Outcome != capabilities.ToolOutcomeFailed || !response.IsError || response.ErrorCode != "site_ambiguous" || response.FailureStage != "resolution" {
		t.Fatalf("expected typed ambiguous response, got %+v", response)
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
		ToolName: "site.status",
		Input:    json.RawMessage(`{"siteReference":"demo"}`),
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

func TestSiteAppStatusReferenceReturnsEditableCandidates(t *testing.T) {
	var requestedURL string
	service := Service{
		Configuration: Configuration{AdmindBaseURL: "http://admind.local"},
		HTTPClient: &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			requestedURL = request.URL.String()
			if request.Method != http.MethodGet || request.URL.Path != "/admin/api/sites" {
				t.Fatalf("unexpected request %s %s", request.Method, request.URL.String())
			}
			return siteToolJSONResponse(`{"sites":[{"siteID":"site-a","slug":"shared","title":"Alpha","status":"published","publishedURL":"https://alpha.example","ownerIdentity":{"personID":"person-a"},"conversationID":"thread-a","liveHTTPStatus":200},{"siteID":"site-b","slug":"shared","title":"Beta","status":"published","publishedURL":"https://beta.example","ownerIdentity":{"personID":"person-b"},"conversationID":"thread-b","liveHTTPStatus":503},{"siteID":"site-c","slug":"shared","title":"Shared","status":"failed","lastError":"build failed","ownerIdentity":{"personID":"person-b"},"collaborators":[{"personID":"person-a","role":"editor"}],"conversationID":"thread-c"}]}`), nil
		})},
	}

	response, errorValue := service.invokeSiteAppTool(context.Background(), capabilities.ToolInvokeRequest{
		ToolName: "site.status",
		Input:    json.RawMessage(`{"siteReference":"shared","checkLive":true}`),
		Context: capabilities.ToolInvokeContext{
			RequesterPersonID: "person-a",
			Platform:          "mattermost",
			ConversationID:    "thread-a",
		},
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	result := string(response.Result)
	if !strings.Contains(requestedURL, "checkLive=true") {
		t.Fatalf("expected checkLive query, got %s", requestedURL)
	}
	if !strings.Contains(result, `"siteID":"site-a"`) || !strings.Contains(result, `"siteID":"site-c"`) {
		t.Fatalf("expected editable sites in owner-wide response, got %s", result)
	}
	if strings.Contains(result, `"siteID":"site-b"`) || strings.Contains(result, "beta.example") {
		t.Fatalf("expected owner B site to be filtered, got %s", result)
	}
	if response.Outcome != capabilities.ToolOutcomeFailed || response.ErrorCode != "site_ambiguous" {
		t.Fatalf("expected typed ambiguous result, got %+v", response)
	}
}

func TestSiteAppStatusBySlugDoesNotExposeNonEditorSite(t *testing.T) {
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
		ToolName: "site.status",
		Input:    json.RawMessage(`{"siteReference":"demo"}`),
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
	if strings.Contains(result, "sourceWorkspacePath") || strings.Contains(result, "appWorkspacePath") || !strings.Contains(result, `"status":"not_found"`) {
		t.Fatalf("expected typed not_found without source paths, got %s", result)
	}
	if response.Outcome != capabilities.ToolOutcomeFailed || response.ErrorCode != "site_not_found" {
		t.Fatalf("expected typed not_found response, got %+v", response)
	}
}

func TestSiteAppStatusReferenceReturnsTypedNotFound(t *testing.T) {
	service := Service{
		Configuration: Configuration{AdmindBaseURL: "http://admind.local"},
		HTTPClient: &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			return siteToolJSONResponse(`{"sites":[]}`), nil
		})},
	}

	response, errorValue := service.invokeSiteAppTool(context.Background(), capabilities.ToolInvokeRequest{
		ToolName: "site.status",
		Input:    json.RawMessage(`{"siteReference":"missing-site"}`),
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if response.Outcome != capabilities.ToolOutcomeFailed || response.ErrorCode != "site_not_found" || !strings.Contains(string(response.Result), `"siteReference":"missing-site"`) {
		t.Fatalf("expected typed exact-ID miss, got %+v", response)
	}
}

func TestSiteAppDeleteReturnsCanonicalResultAndEffect(t *testing.T) {
	service := Service{
		Configuration: Configuration{AdmindBaseURL: "http://admind.local"},
		HTTPClient: &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			if request.Method != http.MethodDelete || request.URL.Path != "/admin/api/sites/site-1" {
				t.Fatalf("unexpected request %s %s", request.Method, request.URL.String())
			}
			var input map[string]any
			if errorValue := json.NewDecoder(request.Body).Decode(&input); errorValue != nil {
				t.Fatal(errorValue)
			}
			if input["confirm"] != "DELETE" || input["userConfirmed"] != true {
				t.Fatalf("expected internal approval proof, got %+v", input)
			}
			return siteToolJSONResponse(`{"siteID":"site-1","slug":"demo","status":"deleted","owner":"internal-only"}`), nil
		})},
	}

	response, errorValue := service.invokeSiteAppTool(context.Background(), capabilities.ToolInvokeRequest{
		ToolName: "site.delete",
		Input:    json.RawMessage(`{"siteID":"site-1","reason":"Remove obsolete launch page"}`),
		Context:  capabilities.ToolInvokeContext{IsApprovalContinuation: true},
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if string(response.Result) != `{"deleted":true,"siteID":"site-1"}` {
		t.Fatalf("unexpected delete result %s", response.Result)
	}
	if len(response.Effects) != 1 || response.Effects[0].ObjectType != "website" || response.Effects[0].Effect != "deleted" || response.Effects[0].ID != "site-1" {
		t.Fatalf("unexpected delete effects %+v", response.Effects)
	}
}

func TestSiteDeleteAdmindInputDoesNotTrustDirectConfirmation(t *testing.T) {
	input, errorValue := siteDeleteAdmindInput(json.RawMessage(`{"siteID":"site-1","confirm":"DELETE","userConfirmed":true}`), false)
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
			"siteID":"site-1",
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

func TestSiteAppRecordWithoutOwnerIsNotEditable(t *testing.T) {
	if siteAppRecordCanEdit(siteAppRecord{SiteID: "site-1"}, siteAppInput{Requester: siteAppIdentity{PersonID: "person-1"}}) {
		t.Fatal("expected an ownerless site record to fail closed")
	}
}

func TestSiteAppRejectsMismatchedMutationResult(t *testing.T) {
	service := Service{
		Configuration: Configuration{AdmindBaseURL: "http://admind.local"},
		HTTPClient: &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			return siteToolJSONResponse(`{"siteID":"site-2","status":"published","sourceWorkspacePath":"/workspace/circles/staff/sites/demo/draft","publishedURL":"https://demo.example","currentVersionID":"version-1"}`), nil
		})},
	}

	_, errorValue := service.invokeSiteAppTool(context.Background(), capabilities.ToolInvokeRequest{
		ToolName:  "site.publish",
		Input:     json.RawMessage(`{"siteID":"site-1"}`),
		Transport: testSiteSourceBundleTransport("/workspace/circles/staff/sites/demo/draft"),
	})
	if errorValue == nil || !strings.Contains(errorValue.Error(), "siteID does not match") {
		t.Fatalf("expected mismatched result rejection, got %v", errorValue)
	}
}

func TestSiteAppPublishRejectsInvalidSourceSHA256(t *testing.T) {
	for _, sourceSHA256 := range []string{
		"abc",
		"AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA",
	} {
		transport := testSiteSourceBundleTransport("/workspace/circles/staff/sites/demo/draft")
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
		ToolName: "site.create",
		Input:    json.RawMessage(`{"slug":"demo"}`),
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
