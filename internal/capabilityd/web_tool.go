package capabilityd

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"

	"gitlab.com/eastriver/internkim/internal/capabilities"
)

type webSearchInput struct {
	Query           string   `json:"query"`
	Location        string   `json:"location"`
	Language        string   `json:"language"`
	Limit           int      `json:"limit"`
	AllowedDomains  []string `json:"allowedDomains"`
	ExcludedDomains []string `json:"excludedDomains"`
}

type webFetchInput struct {
	URLs             []string `json:"urls"`
	MaxContentTokens int      `json:"maxContentTokens"`
	AllowedDomains   []string `json:"allowedDomains"`
	BlockedDomains   []string `json:"blockedDomains"`
}

func (service Service) invokeWebTool(ctx context.Context, request capabilities.ToolInvokeRequest) (capabilities.ToolInvokeResponse, error) {
	if service.Configuration.LocalOnly {
		return webToolErrorResponse(request.ToolName, "remote web tools are disabled in local-only mode", "local_only", false), nil
	}
	apiKey := readSecretValue(service.Configuration.OpenRouterKeyPath)
	if strings.TrimSpace(apiKey) == "" || isPlaceholderOpenRouterKey(apiKey) {
		return webToolErrorResponse(request.ToolName, "OpenRouter API key is not configured", "missing_openrouter_key", false), nil
	}
	result, errorValue := service.invokeOpenRouterWebTool(ctx, request, apiKey)
	if errorValue != nil {
		return webToolErrorResponse(request.ToolName, errorValue.Error(), "openrouter_web_tool_failed", true), nil
	}
	return capabilities.ToolInvokeResponse{
		Provider:        "openrouter",
		SelectedBackend: capabilities.LLMBackendRemote,
		ToolName:        request.ToolName,
		Status:          "ok",
		Result:          result,
	}, nil
}

func (service Service) invokeOpenRouterWebTool(ctx context.Context, request capabilities.ToolInvokeRequest, apiKey string) (json.RawMessage, error) {
	switch strings.TrimSpace(request.ToolName) {
	case "web.search":
		input, errorValue := decodeWebSearchInput(request.Input)
		if errorValue != nil {
			return nil, errorValue
		}
		return service.invokeOpenRouterWebSearch(ctx, input, apiKey)
	case "web.fetch":
		input, errorValue := decodeWebFetchInput(request.Input)
		if errorValue != nil {
			return nil, errorValue
		}
		return service.invokeOpenRouterWebFetch(ctx, input, apiKey)
	default:
		return nil, errors.New("web tool is not configured: " + request.ToolName)
	}
}

func decodeWebSearchInput(document json.RawMessage) (webSearchInput, error) {
	var input webSearchInput
	if errorValue := json.Unmarshal(document, &input); errorValue != nil {
		return webSearchInput{}, errorValue
	}
	input.Query = strings.TrimSpace(input.Query)
	input.Location = strings.TrimSpace(input.Location)
	input.Language = strings.TrimSpace(input.Language)
	input.AllowedDomains = normalizedDomainList(input.AllowedDomains)
	input.ExcludedDomains = normalizedDomainList(input.ExcludedDomains)
	if input.Query == "" {
		return webSearchInput{}, errors.New("query is required")
	}
	if input.Limit == 0 {
		input.Limit = 5
	}
	if input.Limit < 1 || input.Limit > 10 {
		return webSearchInput{}, errors.New("limit must be between 1 and 10")
	}
	return input, nil
}

func decodeWebFetchInput(document json.RawMessage) (webFetchInput, error) {
	var input webFetchInput
	if errorValue := json.Unmarshal(document, &input); errorValue != nil {
		return webFetchInput{}, errorValue
	}
	input.AllowedDomains = normalizedDomainList(input.AllowedDomains)
	input.BlockedDomains = normalizedDomainList(input.BlockedDomains)
	if len(input.URLs) == 0 {
		return webFetchInput{}, errors.New("urls is required")
	}
	if len(input.URLs) > 10 {
		return webFetchInput{}, errors.New("urls must include at most 10 entries")
	}
	for index, rawURL := range input.URLs {
		normalizedURL, errorValue := validatePublicWebURL(rawURL)
		if errorValue != nil {
			return webFetchInput{}, errorValue
		}
		input.URLs[index] = normalizedURL
	}
	if input.MaxContentTokens == 0 {
		input.MaxContentTokens = 50000
	}
	if input.MaxContentTokens < 1000 || input.MaxContentTokens > 100000 {
		return webFetchInput{}, errors.New("maxContentTokens must be between 1000 and 100000")
	}
	return input, nil
}

func (service Service) invokeOpenRouterWebSearch(ctx context.Context, input webSearchInput, apiKey string) (json.RawMessage, error) {
	requestDocument := openRouterWebToolRequest(
		service.Configuration.OpenRouterModel,
		openRouterWebSearchMessages(input),
		openRouterWebSearchTool(input),
		openRouterWebSearchSchema(),
	)
	return service.sendOpenRouterWebRequest(ctx, apiKey, requestDocument, normalizeOpenRouterSearchContent(input))
}

func (service Service) invokeOpenRouterWebFetch(ctx context.Context, input webFetchInput, apiKey string) (json.RawMessage, error) {
	requestDocument := openRouterWebToolRequest(
		service.Configuration.OpenRouterModel,
		openRouterWebFetchMessages(input),
		openRouterWebFetchTool(input),
		openRouterWebFetchSchema(),
	)
	return service.sendOpenRouterWebRequest(ctx, apiKey, requestDocument, normalizeOpenRouterFetchContent(input))
}

func openRouterWebToolRequest(modelName string, messages []map[string]string, tool map[string]any, schema json.RawMessage) map[string]any {
	return map[string]any{
		"model":    modelName,
		"messages": messages,
		"tools":    []map[string]any{tool},
		"stream":   false,
		"response_format": map[string]any{
			"type": "json_schema",
			"json_schema": map[string]any{
				"name":   "web_tool_result",
				"strict": true,
				"schema": schema,
			},
		},
	}
}

func openRouterWebSearchTool(input webSearchInput) map[string]any {
	parameters := map[string]any{
		"engine":      "auto",
		"max_results": input.Limit,
	}
	if len(input.AllowedDomains) > 0 {
		parameters["allowed_domains"] = input.AllowedDomains
	}
	if len(input.ExcludedDomains) > 0 {
		parameters["excluded_domains"] = input.ExcludedDomains
	}
	return map[string]any{"type": "openrouter:web_search", "parameters": parameters}
}

func openRouterWebFetchTool(input webFetchInput) map[string]any {
	parameters := map[string]any{
		"engine":             "auto",
		"max_uses":           len(input.URLs),
		"max_content_tokens": input.MaxContentTokens,
	}
	if len(input.AllowedDomains) > 0 {
		parameters["allowed_domains"] = input.AllowedDomains
	}
	if len(input.BlockedDomains) > 0 {
		parameters["blocked_domains"] = input.BlockedDomains
	}
	return map[string]any{"type": "openrouter:web_fetch", "parameters": parameters}
}

func openRouterWebSearchMessages(input webSearchInput) []map[string]string {
	prompt := "Search the web for: " + input.Query + "\nReturn only the JSON schema result with provider openrouter and remoteLLMInvolved true."
	if input.Location != "" {
		prompt += "\nLocation hint: " + input.Location
	}
	if input.Language != "" {
		prompt += "\nLanguage hint: " + input.Language
	}
	return []map[string]string{
		{"role": "system", "content": "Use the OpenRouter web search server tool. Return concise structured search results. Do not include secrets."},
		{"role": "user", "content": prompt},
	}
}

func openRouterWebFetchMessages(input webFetchInput) []map[string]string {
	return []map[string]string{
		{"role": "system", "content": "Use the OpenRouter web fetch server tool. Return concise structured fetched page content. Do not include secrets."},
		{"role": "user", "content": "Fetch these URLs and return only the JSON schema result with provider openrouter and remoteLLMInvolved true:\n" + strings.Join(input.URLs, "\n")},
	}
}

type openRouterWebContentNormalizer func(string) (json.RawMessage, error)

type openRouterFetchDocumentResult struct {
	URL      string `json:"url"`
	FinalURL string `json:"finalURL"`
	Title    string `json:"title"`
	Content  string `json:"content"`
}

type openRouterFetchDocumentError struct {
	URL   string `json:"url"`
	Error string `json:"error"`
}

func (service Service) sendOpenRouterWebRequest(ctx context.Context, apiKey string, document map[string]any, normalizeContent openRouterWebContentNormalizer) (json.RawMessage, error) {
	requestBody, errorValue := json.Marshal(document)
	if errorValue != nil {
		return nil, errorValue
	}
	httpRequest, errorValue := http.NewRequestWithContext(ctx, http.MethodPost, service.Configuration.OpenRouterWebBaseURL, bytes.NewReader(requestBody))
	if errorValue != nil {
		return nil, errorValue
	}
	httpRequest.Header.Set("Authorization", "Bearer "+apiKey)
	httpRequest.Header.Set("Content-Type", "application/json")
	httpResponse, errorValue := service.httpClient().Do(httpRequest)
	if errorValue != nil {
		return nil, errorValue
	}
	defer httpResponse.Body.Close()
	responseBody, errorValue := io.ReadAll(httpResponse.Body)
	if errorValue != nil {
		return nil, errors.New("read openrouter web response: " + errorValue.Error())
	}
	if httpResponse.StatusCode >= http.StatusBadRequest {
		return nil, errors.New(safeProviderError(responseBody))
	}
	var parsed struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if errorValue := json.Unmarshal(responseBody, &parsed); errorValue != nil {
		return nil, errorValue
	}
	if len(parsed.Choices) == 0 {
		return nil, errors.New("openrouter web response did not include choices")
	}
	content := parsed.Choices[0].Message.Content
	if strings.TrimSpace(content) == "" {
		return nil, errors.New("openrouter web response content was empty")
	}
	return normalizeContent(content)
}

func normalizeOpenRouterSearchContent(input webSearchInput) openRouterWebContentNormalizer {
	return func(content string) (json.RawMessage, error) {
		embeddedContent, found := extractEmbeddedJSON(content)
		if !found {
			return marshalOpenRouterSearchTextResult(input, content)
		}
		if normalizedContent, isSearchSchemaJSON := normalizeOpenRouterSearchJSON(embeddedContent); isSearchSchemaJSON {
			return normalizedContent, nil
		}
		return marshalOpenRouterSearchTextResult(input, content)
	}
}

func normalizeOpenRouterFetchContent(input webFetchInput) openRouterWebContentNormalizer {
	return func(content string) (json.RawMessage, error) {
		embeddedContent, found := extractEmbeddedJSON(content)
		if !found {
			return marshalOpenRouterFetchTextResult(input, content)
		}
		return normalizeOpenRouterFetchJSON(string(embeddedContent), input)
	}
}

func extractEmbeddedJSON(content string) (json.RawMessage, bool) {
	trimmedContent := strings.TrimSpace(content)
	if json.Valid([]byte(trimmedContent)) {
		return json.RawMessage(trimmedContent), true
	}
	if fencedContent, found := stripMarkdownJSONFence(trimmedContent); found {
		return json.RawMessage(fencedContent), true
	}
	if balancedContent, found := firstBalancedJSONValue(trimmedContent); found {
		return json.RawMessage(balancedContent), true
	}
	return nil, false
}

func stripMarkdownJSONFence(content string) (string, bool) {
	if !strings.HasPrefix(content, "```") {
		return "", false
	}
	closingFenceIndex := strings.LastIndex(content, "```")
	if closingFenceIndex <= 3 {
		return "", false
	}
	fencedBody := content[3:closingFenceIndex]
	if newlineIndex := strings.IndexByte(fencedBody, '\n'); newlineIndex >= 0 {
		languageTag := strings.TrimSpace(fencedBody[:newlineIndex])
		if languageTag == "" || isJSONFenceLanguageTag(languageTag) {
			fencedBody = fencedBody[newlineIndex+1:]
		}
	}
	fencedBody = strings.TrimSpace(fencedBody)
	if !json.Valid([]byte(fencedBody)) {
		return "", false
	}
	return fencedBody, true
}

func isJSONFenceLanguageTag(tag string) bool {
	return strings.EqualFold(tag, "json") || strings.EqualFold(tag, "jsonc")
}

func firstBalancedJSONValue(content string) (string, bool) {
	startIndex := strings.IndexAny(content, "{[")
	if startIndex < 0 {
		return "", false
	}
	openCharacter := content[startIndex]
	closeCharacter := byte('}')
	if openCharacter == '[' {
		closeCharacter = ']'
	}
	depth := 0
	isInsideString := false
	isEscaped := false
	for index := startIndex; index < len(content); index++ {
		character := content[index]
		if isInsideString {
			switch {
			case isEscaped:
				isEscaped = false
			case character == '\\':
				isEscaped = true
			case character == '"':
				isInsideString = false
			}
			continue
		}
		switch character {
		case '"':
			isInsideString = true
		case openCharacter:
			depth++
		case closeCharacter:
			depth--
			if depth == 0 {
				candidate := content[startIndex : index+1]
				if json.Valid([]byte(candidate)) {
					return candidate, true
				}
				return "", false
			}
		}
	}
	return "", false
}

type openRouterSearchDocumentResult struct {
	Title   string `json:"title"`
	URL     string `json:"url"`
	Snippet string `json:"snippet"`
}

func normalizeOpenRouterSearchJSON(embeddedContent json.RawMessage) (json.RawMessage, bool) {
	var document struct {
		Provider string                           `json:"provider"`
		Query    string                           `json:"query"`
		Answer   string                           `json:"answer"`
		Results  []openRouterSearchDocumentResult `json:"results"`
	}
	if errorValue := json.Unmarshal(embeddedContent, &document); errorValue != nil {
		return nil, false
	}
	if !openRouterSearchJSONLooksValid(document.Provider, document.Query, document.Results) {
		return nil, false
	}
	return embeddedContent, true
}

func openRouterSearchJSONLooksValid(provider string, query string, results []openRouterSearchDocumentResult) bool {
	if strings.TrimSpace(provider) == "" || strings.TrimSpace(query) == "" {
		return false
	}
	for _, result := range results {
		if strings.TrimSpace(result.Title) == "" || strings.TrimSpace(result.URL) == "" || strings.TrimSpace(result.Snippet) == "" {
			return false
		}
	}
	return true
}

func marshalOpenRouterSearchTextResult(input webSearchInput, content string) (json.RawMessage, error) {
	document, errorValue := json.Marshal(map[string]any{
		"provider":          "openrouter",
		"remoteLLMInvolved": true,
		"compatibility":     "openrouter_server_tool_content_text",
		"query":             input.Query,
		"answer":            strings.TrimSpace(content),
		"results":           []map[string]string{},
	})
	return json.RawMessage(document), errorValue
}

func normalizeOpenRouterFetchJSON(content string, input webFetchInput) (json.RawMessage, error) {
	var document struct {
		Provider          string                          `json:"provider"`
		RemoteLLMInvolved bool                            `json:"remoteLLMInvolved"`
		Compatibility     string                          `json:"compatibility"`
		Results           []openRouterFetchDocumentResult `json:"results"`
		Errors            []openRouterFetchDocumentError  `json:"errors"`
	}
	if errorValue := json.Unmarshal([]byte(content), &document); errorValue == nil && openRouterFetchJSONLooksValid(document.Provider, document.Results, document.Errors) {
		return json.RawMessage(content), nil
	}
	return marshalOpenRouterFetchTextResult(input, content)
}

func openRouterFetchJSONLooksValid(provider string, results []openRouterFetchDocumentResult, errors []openRouterFetchDocumentError) bool {
	if strings.TrimSpace(provider) == "" || len(results)+len(errors) == 0 {
		return false
	}
	for _, result := range results {
		if strings.TrimSpace(result.URL) == "" || strings.TrimSpace(result.Title) == "" {
			return false
		}
	}
	for _, errorValue := range errors {
		if strings.TrimSpace(errorValue.URL) == "" || strings.TrimSpace(errorValue.Error) == "" {
			return false
		}
	}
	return true
}

func marshalOpenRouterFetchTextResult(input webFetchInput, content string) (json.RawMessage, error) {
	if len(input.URLs) > 1 {
		return marshalOpenRouterFetchCombinedTextResult(content)
	}
	urlValue := firstWebFetchURL(input)
	title := webFetchTitleFallback(urlValue)
	document, errorValue := json.Marshal(map[string]any{
		"provider":          "openrouter",
		"remoteLLMInvolved": true,
		"compatibility":     "openrouter_server_tool_content_text",
		"results": []map[string]string{{
			"url":      urlValue,
			"finalURL": urlValue,
			"title":    title,
			"content":  content,
		}},
		"errors": []map[string]string{},
	})
	return json.RawMessage(document), errorValue
}

func marshalOpenRouterFetchCombinedTextResult(content string) (json.RawMessage, error) {
	document, errorValue := json.Marshal(map[string]any{
		"provider":          "openrouter",
		"remoteLLMInvolved": true,
		"compatibility":     "openrouter_server_tool_combined_text",
		"results": []map[string]string{{
			"url":      "",
			"finalURL": "",
			"title":    "OpenRouter combined fetch content",
			"content":  content,
		}},
		"errors": []map[string]string{},
	})
	return json.RawMessage(document), errorValue
}

func firstWebFetchURL(input webFetchInput) string {
	if len(input.URLs) == 0 {
		return ""
	}
	return strings.TrimSpace(input.URLs[0])
}

func webFetchTitleFallback(rawURL string) string {
	parsedURL, errorValue := url.Parse(strings.TrimSpace(rawURL))
	if errorValue != nil || parsedURL.Hostname() == "" {
		return strings.TrimSpace(rawURL)
	}
	return parsedURL.Hostname()
}

func openRouterWebSearchSchema() json.RawMessage {
	return json.RawMessage(`{"type":"object","properties":{"provider":{"type":"string"},"remoteLLMInvolved":{"type":"boolean"},"compatibility":{"type":"string"},"query":{"type":"string"},"answer":{"type":"string"},"results":{"type":"array","items":{"type":"object","properties":{"title":{"type":"string"},"url":{"type":"string"},"snippet":{"type":"string"},"source":{"type":"string"}},"required":["title","url","snippet"],"additionalProperties":false}}},"required":["provider","remoteLLMInvolved","compatibility","query","answer","results"],"additionalProperties":false}`)
}

func openRouterWebFetchSchema() json.RawMessage {
	return json.RawMessage(`{"type":"object","properties":{"provider":{"type":"string"},"remoteLLMInvolved":{"type":"boolean"},"compatibility":{"type":"string"},"results":{"type":"array","items":{"type":"object","properties":{"url":{"type":"string"},"finalURL":{"type":"string"},"title":{"type":"string"},"content":{"type":"string"}},"required":["url","title","content"],"additionalProperties":false}},"errors":{"type":"array","items":{"type":"object","properties":{"url":{"type":"string"},"error":{"type":"string"}},"required":["url","error"],"additionalProperties":false}}},"required":["provider","remoteLLMInvolved","compatibility","results","errors"],"additionalProperties":false}`)
}

func validatePublicWebURL(rawURL string) (string, error) {
	parsedURL, errorValue := url.Parse(strings.TrimSpace(rawURL))
	if errorValue != nil || parsedURL.Hostname() == "" {
		return "", errors.New("url is invalid")
	}
	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return "", errors.New("url scheme must be http or https")
	}
	if isBlockedWebHost(parsedURL.Hostname()) {
		return "", errors.New("url host is not allowed")
	}
	parsedURL.Fragment = ""
	return parsedURL.String(), nil
}

func isBlockedWebHost(host string) bool {
	normalizedHost := strings.ToLower(strings.TrimSpace(host))
	if normalizedHost == "localhost" || strings.HasSuffix(normalizedHost, ".localhost") || normalizedHost == "metadata.google.internal" {
		return true
	}
	ipAddress := net.ParseIP(normalizedHost)
	if ipAddress == nil {
		return false
	}
	return ipAddress.IsLoopback() || ipAddress.IsPrivate() || ipAddress.IsLinkLocalUnicast() || ipAddress.IsLinkLocalMulticast() || ipAddress.IsUnspecified()
}

func normalizedDomainList(values []string) []string {
	result := []string{}
	for _, value := range values {
		trimmedValue := strings.ToLower(strings.TrimSpace(value))
		if trimmedValue != "" {
			result = append(result, trimmedValue)
		}
	}
	return result
}

func safeProviderError(document []byte) string {
	message := strings.TrimSpace(string(document))
	if message == "" {
		return "provider request failed"
	}
	if len(message) > 500 {
		return message[:500]
	}
	return message
}

func webToolErrorResponse(toolName string, message string, code string, retryable bool) capabilities.ToolInvokeResponse {
	result, _ := json.Marshal(map[string]any{
		"error":             message,
		"code":              code,
		"provider":          "openrouter",
		"remoteLLMInvolved": true,
		"compatibility":     "openrouter_server_tool_auto",
	})
	return capabilities.ToolInvokeResponse{
		Provider:        "openrouter",
		SelectedBackend: capabilities.LLMBackendRemote,
		ToolName:        toolName,
		Status:          "error",
		Content:         message,
		IsError:         true,
		ErrorCode:       code,
		Retryable:       retryable,
		SafeRetry:       retryable,
		Result:          result,
	}
}
