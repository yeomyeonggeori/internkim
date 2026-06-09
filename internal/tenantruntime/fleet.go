package tenantruntime

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

type FleetCreateOptions struct {
	Count                       int
	StartIndex                  int
	TenantPrefix                string
	DisplayNamePrefix           string
	AssignedHost                string
	PublicURLTemplate           string
	MirrorHost                  string
	GatewayTokensPath           string
	HardLimitMicrounits         int64
	RequestsPerMinute           int
	MattermostPortStart         int
	OpenRouterManagementKeyPath string
	OpenRouterAPIKeysURL        string
	OpenRouterKeyLimitUSD       float64
	OpenRouterKeyLimitReset     string
	OpenRouterKeyExpiresAt      string
}

type FleetTenantStatus struct {
	TenantID                   string `json:"tenantID"`
	DisplayName                string `json:"displayName"`
	MattermostURL              string `json:"mattermostURL"`
	OpenRouterAPIKey           string `json:"openRouterAPIKey"`
	ProviderOpenRouterAPIKey   string `json:"providerOpenRouterAPIKey,omitempty"`
	ProviderOpenRouterKeyHash  string `json:"providerOpenRouterKeyHash,omitempty"`
	ProviderOpenRouterKeyLabel string `json:"providerOpenRouterKeyLabel,omitempty"`
	AdminUsername              string `json:"adminUsername"`
	AdminPassword              string `json:"adminPassword"`
}

func (service Service) CreateCloudSharedFleet(options FleetCreateOptions) ([]FleetTenantStatus, error) {
	options = normalizeFleetCreateOptions(options)
	if errorValue := validateFleetCreateOptions(options); errorValue != nil {
		return nil, errorValue
	}
	statuses := []FleetTenantStatus{}
	for index := options.StartIndex; index < options.StartIndex+options.Count; index++ {
		tenantID := fmt.Sprintf("%s-%02d", strings.TrimSpace(options.TenantPrefix), index)
		displayName := fmt.Sprintf("%s %02d", strings.TrimSpace(options.DisplayNamePrefix), index)
		mattermostURL := strings.ReplaceAll(strings.TrimSpace(options.PublicURLTemplate), "{tenant}", tenantID)
		manifest, errorValue := NewCloudSharedManifest(tenantID, displayName, options.AssignedHost, mattermostURL, options.MirrorHost)
		if errorValue != nil {
			return nil, errorValue
		}
		if errorValue := validateMattermostInstanceURL(mattermostURL, tenantID); errorValue != nil {
			return nil, errorValue
		}
		manifest.MattermostInstance = MattermostInstance{
			PublicURL:    mattermostURL,
			InternalURL:  fmt.Sprintf("http://127.0.0.1:%d", mattermostPortForFleetIndex(options, index)),
			Port:         mattermostPortForFleetIndex(options, index),
			DatabaseName: mattermostDatabaseName(tenantID),
		}
		if _, errorValue := service.CreateTenant(manifest); errorValue != nil {
			return nil, errorValue
		}
		credentials, errorValue := GenerateTenantCredentials()
		if errorValue != nil {
			return nil, errorValue
		}
		provisionedAPIKey, errorValue := service.provisionOpenRouterAPIKey(context.Background(), options, manifest)
		if errorValue != nil {
			return nil, errorValue
		}
		credentials.ProviderOpenRouterAPIKey = provisionedAPIKey.APIKey
		credentials.ProviderOpenRouterKeyHash = provisionedAPIKey.Hash
		credentials.ProviderOpenRouterKeyLabel = provisionedAPIKey.Label
		paths, errorValue := BuildRuntimePaths(service.BasePath, manifest.TenantID)
		if errorValue != nil {
			return nil, errorValue
		}
		if errorValue := installTenantCredentials(paths, credentials); errorValue != nil {
			return nil, errorValue
		}
		statuses = append(statuses, FleetTenantStatus{
			TenantID:                   manifest.TenantID,
			DisplayName:                manifest.DisplayName,
			MattermostURL:              manifest.PublicURL,
			OpenRouterAPIKey:           credentials.OpenRouterAPIKey,
			ProviderOpenRouterAPIKey:   credentials.ProviderOpenRouterAPIKey,
			ProviderOpenRouterKeyHash:  credentials.ProviderOpenRouterKeyHash,
			ProviderOpenRouterKeyLabel: credentials.ProviderOpenRouterKeyLabel,
			AdminUsername:              credentials.AdminUsername,
			AdminPassword:              credentials.AdminPassword,
		})
	}
	if strings.TrimSpace(options.GatewayTokensPath) != "" {
		if errorValue := writeFleetGatewayTokens(options, statuses); errorValue != nil {
			return nil, errorValue
		}
	}
	return statuses, nil
}

func normalizeFleetCreateOptions(options FleetCreateOptions) FleetCreateOptions {
	if options.StartIndex <= 0 {
		options.StartIndex = 1
	}
	return options
}

func validateFleetCreateOptions(options FleetCreateOptions) error {
	if options.Count < 1 {
		return errors.New("fleet count must be at least 1")
	}
	if options.StartIndex < 1 {
		return errors.New("fleet start index must be at least 1")
	}
	if strings.TrimSpace(options.TenantPrefix) == "" {
		return errors.New("tenant prefix is required")
	}
	if strings.TrimSpace(options.DisplayNamePrefix) == "" {
		return errors.New("display name prefix is required")
	}
	if !strings.Contains(options.PublicURLTemplate, "{tenant}") {
		return errors.New("public URL template must include {tenant}")
	}
	if options.MattermostPortStart <= 0 {
		return errors.New("mattermost port start is required")
	}
	if openRouterKeyProvisioningIsEnabled(options) && strings.TrimSpace(options.GatewayTokensPath) == "" {
		return errors.New("gateway tokens path is required when openrouter key provisioning is enabled")
	}
	return nil
}

func validateMattermostInstanceURL(value string, tenantID string) error {
	parsedURL, errorValue := url.Parse(strings.TrimSpace(value))
	if errorValue != nil || parsedURL.Scheme == "" || parsedURL.Host == "" {
		return errors.New("mattermost public URL must be absolute")
	}
	if parsedURL.Path != "" && parsedURL.Path != "/" {
		return errors.New("mattermost public URL must point to a tenant instance host, not a shared instance path")
	}
	if !strings.Contains(parsedURL.Hostname(), tenantID) {
		return errors.New("mattermost public URL hostname must include the tenant id")
	}
	return nil
}

var mattermostDatabaseCharacterPattern = regexp.MustCompile(`[^a-z0-9_]`)

func mattermostDatabaseName(tenantID string) string {
	return "mattermost_" + mattermostDatabaseCharacterPattern.ReplaceAllString(strings.ReplaceAll(tenantID, "-", "_"), "_")
}

func mattermostPortForFleetIndex(options FleetCreateOptions, index int) int {
	return options.MattermostPortStart + index - options.StartIndex
}

func (service Service) provisionOpenRouterAPIKey(ctx context.Context, options FleetCreateOptions, manifest Manifest) (OpenRouterProvisionedAPIKey, error) {
	if !openRouterKeyProvisioningIsEnabled(options) {
		return OpenRouterProvisionedAPIKey{}, nil
	}
	return service.openRouterKeyProvisioner(options).CreateAPIKey(ctx, OpenRouterAPIKeyCreateRequest{
		Name:               "internkim-" + manifest.TenantID,
		LimitUSD:           options.OpenRouterKeyLimitUSD,
		LimitReset:         options.OpenRouterKeyLimitReset,
		ExpiresAt:          options.OpenRouterKeyExpiresAt,
		IncludeBYOKInLimit: false,
	})
}

func (service Service) openRouterKeyProvisioner(options FleetCreateOptions) OpenRouterKeyProvisioner {
	if service.OpenRouterKeyProvisioner != nil {
		return service.OpenRouterKeyProvisioner
	}
	return HTTPOpenRouterKeyProvisioner{
		ManagementKeyPath: options.OpenRouterManagementKeyPath,
		APIKeysURL:        options.OpenRouterAPIKeysURL,
	}
}

func openRouterKeyProvisioningIsEnabled(options FleetCreateOptions) bool {
	return strings.TrimSpace(options.OpenRouterManagementKeyPath) != ""
}

func writeFleetGatewayTokens(options FleetCreateOptions, statuses []FleetTenantStatus) error {
	document := struct {
		DeviceTokens []struct {
			Token               string `json:"token"`
			TokenHash           string `json:"tokenHash"`
			TenantID            string `json:"tenantID"`
			DeviceID            string `json:"deviceID"`
			ProviderAPIKey      string `json:"providerAPIKey,omitempty"`
			HardLimitMicrounits int64  `json:"hardLimitMicrounits"`
			RequestsPerMinute   int    `json:"requestsPerMinute,omitempty"`
		} `json:"deviceTokens"`
	}{}
	for _, status := range statuses {
		document.DeviceTokens = append(document.DeviceTokens, struct {
			Token               string `json:"token"`
			TokenHash           string `json:"tokenHash"`
			TenantID            string `json:"tenantID"`
			DeviceID            string `json:"deviceID"`
			ProviderAPIKey      string `json:"providerAPIKey,omitempty"`
			HardLimitMicrounits int64  `json:"hardLimitMicrounits"`
			RequestsPerMinute   int    `json:"requestsPerMinute,omitempty"`
		}{
			Token:               status.OpenRouterAPIKey,
			TokenHash:           sha256Hex(status.OpenRouterAPIKey),
			TenantID:            status.TenantID,
			DeviceID:            "cloud-shared-" + status.TenantID,
			ProviderAPIKey:      status.ProviderOpenRouterAPIKey,
			HardLimitMicrounits: options.HardLimitMicrounits,
			RequestsPerMinute:   options.RequestsPerMinute,
		})
	}
	documentBytes, errorValue := json.MarshalIndent(document, "", "  ")
	if errorValue != nil {
		return errorValue
	}
	path := filepath.Clean(options.GatewayTokensPath)
	if errorValue := os.MkdirAll(filepath.Dir(path), 0o700); errorValue != nil {
		return errorValue
	}
	return os.WriteFile(path, append(documentBytes, '\n'), 0o600)
}

func sha256Hex(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}
