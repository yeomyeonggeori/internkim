package main

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	browserruntime "github.com/anthropic-lab/internkim/internal/browser"
	"github.com/anthropic-lab/internkim/internal/capabilities"
	companionruntime "github.com/anthropic-lab/internkim/internal/companion"
)

func TestDefaultCapabilitiesAdvertiseLLMOnlyInDevelopmentMockMode(t *testing.T) {
	withoutMockLLM := defaultCapabilities(true, false)
	withMockLLM := defaultCapabilities(true, true)

	if hasCapability(withoutMockLLM, "llm.structured") {
		t.Fatal("expected LLM capability to be hidden without development mock mode")
	}
	if !hasCapability(withMockLLM, "llm.structured") {
		t.Fatal("expected LLM capability in development mock mode")
	}
	if !hasCapability(withoutMockLLM, "browser.navigate") {
		t.Fatal("expected browser capability to be advertised")
	}
}

func TestMainSubcommandsReturnAfterExecution(t *testing.T) {
	for _, commandName := range []string{"pair", "run", "status"} {
		if !isCompanionSubcommand(commandName) {
			t.Fatalf("expected %s to be a subcommand", commandName)
		}
	}
	if isCompanionSubcommand("serve") {
		t.Fatal("serve should fall through to default server mode")
	}
}

func hasCapability(descriptors []capabilities.Descriptor, name string) bool {
	for _, descriptor := range descriptors {
		if descriptor.Name == name {
			return true
		}
	}
	return false
}

func TestPairSavesState(t *testing.T) {
	statePath := filepath.Join(t.TempDir(), "state.json")
	httpClient := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.URL.Path != "/_internkim/companion/pair" {
			t.Fatalf("unexpected pair path: %s", request.URL.Path)
		}
		return textResponse(http.StatusOK, `{"companionID":"companion-1","token":"token-1"}`), nil
	})}

	secureStore := companionruntime.NewMemorySecureStore()
	errorValue := runPairWithStore([]string{"--device-url", "https://device.intern.kim", "--code", "ABCD-1234", "--state", statePath, "--local-only", "--dev-mock-llm"}, httpClient, secureStore)
	if errorValue != nil {
		t.Fatalf("expected pair success: %v", errorValue)
	}
	state, errorValue := loadState(statePath)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if state.CompanionID != "companion-1" || state.Token != "token-1" || !state.LocalOnly {
		t.Fatalf("unexpected state: %+v", state)
	}
	if state.PrivateKey != "" || state.PrivateKeyID == "" {
		t.Fatalf("expected state to keep only private key reference: %+v", state)
	}
	if _, errorValue := secureStore.Get(nilContext(), state.PrivateKeyID); errorValue != nil {
		t.Fatalf("expected private key in secure store: %v", errorValue)
	}
	if !hasCapability(state.Capabilities, "llm.structured") {
		t.Fatal("expected development LLM capability to be stored")
	}
}

func TestPairAcceptsDeepLinkArgument(t *testing.T) {
	statePath := filepath.Join(t.TempDir(), "state.json")
	httpClient := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.URL.String() != "https://device.intern.kim/_internkim/companion/pair" {
			t.Fatalf("unexpected pair endpoint: %s", request.URL.String())
		}
		return textResponse(http.StatusOK, `{"companionID":"companion-1","token":"token-1"}`), nil
	})}

	secureStore := companionruntime.NewMemorySecureStore()
	errorValue := runPairWithStore([]string{"--state", statePath, "internkim://pair?device_url=https%3A%2F%2Fdevice.intern.kim&code=ABCD-1234"}, httpClient, secureStore)
	if errorValue != nil {
		t.Fatalf("expected deep link pair success: %v", errorValue)
	}
	state, errorValue := loadState(statePath)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if state.DeviceURL != "https://device.intern.kim" {
		t.Fatalf("unexpected device URL: %s", state.DeviceURL)
	}
}

func TestPairReportsHTMLInsteadOfRawJSONDecodeError(t *testing.T) {
	statePath := filepath.Join(t.TempDir(), "state.json")
	httpClient := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		return textResponse(http.StatusOK, `<html>Cloudflare Access</html>`), nil
	})}

	secureStore := companionruntime.NewMemorySecureStore()
	errorValue := runPairWithStore([]string{"--device-url", "https://device.intern.kim", "--code", "ABCD-1234", "--state", statePath}, httpClient, secureStore)
	if errorValue == nil {
		t.Fatal("expected pair to fail")
	}
	if !strings.Contains(errorValue.Error(), "returned HTML instead of JSON") {
		t.Fatalf("expected helpful HTML response error, got %v", errorValue)
	}
}

func TestRunOnceCompletesMockLLMJob(t *testing.T) {
	statePath := filepath.Join(t.TempDir(), "state.json")
	state, secureStore := testCompanionState(t, true, true)
	if errorValue := saveState(statePath, state); errorValue != nil {
		t.Fatal(errorValue)
	}
	seenComplete := false
	httpClient := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		switch request.URL.Path {
		case "/_internkim/companion/heartbeat":
			return textResponse(http.StatusOK, `{}`), nil
		case "/_internkim/companion/jobs/next":
			return textResponse(http.StatusOK, `{"jobID":"job-1","status":"running","request":{"toolName":"llm.structured","input":{"structuredOutputSchema":{"document":{"required":["reply"]}}}}}`), nil
		case "/_internkim/companion/jobs/job-1/complete":
			seenComplete = true
			return textResponse(http.StatusOK, `{}`), nil
		default:
			t.Fatalf("unexpected run path: %s", request.URL.Path)
			return nil, nil
		}
	})}

	errorValue := runCompanionWithStore([]string{"--state", statePath, "--once", "--dev-mock-llm"}, httpClient, secureStore)
	if errorValue != nil {
		t.Fatalf("expected run once success: %v", errorValue)
	}
	if !seenComplete {
		t.Fatal("expected companion to complete the job")
	}
}

func TestRunOnceCompletesShellBridgeConfirmJob(t *testing.T) {
	statePath := filepath.Join(t.TempDir(), "state.json")
	state, secureStore := testCompanionState(t, true, false)
	if errorValue := saveState(statePath, state); errorValue != nil {
		t.Fatal(errorValue)
	}
	seenComplete := false
	httpClient := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		switch request.URL.Path {
		case "/v1/user/confirm":
			return textResponse(http.StatusOK, `{"confirmed":true}`), nil
		case "/_internkim/companion/heartbeat":
			return textResponse(http.StatusOK, `{}`), nil
		case "/_internkim/companion/jobs/next":
			return textResponse(http.StatusOK, `{"jobID":"job-1","status":"running","request":{"toolName":"user.confirm","input":{"message":"continue?"}}}`), nil
		case "/_internkim/companion/jobs/job-1/complete":
			seenComplete = true
			return textResponse(http.StatusOK, `{}`), nil
		default:
			t.Fatalf("unexpected run path: %s", request.URL.Path)
			return nil, nil
		}
	})}

	errorValue := runCompanionWithStore([]string{"--state", statePath, "--once", "--shell-bridge-url", "http://127.0.0.1:1234"}, httpClient, secureStore)
	if errorValue != nil {
		t.Fatalf("expected run once success: %v", errorValue)
	}
	if !seenComplete {
		t.Fatal("expected companion to complete the bridge job")
	}
}

func TestRunOnceDeniesBrowserJobWithReason(t *testing.T) {
	statePath := filepath.Join(t.TempDir(), "state.json")
	state, secureStore := testCompanionState(t, true, false)
	if errorValue := saveState(statePath, state); errorValue != nil {
		t.Fatal(errorValue)
	}
	seenDeny := false
	httpClient := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		switch request.URL.Path {
		case "/v1/security/approval":
			return textResponse(http.StatusOK, `{"allowed":false,"userReason":"not now","suggestedConstraint":"ask for text"}`), nil
		case "/_internkim/companion/heartbeat":
			return textResponse(http.StatusOK, `{}`), nil
		case "/_internkim/companion/jobs/next":
			return textResponse(http.StatusOK, `{"jobID":"job-1","status":"running","toolName":"browser.navigate","resourceScope":{"kind":"web_origin","value":"https://github.com"},"request":{"toolName":"browser.navigate","input":{"url":"https://github.com"}}}`), nil
		case "/_internkim/companion/jobs/job-1/deny":
			seenDeny = true
			return textResponse(http.StatusOK, `{}`), nil
		default:
			t.Fatalf("unexpected run path: %s", request.URL.Path)
			return nil, nil
		}
	})}

	errorValue := runCompanionWithStore([]string{"--state", statePath, "--once", "--shell-bridge-url", "http://127.0.0.1:1234"}, httpClient, secureStore)
	if errorValue == nil {
		t.Fatal("expected denied browser job to return denial error")
	}
	if !seenDeny {
		t.Fatal("expected companion to send a denial result")
	}
}

func TestLegacyPrivateKeyStateMigratesToSecureStore(t *testing.T) {
	statePath := filepath.Join(t.TempDir(), "state.json")
	keyPair, errorValue := companionruntime.GenerateKeyPair()
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	state := companionState{
		DeviceURL:    "https://device.intern.kim",
		CompanionID:  "companion-1",
		Token:        "token-1",
		PublicKey:    keyPair.PublicKey,
		PrivateKey:   keyPair.PrivateKey,
		LocalOnly:    true,
		Capabilities: defaultCapabilities(true, false),
	}
	if errorValue := saveState(statePath, state); errorValue != nil {
		t.Fatal(errorValue)
	}
	secureStore := companionruntime.NewMemorySecureStore()

	migratedState, errorValue := loadStateAndMigrateSecrets(nilContext(), statePath, secureStore)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if migratedState.PrivateKey != "" || migratedState.PrivateKeyID == "" {
		t.Fatalf("expected migrated state without raw private key: %+v", migratedState)
	}
	storedPrivateKey, errorValue := secureStore.Get(nilContext(), migratedState.PrivateKeyID)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if storedPrivateKey != keyPair.PrivateKey {
		t.Fatal("expected secure store to contain migrated private key")
	}
	reloadedState, errorValue := loadState(statePath)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if reloadedState.PrivateKey != "" {
		t.Fatal("expected saved state to remove legacy private key")
	}
}

func TestControlHandlerListsAndRevokesGrants(t *testing.T) {
	grantStore := companionruntime.NewMemoryGrantStore()
	approvalHandler := companionruntime.ApprovalHandler(companionApprovalHandler{allowed: true})
	request := capabilities.ToolInvokeRequest{
		ToolName:      "browser.navigate",
		ResourceScope: capabilities.ResourceScope{Kind: "web_origin", Value: "https://github.com"},
	}
	if errorValue := grantStore.Authorize(context.Background(), companionruntime.JobEnvelope{
		JobID:         "job-1",
		ToolName:      "browser.navigate",
		ResourceScope: request.ResourceScope,
	}, request, approvalHandler); errorValue != nil {
		t.Fatal(errorValue)
	}
	runtimeStatus := &runtimeState{}
	runtimeStatus.recordHeartbeat(nil)
	handler := controlHandler(grantStore, runtimeStatus)

	listRequest := httptest.NewRequest(http.MethodGet, "/v1/security/grants", nil)
	listRequest.RemoteAddr = "127.0.0.1:1234"
	listResponse := httptest.NewRecorder()
	handler.ServeHTTP(listResponse, listRequest)
	if listResponse.Code != http.StatusOK {
		t.Fatalf("expected grant list success, got %d", listResponse.Code)
	}
	var listDocument grantListDocument
	if errorValue := json.Unmarshal(listResponse.Body.Bytes(), &listDocument); errorValue != nil {
		t.Fatal(errorValue)
	}
	if len(listDocument.Grants) != 1 {
		t.Fatalf("expected one grant, got %d", len(listDocument.Grants))
	}

	runtimeRequest := httptest.NewRequest(http.MethodGet, "/v1/runtime/status", nil)
	runtimeRequest.RemoteAddr = "127.0.0.1:1234"
	runtimeResponse := httptest.NewRecorder()
	handler.ServeHTTP(runtimeResponse, runtimeRequest)
	if runtimeResponse.Code != http.StatusOK || !strings.Contains(runtimeResponse.Body.String(), "lastHeartbeatAt") {
		t.Fatalf("expected runtime status, got %d %s", runtimeResponse.Code, runtimeResponse.Body.String())
	}

	revokeRequest := httptest.NewRequest(http.MethodPost, "/v1/security/grants/"+listDocument.Grants[0].GrantID+"/revoke", nil)
	revokeRequest.RemoteAddr = "127.0.0.1:1234"
	revokeResponse := httptest.NewRecorder()
	handler.ServeHTTP(revokeResponse, revokeRequest)
	if revokeResponse.Code != http.StatusOK {
		t.Fatalf("expected revoke success, got %d", revokeResponse.Code)
	}
	if len(grantStore.ListActive()) != 0 {
		t.Fatal("expected revoked grant to be inactive")
	}
}

func TestStatusRequiresPairedState(t *testing.T) {
	errorValue := runStatus([]string{"--state", filepath.Join(t.TempDir(), "missing.json")})
	if errorValue == nil {
		t.Fatal("expected missing state to fail")
	}
}

func TestStatusJSONReportsUnpairedState(t *testing.T) {
	errorValue := runStatus([]string{"--state", filepath.Join(t.TempDir(), "missing.json"), "--json"})
	if errorValue != nil {
		t.Fatalf("expected missing JSON status to succeed: %v", errorValue)
	}
}

func TestCompanionStatusFromState(t *testing.T) {
	state := companionState{
		DeviceURL:   "https://device.intern.kim",
		CompanionID: "companion-1",
		Token:       "token-1",
		LocalOnly:   true,
	}

	document, errorValue := json.Marshal(companionStatusFromState(state, browserruntime.RuntimeReadiness{Status: "ready"}))
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if !strings.Contains(string(document), `"paired":true`) {
		t.Fatalf("expected paired JSON, got %s", string(document))
	}
	if strings.Contains(string(document), "privateKey") || strings.Contains(string(document), "token-1") {
		t.Fatalf("expected status JSON to omit secrets, got %s", string(document))
	}
}

func TestCompanionStatusFiltersBrowserCapabilitiesWhenRuntimeUnavailable(t *testing.T) {
	state := companionState{
		DeviceURL:    "https://device.intern.kim",
		CompanionID:  "companion-1",
		Token:        "token-1",
		Capabilities: defaultCapabilities(false, false),
	}

	document := companionStatusFromState(state, browserruntime.RuntimeReadiness{
		Status: "unavailable",
		Error:  "companion browser runtime unavailable",
	})

	if hasCapability(document.Capabilities, "browser.navigate") {
		t.Fatal("expected browser capabilities to be hidden when runtime is unavailable")
	}
	if document.BrowserRuntimeStatus != "unavailable" {
		t.Fatalf("unexpected browser runtime status: %s", document.BrowserRuntimeStatus)
	}
	if strings.Contains(document.BrowserRuntimeError, "token-1") {
		t.Fatalf("expected sanitized browser runtime error, got %s", document.BrowserRuntimeError)
	}
}

func TestResolveAgentBrowserPathUsesFlagBeforeEnvironment(t *testing.T) {
	t.Setenv("INTERNKIM_AGENT_BROWSER_PATH", "/env/agent-browser")

	path := resolveAgentBrowserPath("/flag/agent-browser")

	if path != "/flag/agent-browser" {
		t.Fatalf("expected flag path, got %s", path)
	}
}

func TestResolveAgentBrowserPathUsesEnvironmentWhenFlagEmpty(t *testing.T) {
	t.Setenv("INTERNKIM_AGENT_BROWSER_PATH", "/env/agent-browser")

	path := resolveAgentBrowserPath("")

	if path != "/env/agent-browser" {
		t.Fatalf("expected environment path, got %s", path)
	}
}

func testCompanionState(t *testing.T, localOnly bool, devMockLLM bool) (companionState, *companionruntime.MemorySecureStore) {
	t.Helper()
	keyPair, errorValue := companionruntime.GenerateKeyPair()
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	secureStore := companionruntime.NewMemorySecureStore()
	privateKeyID := companionPrivateKeyID("companion-1")
	if errorValue := secureStore.Put(nilContext(), privateKeyID, keyPair.PrivateKey); errorValue != nil {
		t.Fatal(errorValue)
	}
	return companionState{
		DeviceURL:    "https://device.intern.kim",
		CompanionID:  "companion-1",
		Token:        "token-1",
		PublicKey:    keyPair.PublicKey,
		PrivateKeyID: privateKeyID,
		LocalOnly:    localOnly,
		Capabilities: defaultCapabilities(localOnly, devMockLLM),
	}, secureStore
}

func nilContext() context.Context {
	return context.Background()
}

type companionApprovalHandler struct {
	allowed bool
}

func (handler companionApprovalHandler) Approve(ctx context.Context, request companionruntime.ApprovalRequest) (companionruntime.ApprovalDecision, error) {
	_ = ctx
	_ = request
	return companionruntime.ApprovalDecision{Allowed: handler.allowed}, nil
}

func textResponse(statusCode int, body string) *http.Response {
	return &http.Response{
		StatusCode: statusCode,
		Body:       io.NopCloser(strings.NewReader(body)),
		Header:     make(http.Header),
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (function roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return function(request)
}

func TestMain(m *testing.M) {
	os.Exit(m.Run())
}
