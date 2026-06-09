package llmgateway

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

type Configuration struct {
	ProviderChatCompletionsURL string
	ProviderEmbeddingsURL      string
	ProviderAPIKeyPath         string
	DeviceTokensPath           string
	LedgerPath                 string
	MinimumRequestMicrounits   int64
	PromptMicrounitsPerMillion int64
	OutputMicrounitsPerMillion int64
}

type DeviceToken struct {
	Token                    string `json:"token"`
	TenantID                 string `json:"tenantID"`
	DeviceID                 string `json:"deviceID"`
	ProviderAPIKey           string `json:"providerAPIKey,omitempty"`
	IsRevoked                bool   `json:"isRevoked,omitempty"`
	HardLimitMicrounits      int64  `json:"hardLimitMicrounits"`
	MinimumRequestMicrounits int64  `json:"minimumRequestMicrounits,omitempty"`
	RequestsPerMinute        int    `json:"requestsPerMinute,omitempty"`
}

type UsageRecord struct {
	TenantID            string    `json:"tenantID"`
	DeviceID            string    `json:"deviceID"`
	Model               string    `json:"model"`
	PromptTokens        int64     `json:"promptTokens"`
	CompletionTokens    int64     `json:"completionTokens"`
	TotalTokens         int64     `json:"totalTokens"`
	EstimatedMicrounits int64     `json:"estimatedMicrounits"`
	CreatedAt           time.Time `json:"createdAt"`
}

type TokenStore interface {
	FindDeviceToken(token string) (DeviceToken, bool)
}

type UsageLedger interface {
	TotalMicrounits(tenantID string, deviceID string) int64
	Append(record UsageRecord) error
}

type RateLimiter interface {
	Allow(deviceToken DeviceToken, createdAt time.Time) bool
}

type Service struct {
	Configuration Configuration
	TokenStore    TokenStore
	UsageLedger   UsageLedger
	RateLimiter   RateLimiter
	HTTPClient    *http.Client
}

func (service Service) Handler() http.Handler {
	multiplexer := http.NewServeMux()
	multiplexer.HandleFunc("POST /api/v1/chat/completions", service.handleChatCompletions)
	multiplexer.HandleFunc("POST /v1/chat/completions", service.handleChatCompletions)
	multiplexer.HandleFunc("POST /api/v1/embeddings", service.handleEmbeddings)
	multiplexer.HandleFunc("POST /v1/embeddings", service.handleEmbeddings)
	multiplexer.HandleFunc("GET /health", service.handleHealth)
	return multiplexer
}

func (service Service) handleHealth(responseWriter http.ResponseWriter, request *http.Request) {
	_ = request
	responseWriter.Header().Set("Content-Type", "application/json")
	_, _ = responseWriter.Write([]byte(`{"status":"ok"}`))
}

func (service Service) handleChatCompletions(responseWriter http.ResponseWriter, request *http.Request) {
	service.handleProviderProxy(responseWriter, request, service.providerChatCompletionsURL())
}

func (service Service) handleEmbeddings(responseWriter http.ResponseWriter, request *http.Request) {
	service.handleProviderProxy(responseWriter, request, service.providerEmbeddingsURL())
}

func (service Service) handleProviderProxy(responseWriter http.ResponseWriter, request *http.Request, providerURL string) {
	deviceToken, errorValue := service.authenticate(request)
	if errorValue != nil {
		writeJSONError(responseWriter, http.StatusUnauthorized, errorValue.Error())
		return
	}
	if errorValue := service.checkQuotaBeforeProvider(deviceToken); errorValue != nil {
		writeJSONError(responseWriter, http.StatusPaymentRequired, errorValue.Error())
		return
	}
	if errorValue := service.checkRateLimitBeforeProvider(deviceToken); errorValue != nil {
		writeJSONError(responseWriter, http.StatusTooManyRequests, errorValue.Error())
		return
	}
	requestDocument, errorValue := io.ReadAll(request.Body)
	if errorValue != nil {
		writeJSONError(responseWriter, http.StatusBadRequest, "read request body: "+errorValue.Error())
		return
	}
	providerResponse, responseDocument, errorValue := service.proxyProviderRequest(request.Context(), deviceToken, providerURL, requestDocument)
	if errorValue != nil {
		writeJSONError(responseWriter, http.StatusBadGateway, errorValue.Error())
		return
	}
	if providerResponse.StatusCode >= http.StatusBadRequest {
		writeProxyResponse(responseWriter, providerResponse.StatusCode, responseDocument)
		return
	}
	if errorValue := service.recordUsage(deviceToken, requestDocument, responseDocument); errorValue != nil {
		writeJSONError(responseWriter, http.StatusBadGateway, "record usage: "+errorValue.Error())
		return
	}
	writeProxyResponse(responseWriter, providerResponse.StatusCode, responseDocument)
}

func (service Service) authenticate(request *http.Request) (DeviceToken, error) {
	token := strings.TrimSpace(strings.TrimPrefix(request.Header.Get("Authorization"), "Bearer "))
	if token == "" {
		return DeviceToken{}, errors.New("device token is required")
	}
	deviceToken, found := service.tokenStore().FindDeviceToken(token)
	if !found || deviceToken.IsRevoked {
		return DeviceToken{}, errors.New("device token is revoked or unknown")
	}
	return deviceToken, nil
}

func (service Service) checkQuotaBeforeProvider(deviceToken DeviceToken) error {
	hardLimitMicrounits := deviceToken.HardLimitMicrounits
	if hardLimitMicrounits <= 0 {
		return nil
	}
	usedMicrounits := service.usageLedger().TotalMicrounits(deviceToken.TenantID, deviceToken.DeviceID)
	minimumRequestMicrounits := firstPositiveInt64(deviceToken.MinimumRequestMicrounits, service.Configuration.MinimumRequestMicrounits, 1)
	if usedMicrounits+minimumRequestMicrounits > hardLimitMicrounits {
		return errors.New("tenant llm quota is exhausted")
	}
	return nil
}

func (service Service) checkRateLimitBeforeProvider(deviceToken DeviceToken) error {
	if deviceToken.RequestsPerMinute <= 0 {
		return nil
	}
	if service.rateLimiter().Allow(deviceToken, time.Now().UTC()) {
		return nil
	}
	return errors.New("tenant llm rate limit is exceeded")
}

func (service Service) proxyProviderRequest(ctx context.Context, deviceToken DeviceToken, providerURL string, requestDocument []byte) (*http.Response, []byte, error) {
	providerAPIKey := service.providerAPIKey(deviceToken)
	if providerAPIKey == "" {
		return nil, nil, errors.New("provider api key is not configured")
	}
	providerRequest, errorValue := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		providerURL,
		bytes.NewReader(requestDocument),
	)
	if errorValue != nil {
		return nil, nil, errorValue
	}
	providerRequest.Header.Set("Authorization", "Bearer "+providerAPIKey)
	providerRequest.Header.Set("Content-Type", "application/json")
	providerResponse, errorValue := service.httpClient().Do(providerRequest)
	if errorValue != nil {
		return nil, nil, errorValue
	}
	defer providerResponse.Body.Close()
	responseDocument, errorValue := io.ReadAll(providerResponse.Body)
	if errorValue != nil {
		return nil, nil, errorValue
	}
	return providerResponse, responseDocument, nil
}

func (service Service) providerChatCompletionsURL() string {
	return strings.TrimSpace(service.Configuration.ProviderChatCompletionsURL)
}

func (service Service) providerEmbeddingsURL() string {
	if strings.TrimSpace(service.Configuration.ProviderEmbeddingsURL) != "" {
		return strings.TrimSpace(service.Configuration.ProviderEmbeddingsURL)
	}
	chatURL := strings.TrimSpace(service.Configuration.ProviderChatCompletionsURL)
	if strings.HasSuffix(chatURL, "/chat/completions") {
		return strings.TrimSuffix(chatURL, "/chat/completions") + "/embeddings"
	}
	return strings.TrimRight(chatURL, "/") + "/embeddings"
}

func (service Service) providerAPIKey(deviceToken DeviceToken) string {
	if strings.TrimSpace(deviceToken.ProviderAPIKey) != "" {
		return strings.TrimSpace(deviceToken.ProviderAPIKey)
	}
	return strings.TrimSpace(readOptionalFile(service.Configuration.ProviderAPIKeyPath))
}

func (service Service) recordUsage(deviceToken DeviceToken, requestDocument []byte, responseDocument []byte) error {
	record := UsageRecord{
		TenantID:  deviceToken.TenantID,
		DeviceID:  deviceToken.DeviceID,
		Model:     requestModel(requestDocument),
		CreatedAt: time.Now().UTC(),
	}
	record.PromptTokens, record.CompletionTokens, record.TotalTokens = responseUsage(responseDocument)
	record.EstimatedMicrounits = service.estimateMicrounits(record)
	if record.EstimatedMicrounits <= 0 {
		record.EstimatedMicrounits = firstPositiveInt64(deviceToken.MinimumRequestMicrounits, service.Configuration.MinimumRequestMicrounits, 1)
	}
	return service.usageLedger().Append(record)
}

func (service Service) estimateMicrounits(record UsageRecord) int64 {
	promptRate := service.Configuration.PromptMicrounitsPerMillion
	outputRate := service.Configuration.OutputMicrounitsPerMillion
	if promptRate <= 0 && outputRate <= 0 {
		return record.TotalTokens
	}
	return (record.PromptTokens*promptRate + record.CompletionTokens*outputRate) / 1_000_000
}

func (service Service) tokenStore() TokenStore {
	if service.TokenStore != nil {
		return service.TokenStore
	}
	return FileTokenStore{Path: service.Configuration.DeviceTokensPath}
}

func (service Service) usageLedger() UsageLedger {
	if service.UsageLedger != nil {
		return service.UsageLedger
	}
	return NewFileUsageLedger(service.Configuration.LedgerPath)
}

func (service Service) rateLimiter() RateLimiter {
	if service.RateLimiter != nil {
		return service.RateLimiter
	}
	return NoopRateLimiter{}
}

func (service Service) httpClient() *http.Client {
	if service.HTTPClient != nil {
		return service.HTTPClient
	}
	return http.DefaultClient
}

type NoopRateLimiter struct{}

func (limiter NoopRateLimiter) Allow(deviceToken DeviceToken, createdAt time.Time) bool {
	_ = deviceToken
	_ = createdAt
	return true
}

type MemoryRateLimiter struct {
	mutex         sync.Mutex
	requestsByKey map[string][]time.Time
}

func NewMemoryRateLimiter() *MemoryRateLimiter {
	return &MemoryRateLimiter{requestsByKey: map[string][]time.Time{}}
}

func (limiter *MemoryRateLimiter) Allow(deviceToken DeviceToken, createdAt time.Time) bool {
	if deviceToken.RequestsPerMinute <= 0 {
		return true
	}
	limiter.mutex.Lock()
	defer limiter.mutex.Unlock()
	key := rateLimitKey(deviceToken)
	requests := recentRequests(limiter.requestsByKey[key], createdAt)
	if len(requests) >= deviceToken.RequestsPerMinute {
		limiter.requestsByKey[key] = requests
		return false
	}
	limiter.requestsByKey[key] = append(requests, createdAt)
	return true
}

func rateLimitKey(deviceToken DeviceToken) string {
	return deviceToken.TenantID + ":" + deviceToken.DeviceID + ":" + deviceToken.Token
}

func recentRequests(requests []time.Time, createdAt time.Time) []time.Time {
	windowStart := createdAt.Add(-time.Minute)
	recentRequests := []time.Time{}
	for _, requestTime := range requests {
		if requestTime.After(windowStart) {
			recentRequests = append(recentRequests, requestTime)
		}
	}
	return recentRequests
}

func requestModel(requestDocument []byte) string {
	var document struct {
		Model string `json:"model"`
	}
	_ = json.Unmarshal(requestDocument, &document)
	return strings.TrimSpace(document.Model)
}

func responseUsage(responseDocument []byte) (int64, int64, int64) {
	var document struct {
		Usage struct {
			PromptTokens     int64 `json:"prompt_tokens"`
			CompletionTokens int64 `json:"completion_tokens"`
			TotalTokens      int64 `json:"total_tokens"`
		} `json:"usage"`
	}
	_ = json.Unmarshal(responseDocument, &document)
	totalTokens := document.Usage.TotalTokens
	if totalTokens == 0 {
		totalTokens = document.Usage.PromptTokens + document.Usage.CompletionTokens
	}
	return document.Usage.PromptTokens, document.Usage.CompletionTokens, totalTokens
}

func writeProxyResponse(responseWriter http.ResponseWriter, statusCode int, responseDocument []byte) {
	responseWriter.Header().Set("Content-Type", "application/json")
	responseWriter.WriteHeader(statusCode)
	_, _ = responseWriter.Write(responseDocument)
}

func writeJSONError(responseWriter http.ResponseWriter, statusCode int, message string) {
	responseWriter.Header().Set("Content-Type", "application/json")
	responseWriter.WriteHeader(statusCode)
	_ = json.NewEncoder(responseWriter).Encode(map[string]string{"error": message})
}

func readOptionalFile(path string) string {
	document, errorValue := os.ReadFile(strings.TrimSpace(path))
	if errorValue != nil {
		return ""
	}
	return strings.TrimSpace(string(document))
}

func firstPositiveInt64(values ...int64) int64 {
	for _, value := range values {
		if value > 0 {
			return value
		}
	}
	return 0
}

type MemoryTokenStore struct {
	DeviceTokens []DeviceToken
}

func (store MemoryTokenStore) FindDeviceToken(token string) (DeviceToken, bool) {
	for _, deviceToken := range store.DeviceTokens {
		if deviceToken.Token == token {
			return deviceToken, true
		}
	}
	return DeviceToken{}, false
}

type MemoryUsageLedger struct {
	Records []UsageRecord
}

func (ledger *MemoryUsageLedger) TotalMicrounits(tenantID string, deviceID string) int64 {
	var totalMicrounits int64
	for _, record := range ledger.Records {
		if record.TenantID == tenantID && record.DeviceID == deviceID {
			totalMicrounits += record.EstimatedMicrounits
		}
	}
	return totalMicrounits
}

func (ledger *MemoryUsageLedger) Append(record UsageRecord) error {
	ledger.Records = append(ledger.Records, record)
	return nil
}

type FileTokenStore struct {
	Path string
}

func (store FileTokenStore) FindDeviceToken(token string) (DeviceToken, bool) {
	var document struct {
		DeviceTokens []DeviceToken `json:"deviceTokens"`
	}
	if errorValue := readJSONFile(store.Path, &document); errorValue != nil {
		return DeviceToken{}, false
	}
	return MemoryTokenStore{DeviceTokens: document.DeviceTokens}.FindDeviceToken(token)
}

type FileUsageLedger struct {
	Path  string
	mutex *sync.Mutex
}

func NewFileUsageLedger(path string) *FileUsageLedger {
	return &FileUsageLedger{Path: path, mutex: &sync.Mutex{}}
}

func (ledger *FileUsageLedger) TotalMicrounits(tenantID string, deviceID string) int64 {
	records := ledger.readRecords()
	return (&MemoryUsageLedger{Records: records}).TotalMicrounits(tenantID, deviceID)
}

func (ledger *FileUsageLedger) Append(record UsageRecord) error {
	if strings.TrimSpace(ledger.Path) == "" {
		return nil
	}
	ledger.mutex.Lock()
	defer ledger.mutex.Unlock()
	document, errorValue := json.Marshal(record)
	if errorValue != nil {
		return errorValue
	}
	file, errorValue := os.OpenFile(ledger.Path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if errorValue != nil {
		return errorValue
	}
	defer file.Close()
	_, errorValue = file.Write(append(document, '\n'))
	return errorValue
}

func (ledger *FileUsageLedger) readRecords() []UsageRecord {
	document, errorValue := os.ReadFile(strings.TrimSpace(ledger.Path))
	if errorValue != nil {
		return nil
	}
	var records []UsageRecord
	for _, line := range strings.Split(string(document), "\n") {
		var record UsageRecord
		if json.Unmarshal([]byte(strings.TrimSpace(line)), &record) == nil && record.TenantID != "" {
			records = append(records, record)
		}
	}
	return records
}

func readJSONFile(path string, target any) error {
	document, errorValue := os.ReadFile(strings.TrimSpace(path))
	if errorValue != nil {
		return errorValue
	}
	return json.Unmarshal(document, target)
}
