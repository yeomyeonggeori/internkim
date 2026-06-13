package llmbackend

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
)

const providerErrorMessageMaxLength = 300

var providerErrorURLPattern = regexp.MustCompile(`https?://[^\s)]+`)
var providerErrorTokenPattern = regexp.MustCompile(`(?i)\b(?:sk|or|pk|rk|pat|ghp|gho|ghu|ghs|xox[baprs])-?[A-Za-z0-9_.=-]{8,}\b`)

func normalizeProviderError(providerName string, httpStatus int, rawBody []byte) error {
	return errors.New(normalizedProviderErrorMessage(providerName, httpStatus, rawBody))
}

func normalizedProviderErrorMessage(providerName string, httpStatus int, rawBody []byte) string {
	message, code, isParsed := parseOpenAIProviderError(rawBody)
	if isParsed {
		return providerHTTPErrorMessage(providerName, httpStatus, message, code)
	}
	return providerHTTPFallbackMessage(providerName, httpStatus, rawBody)
}

func parseOpenAIProviderError(rawBody []byte) (string, string, bool) {
	var document struct {
		Error json.RawMessage `json:"error"`
	}
	if errorValue := json.Unmarshal(rawBody, &document); errorValue != nil {
		return "", "", false
	}
	if len(bytes.TrimSpace(document.Error)) == 0 {
		return "", "", false
	}
	return parseProviderErrorValue(document.Error)
}

func parseProviderErrorValue(rawError json.RawMessage) (string, string, bool) {
	var message string
	if errorValue := json.Unmarshal(rawError, &message); errorValue == nil {
		return compactProviderErrorMessage(message), "", strings.TrimSpace(message) != ""
	}

	var document struct {
		Message string `json:"message"`
		Code    any    `json:"code"`
	}
	if errorValue := json.Unmarshal(rawError, &document); errorValue != nil {
		return "", "", false
	}
	message = compactProviderErrorMessage(document.Message)
	if message == "" {
		return "", "", false
	}
	return message, providerErrorCodeText(document.Code), true
}

func providerHTTPErrorMessage(providerName string, httpStatus int, message string, code string) string {
	if code == "" {
		return fmt.Sprintf("%s: HTTP %d: %s", providerName, httpStatus, message)
	}
	return fmt.Sprintf("%s: HTTP %d: %s (%s)", providerName, httpStatus, message, code)
}

func providerHTTPFallbackMessage(providerName string, httpStatus int, rawBody []byte) string {
	if json.Valid(rawBody) {
		return fmt.Sprintf("%s: HTTP %d: (%d-byte unsupported JSON error response)", providerName, httpStatus, len(rawBody))
	}
	return fmt.Sprintf("%s: HTTP %d: (%d-byte non-JSON response)", providerName, httpStatus, len(rawBody))
}

func compactProviderErrorMessage(message string) string {
	compactedMessage := strings.Join(strings.Fields(message), " ")
	compactedMessage = providerErrorURLPattern.ReplaceAllString(compactedMessage, "<url>")
	compactedMessage = providerErrorTokenPattern.ReplaceAllString(compactedMessage, "<token>")
	if len(compactedMessage) <= providerErrorMessageMaxLength {
		return compactedMessage
	}
	return compactedMessage[:providerErrorMessageMaxLength] + "..."
}

func providerErrorCodeText(code any) string {
	switch value := code.(type) {
	case string:
		return compactProviderErrorMessage(value)
	case float64:
		return fmt.Sprintf("%g", value)
	case bool:
		return fmt.Sprintf("%t", value)
	default:
		return ""
	}
}
