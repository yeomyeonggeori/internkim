package tenantruntime

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
)

const TenantInitialAdminUsername = "admin"

type TenantCredentials struct {
	OpenRouterAPIKey           string `json:"openRouterAPIKey"`
	ProviderOpenRouterAPIKey   string `json:"providerOpenRouterAPIKey,omitempty"`
	ProviderOpenRouterKeyHash  string `json:"providerOpenRouterKeyHash,omitempty"`
	ProviderOpenRouterKeyLabel string `json:"providerOpenRouterKeyLabel,omitempty"`
	AdminUsername              string `json:"adminUsername"`
	AdminPassword              string `json:"adminPassword"`
}

func GenerateTenantCredentials() (TenantCredentials, error) {
	openRouterAPIKey, errorValue := randomSecret("ik_or_", 32)
	if errorValue != nil {
		return TenantCredentials{}, errorValue
	}
	adminPassword, errorValue := randomSecret("", 24)
	if errorValue != nil {
		return TenantCredentials{}, errorValue
	}
	return TenantCredentials{
		OpenRouterAPIKey: openRouterAPIKey,
		AdminUsername:    TenantInitialAdminUsername,
		AdminPassword:    adminPassword,
	}, nil
}

func randomSecret(prefix string, byteCount int) (string, error) {
	if byteCount < 16 {
		return "", errors.New("random secret must be at least 16 bytes")
	}
	document := make([]byte, byteCount)
	if _, errorValue := rand.Read(document); errorValue != nil {
		return "", errorValue
	}
	return prefix + base64.RawURLEncoding.EncodeToString(document), nil
}
