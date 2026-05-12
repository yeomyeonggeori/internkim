package admind

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

type credentialProviderStatus struct {
	Provider    string `json:"provider"`
	Configured  bool   `json:"configured"`
	Fingerprint string `json:"fingerprint,omitempty"`
}

type credentialProvidersResponse struct {
	Providers []credentialProviderStatus `json:"providers"`
}

type openRouterKeyRequest struct {
	APIKey string `json:"apiKey"`
}

func (service *Service) writeCredentialProviders(responseWriter http.ResponseWriter) {
	service.writeJSON(responseWriter, credentialProvidersResponse{
		Providers: []credentialProviderStatus{service.openRouterProviderStatus()},
	})
}

func (service *Service) updateOpenRouterKey(responseWriter http.ResponseWriter, request *http.Request) {
	var payload openRouterKeyRequest
	if errorValue := json.NewDecoder(request.Body).Decode(&payload); errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusBadRequest)
		return
	}
	apiKey := strings.TrimSpace(payload.APIKey)
	if apiKey == "" {
		http.Error(responseWriter, "OpenRouter API key is required", http.StatusBadRequest)
		return
	}
	if errorValue := service.validateOpenRouterKey(request.Context(), apiKey); errorValue != nil {
		http.Error(responseWriter, "OpenRouter API key validation failed", http.StatusBadRequest)
		return
	}
	if errorValue := writeSecretFile(service.Configuration.OpenRouterKeyPath, apiKey); errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusInternalServerError)
		return
	}
	service.writeJSON(responseWriter, service.openRouterProviderStatus())
}

func (service *Service) deleteOpenRouterKey(responseWriter http.ResponseWriter) {
	errorValue := os.Remove(service.Configuration.OpenRouterKeyPath)
	if errorValue != nil && !errors.Is(errorValue, os.ErrNotExist) {
		http.Error(responseWriter, errorValue.Error(), http.StatusInternalServerError)
		return
	}
	service.writeJSON(responseWriter, service.openRouterProviderStatus())
}

func (service *Service) openRouterProviderStatus() credentialProviderStatus {
	apiKey := readTrimmedFile(service.Configuration.OpenRouterKeyPath)
	return credentialProviderStatus{
		Provider:    "openrouter",
		Configured:  apiKey != "",
		Fingerprint: secretFingerprint(apiKey),
	}
}

func (service *Service) validateOpenRouterKey(ctx context.Context, apiKey string) error {
	httpRequest, errorValue := http.NewRequestWithContext(ctx, http.MethodGet, service.Configuration.OpenRouterModelsURL, nil)
	if errorValue != nil {
		return errorValue
	}
	httpRequest.Header.Set("Authorization", "Bearer "+apiKey)
	httpResponse, errorValue := service.httpClient().Do(httpRequest)
	if errorValue != nil {
		return errorValue
	}
	defer httpResponse.Body.Close()
	_, _ = io.Copy(io.Discard, httpResponse.Body)
	if httpResponse.StatusCode >= http.StatusBadRequest {
		return errors.New("openrouter returned " + httpResponse.Status)
	}
	return nil
}

func writeSecretFile(path string, value string) error {
	if errorValue := os.MkdirAll(filepath.Dir(path), 0o700); errorValue != nil {
		return errorValue
	}
	temporaryPath := path + ".tmp"
	if errorValue := os.WriteFile(temporaryPath, []byte(strings.TrimSpace(value)), 0o600); errorValue != nil {
		return errorValue
	}
	return os.Rename(temporaryPath, path)
}

func secretFingerprint(value string) string {
	trimmedValue := strings.TrimSpace(value)
	if trimmedValue == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(trimmedValue))
	return "sha256:" + hex.EncodeToString(sum[:])[:12]
}
