package llmgateway

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidDeviceTokenProxiesOpenRouterCompatibleRequest(t *testing.T) {
	providerAuthorization := ""
	providerRequestBody := ""
	provider := httptest.NewServer(http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		providerAuthorization = request.Header.Get("Authorization")
		document, _ := io.ReadAll(request.Body)
		providerRequestBody = string(document)
		responseWriter.Header().Set("Content-Type", "application/json")
		_, _ = responseWriter.Write([]byte(`{"choices":[{"message":{"content":"hello"}}],"usage":{"prompt_tokens":7,"completion_tokens":5,"total_tokens":12}}`))
	}))
	defer provider.Close()

	ledger := &MemoryUsageLedger{}
	gateway := httptest.NewServer(newTestService(t, provider.URL, ledger).Handler())
	defer gateway.Close()

	response := postChatCompletion(t, gateway.URL, "device-token", `{"model":"google/test","messages":[{"role":"user","content":"hi"}]}`)

	if response.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", response.Code, response.Body)
	}
	if providerAuthorization != "Bearer provider-secret" {
		t.Fatalf("expected provider authorization to use provider key, got %q", providerAuthorization)
	}
	if !strings.Contains(providerRequestBody, `"model":"google/test"`) {
		t.Fatalf("expected provider request to preserve body, got %s", providerRequestBody)
	}
	if len(ledger.Records) != 1 {
		t.Fatalf("expected one usage record, got %d", len(ledger.Records))
	}
	record := ledger.Records[0]
	if record.TenantID != "tenant-a" || record.DeviceID != "device-a" || record.Model != "google/test" {
		t.Fatalf("unexpected usage record identity: %+v", record)
	}
	if record.PromptTokens != 7 || record.CompletionTokens != 5 || record.TotalTokens != 12 || record.EstimatedMicrounits != 12 {
		t.Fatalf("unexpected usage accounting: %+v", record)
	}
}

func TestDeviceTokenProviderKeyCanReplaceGatewayMasterKey(t *testing.T) {
	providerAuthorization := ""
	provider := httptest.NewServer(http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		providerAuthorization = request.Header.Get("Authorization")
		responseWriter.Header().Set("Content-Type", "application/json")
		_, _ = responseWriter.Write([]byte(`{"choices":[],"usage":{"total_tokens":1}}`))
	}))
	defer provider.Close()

	service := newTestService(t, provider.URL, &MemoryUsageLedger{})
	service.Configuration.ProviderAPIKeyPath = filepath.Join(t.TempDir(), "missing-provider-key")
	service.TokenStore = MemoryTokenStore{DeviceTokens: []DeviceToken{{
		Token:               "device-token",
		TenantID:            "tenant-a",
		DeviceID:            "device-a",
		ProviderAPIKey:      "sk-or-v1-tenant-a",
		HardLimitMicrounits: 100,
	}}}
	gateway := httptest.NewServer(service.Handler())
	defer gateway.Close()

	response := postChatCompletion(t, gateway.URL, "device-token", `{"model":"google/test","messages":[]}`)

	if response.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", response.Code, response.Body)
	}
	if providerAuthorization != "Bearer sk-or-v1-tenant-a" {
		t.Fatalf("expected device-scoped provider key, got %q", providerAuthorization)
	}
}

func TestRevokedDeviceTokenIsRejectedBeforeProviderCall(t *testing.T) {
	providerCalls := 0
	provider := httptest.NewServer(http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		providerCalls += 1
		responseWriter.WriteHeader(http.StatusOK)
	}))
	defer provider.Close()

	service := newTestService(t, provider.URL, &MemoryUsageLedger{})
	service.TokenStore = MemoryTokenStore{DeviceTokens: []DeviceToken{{
		Token:               "revoked-token",
		TenantID:            "tenant-a",
		DeviceID:            "device-a",
		IsRevoked:           true,
		HardLimitMicrounits: 100,
	}}}
	gateway := httptest.NewServer(service.Handler())
	defer gateway.Close()

	response := postChatCompletion(t, gateway.URL, "revoked-token", `{"model":"google/test","messages":[]}`)

	if response.Code != http.StatusUnauthorized {
		t.Fatalf("expected status 401, got %d: %s", response.Code, response.Body)
	}
	if providerCalls != 0 {
		t.Fatalf("expected provider not to be called, got %d calls", providerCalls)
	}
}

func TestHardCapBlocksProviderCall(t *testing.T) {
	providerCalls := 0
	provider := httptest.NewServer(http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		providerCalls += 1
		responseWriter.WriteHeader(http.StatusOK)
	}))
	defer provider.Close()

	ledger := &MemoryUsageLedger{Records: []UsageRecord{{
		TenantID:            "tenant-a",
		DeviceID:            "device-a",
		EstimatedMicrounits: 100,
	}}}
	gateway := httptest.NewServer(newTestService(t, provider.URL, ledger).Handler())
	defer gateway.Close()

	response := postChatCompletion(t, gateway.URL, "device-token", `{"model":"google/test","messages":[]}`)

	if response.Code != http.StatusPaymentRequired {
		t.Fatalf("expected status 402, got %d: %s", response.Code, response.Body)
	}
	if providerCalls != 0 {
		t.Fatalf("expected provider not to be called, got %d calls", providerCalls)
	}
	if len(ledger.Records) != 1 {
		t.Fatalf("expected no new usage record, got %d", len(ledger.Records))
	}
}

func TestRequestsPerMinuteBlocksProviderCall(t *testing.T) {
	providerCalls := 0
	provider := httptest.NewServer(http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		providerCalls += 1
		responseWriter.Header().Set("Content-Type", "application/json")
		_, _ = responseWriter.Write([]byte(`{"choices":[],"usage":{"total_tokens":1}}`))
	}))
	defer provider.Close()

	ledger := &MemoryUsageLedger{}
	service := newTestService(t, provider.URL, ledger)
	service.RateLimiter = NewMemoryRateLimiter()
	service.TokenStore = MemoryTokenStore{DeviceTokens: []DeviceToken{{
		Token:               "limited-token",
		TenantID:            "tenant-a",
		DeviceID:            "device-a",
		HardLimitMicrounits: 100,
		RequestsPerMinute:   1,
	}}}
	gateway := httptest.NewServer(service.Handler())
	defer gateway.Close()

	firstResponse := postChatCompletion(t, gateway.URL, "limited-token", `{"model":"google/test","messages":[]}`)
	secondResponse := postChatCompletion(t, gateway.URL, "limited-token", `{"model":"google/test","messages":[]}`)

	if firstResponse.Code != http.StatusOK {
		t.Fatalf("expected first status 200, got %d: %s", firstResponse.Code, firstResponse.Body)
	}
	if secondResponse.Code != http.StatusTooManyRequests {
		t.Fatalf("expected second status 429, got %d: %s", secondResponse.Code, secondResponse.Body)
	}
	if providerCalls != 1 {
		t.Fatalf("expected provider to be called once, got %d calls", providerCalls)
	}
	if len(ledger.Records) != 1 {
		t.Fatalf("expected one usage record, got %d", len(ledger.Records))
	}
}

func TestFileTokenStoreReadsDeviceScopedTokens(t *testing.T) {
	temporaryDirectory := t.TempDir()
	tokenPath := filepath.Join(temporaryDirectory, "tokens.json")
	document := `{"deviceTokens":[{"token":"device-token","tenantID":"tenant-a","deviceID":"device-a","hardLimitMicrounits":100,"requestsPerMinute":30}]}`
	if errorValue := os.WriteFile(tokenPath, []byte(document), 0o600); errorValue != nil {
		t.Fatal(errorValue)
	}

	deviceToken, found := (FileTokenStore{Path: tokenPath}).FindDeviceToken("device-token")

	if !found {
		t.Fatal("expected device token to be found")
	}
	if deviceToken.TenantID != "tenant-a" || deviceToken.DeviceID != "device-a" || deviceToken.RequestsPerMinute != 30 {
		t.Fatalf("unexpected device token: %+v", deviceToken)
	}
}

type gatewayResponse struct {
	Code int
	Body string
}

func newTestService(t *testing.T, providerURL string, ledger *MemoryUsageLedger) Service {
	t.Helper()
	temporaryDirectory := t.TempDir()
	providerKeyPath := filepath.Join(temporaryDirectory, "provider-key")
	if errorValue := os.WriteFile(providerKeyPath, []byte("provider-secret"), 0o600); errorValue != nil {
		t.Fatal(errorValue)
	}
	return Service{
		Configuration: Configuration{
			ProviderChatCompletionsURL: providerURL,
			ProviderAPIKeyPath:         providerKeyPath,
			MinimumRequestMicrounits:   1,
		},
		TokenStore: MemoryTokenStore{DeviceTokens: []DeviceToken{{
			Token:               "device-token",
			TenantID:            "tenant-a",
			DeviceID:            "device-a",
			HardLimitMicrounits: 100,
		}}},
		UsageLedger: ledger,
	}
}

func postChatCompletion(t *testing.T, baseURL string, token string, body string) gatewayResponse {
	t.Helper()
	request, errorValue := http.NewRequest(http.MethodPost, baseURL+"/api/v1/chat/completions", strings.NewReader(body))
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	request.Header.Set("Authorization", "Bearer "+token)
	request.Header.Set("Content-Type", "application/json")
	response, errorValue := http.DefaultClient.Do(request)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	defer response.Body.Close()
	var parsedBody map[string]any
	_ = json.NewDecoder(response.Body).Decode(&parsedBody)
	document, _ := json.Marshal(parsedBody)
	return gatewayResponse{Code: response.StatusCode, Body: string(document)}
}
