package tenantruntime

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

const DefaultOpenRouterAPIKeysURL = "https://openrouter.ai/api/v1/keys"

type OpenRouterKeyProvisioner interface {
	CreateAPIKey(ctx context.Context, request OpenRouterAPIKeyCreateRequest) (OpenRouterProvisionedAPIKey, error)
}

type OpenRouterAPIKeyCreateRequest struct {
	Name               string
	LimitUSD           float64
	LimitReset         string
	ExpiresAt          string
	IncludeBYOKInLimit bool
}

type OpenRouterProvisionedAPIKey struct {
	APIKey string
	Hash   string
	Label  string
}

type HTTPOpenRouterKeyProvisioner struct {
	ManagementKeyPath string
	APIKeysURL        string
	HTTPClient        *http.Client
}

func (provisioner HTTPOpenRouterKeyProvisioner) CreateAPIKey(ctx context.Context, request OpenRouterAPIKeyCreateRequest) (OpenRouterProvisionedAPIKey, error) {
	if strings.TrimSpace(request.Name) == "" {
		return OpenRouterProvisionedAPIKey{}, errors.New("openrouter key name is required")
	}
	managementKey := strings.TrimSpace(readOptionalTenantFile(provisioner.ManagementKeyPath))
	if managementKey == "" {
		return OpenRouterProvisionedAPIKey{}, errors.New("openrouter management key is required")
	}
	requestDocument, errorValue := json.Marshal(openRouterAPIKeyCreatePayload(request))
	if errorValue != nil {
		return OpenRouterProvisionedAPIKey{}, errorValue
	}
	httpRequest, errorValue := http.NewRequestWithContext(ctx, http.MethodPost, provisioner.apiKeysURL(), bytes.NewReader(requestDocument))
	if errorValue != nil {
		return OpenRouterProvisionedAPIKey{}, errorValue
	}
	httpRequest.Header.Set("Authorization", "Bearer "+managementKey)
	httpRequest.Header.Set("Content-Type", "application/json")
	httpResponse, errorValue := provisioner.httpClient().Do(httpRequest)
	if errorValue != nil {
		return OpenRouterProvisionedAPIKey{}, errorValue
	}
	defer httpResponse.Body.Close()
	responseDocument, errorValue := io.ReadAll(httpResponse.Body)
	if errorValue != nil {
		return OpenRouterProvisionedAPIKey{}, errorValue
	}
	if httpResponse.StatusCode >= http.StatusBadRequest {
		return OpenRouterProvisionedAPIKey{}, errors.New("openrouter key create failed: " + httpResponse.Status)
	}
	return parseOpenRouterProvisionedAPIKey(responseDocument)
}

func openRouterAPIKeyCreatePayload(request OpenRouterAPIKeyCreateRequest) map[string]any {
	payload := map[string]any{
		"name":                  strings.TrimSpace(request.Name),
		"include_byok_in_limit": request.IncludeBYOKInLimit,
	}
	if request.LimitUSD > 0 {
		payload["limit"] = request.LimitUSD
	}
	if strings.TrimSpace(request.LimitReset) != "" {
		payload["limit_reset"] = strings.TrimSpace(request.LimitReset)
	}
	if strings.TrimSpace(request.ExpiresAt) != "" {
		payload["expires_at"] = strings.TrimSpace(request.ExpiresAt)
	}
	return payload
}

func parseOpenRouterProvisionedAPIKey(document []byte) (OpenRouterProvisionedAPIKey, error) {
	var response struct {
		Key  string `json:"key"`
		Data struct {
			Hash  string `json:"hash"`
			Label string `json:"label"`
		} `json:"data"`
	}
	if errorValue := json.Unmarshal(document, &response); errorValue != nil {
		return OpenRouterProvisionedAPIKey{}, errorValue
	}
	if strings.TrimSpace(response.Key) == "" {
		return OpenRouterProvisionedAPIKey{}, errors.New("openrouter key create response did not include key")
	}
	return OpenRouterProvisionedAPIKey{
		APIKey: strings.TrimSpace(response.Key),
		Hash:   strings.TrimSpace(response.Data.Hash),
		Label:  strings.TrimSpace(response.Data.Label),
	}, nil
}

func (provisioner HTTPOpenRouterKeyProvisioner) apiKeysURL() string {
	if strings.TrimSpace(provisioner.APIKeysURL) != "" {
		return strings.TrimSpace(provisioner.APIKeysURL)
	}
	return DefaultOpenRouterAPIKeysURL
}

func (provisioner HTTPOpenRouterKeyProvisioner) httpClient() *http.Client {
	if provisioner.HTTPClient != nil {
		return provisioner.HTTPClient
	}
	return &http.Client{Timeout: 30 * time.Second}
}

func readOptionalTenantFile(path string) string {
	if strings.TrimSpace(path) == "" {
		return ""
	}
	document, errorValue := os.ReadFile(strings.TrimSpace(path))
	if errorValue != nil {
		return ""
	}
	return strings.TrimSpace(string(document))
}
