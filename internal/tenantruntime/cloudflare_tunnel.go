package tenantruntime

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
)

const DefaultCloudflareAPIBaseURL = "https://api.cloudflare.com/client/v4"

type CloudflareTunnelSyncOptions struct {
	AccountID              string
	TunnelID               string
	APIToken               string
	APITokenPath           string
	APIBaseURL             string
	PublicHostnameTemplate string
	TenantIDs              []string
}

type CloudflareTunnelSyncStatus struct {
	TenantIDs    []string `json:"tenantIDs"`
	IngressCount int      `json:"ingressCount"`
}

type CloudflareTunnelRemoveOptions struct {
	AccountID              string
	TunnelID               string
	APIToken               string
	APITokenPath           string
	APIBaseURL             string
	PublicHostnameTemplate string
	TenantIDs              []string
	Manifests              []Manifest
}

type CloudflareTunnelRemovalStatus struct {
	TenantIDs           []string `json:"tenantIDs"`
	RemovedIngressCount int      `json:"removedIngressCount"`
	IngressCount        int      `json:"ingressCount"`
}

type cloudflareTunnelConfigurationResponse struct {
	Success bool                                  `json:"success"`
	Result  cloudflareTunnelConfigurationResource `json:"result"`
	Errors  []cloudflareAPIMessage                `json:"errors"`
}

type cloudflareTunnelConfigurationResource struct {
	Configuration cloudflareTunnelConfiguration `json:"config"`
}

type cloudflareTunnelConfiguration struct {
	Ingress []cloudflareTunnelIngress `json:"ingress"`
}

type cloudflareTunnelIngress struct {
	Hostname string `json:"hostname,omitempty"`
	Path     string `json:"path,omitempty"`
	Service  string `json:"service"`
}

type cloudflareAPIMessage struct {
	Message string `json:"message"`
}

func (service Service) SyncCloudflareTenantTunnel(ctx context.Context, options CloudflareTunnelSyncOptions) (CloudflareTunnelSyncStatus, error) {
	if errorValue := validateCloudflareTunnelSyncOptions(options); errorValue != nil {
		return CloudflareTunnelSyncStatus{}, errorValue
	}
	token, errorValue := cloudflareAPIToken(options)
	if errorValue != nil {
		return CloudflareTunnelSyncStatus{}, errorValue
	}
	manifests, errorValue := service.readCloudflareTunnelTenantManifests(options.TenantIDs)
	if errorValue != nil {
		return CloudflareTunnelSyncStatus{}, errorValue
	}
	client := cloudflareTunnelAPIClient{httpClient: http.DefaultClient, baseURL: firstNonEmptyTenantString(options.APIBaseURL, DefaultCloudflareAPIBaseURL), accountID: options.AccountID, tunnelID: options.TunnelID, apiToken: token}
	configuration, errorValue := client.fetchConfiguration(ctx)
	if errorValue != nil {
		return CloudflareTunnelSyncStatus{}, errorValue
	}
	configuration.Ingress = mergedCloudflareTenantIngress(configuration.Ingress, manifests, options.PublicHostnameTemplate)
	if errorValue := client.updateConfiguration(ctx, configuration); errorValue != nil {
		return CloudflareTunnelSyncStatus{}, errorValue
	}
	return CloudflareTunnelSyncStatus{TenantIDs: manifestTenantIDs(manifests), IngressCount: len(configuration.Ingress)}, nil
}

func (service Service) RemoveCloudflareTenantTunnelIngress(ctx context.Context, options CloudflareTunnelRemoveOptions) (CloudflareTunnelRemovalStatus, error) {
	if errorValue := validateCloudflareTunnelRemoveOptions(options); errorValue != nil {
		return CloudflareTunnelRemovalStatus{}, errorValue
	}
	token, errorValue := cloudflareAPIToken(CloudflareTunnelSyncOptions{APIToken: options.APIToken, APITokenPath: options.APITokenPath})
	if errorValue != nil {
		return CloudflareTunnelRemovalStatus{}, errorValue
	}
	manifests, errorValue := service.cloudflareTunnelRemoveManifests(options)
	if errorValue != nil {
		return CloudflareTunnelRemovalStatus{}, errorValue
	}
	client := cloudflareTunnelAPIClient{httpClient: http.DefaultClient, baseURL: firstNonEmptyTenantString(options.APIBaseURL, DefaultCloudflareAPIBaseURL), accountID: options.AccountID, tunnelID: options.TunnelID, apiToken: token}
	configuration, errorValue := client.fetchConfiguration(ctx)
	if errorValue != nil {
		return CloudflareTunnelRemovalStatus{}, errorValue
	}
	updatedIngress, removedCount := removedCloudflareTenantIngress(configuration.Ingress, manifests, options.PublicHostnameTemplate)
	configuration.Ingress = updatedIngress
	if errorValue := client.updateConfiguration(ctx, configuration); errorValue != nil {
		return CloudflareTunnelRemovalStatus{}, errorValue
	}
	return CloudflareTunnelRemovalStatus{TenantIDs: manifestTenantIDs(manifests), RemovedIngressCount: removedCount, IngressCount: len(configuration.Ingress)}, nil
}

func validateCloudflareTunnelSyncOptions(options CloudflareTunnelSyncOptions) error {
	if strings.TrimSpace(options.AccountID) == "" {
		return errors.New("Cloudflare account id is required")
	}
	if strings.TrimSpace(options.TunnelID) == "" {
		return errors.New("Cloudflare tunnel id is required")
	}
	if len(options.TenantIDs) == 0 {
		return errors.New("at least one tenant id is required")
	}
	if strings.TrimSpace(options.APIToken) == "" && strings.TrimSpace(options.APITokenPath) == "" {
		return errors.New("Cloudflare API token or token path is required")
	}
	return nil
}

func validateCloudflareTunnelRemoveOptions(options CloudflareTunnelRemoveOptions) error {
	if strings.TrimSpace(options.AccountID) == "" {
		return errors.New("Cloudflare account id is required")
	}
	if strings.TrimSpace(options.TunnelID) == "" {
		return errors.New("Cloudflare tunnel id is required")
	}
	if len(options.TenantIDs) == 0 && len(options.Manifests) == 0 {
		return errors.New("at least one tenant id is required")
	}
	if strings.TrimSpace(options.APIToken) == "" && strings.TrimSpace(options.APITokenPath) == "" {
		return errors.New("Cloudflare API token or token path is required")
	}
	return nil
}

func cloudflareAPIToken(options CloudflareTunnelSyncOptions) (string, error) {
	if strings.TrimSpace(options.APIToken) != "" {
		return strings.TrimSpace(options.APIToken), nil
	}
	document, errorValue := os.ReadFile(options.APITokenPath)
	if errorValue != nil {
		return "", errorValue
	}
	token := strings.TrimSpace(string(document))
	if token == "" {
		return "", errors.New("Cloudflare API token file is empty")
	}
	return token, nil
}

func (service Service) readCloudflareTunnelTenantManifests(tenantIDs []string) ([]Manifest, error) {
	manifests := make([]Manifest, 0, len(tenantIDs))
	for _, tenantID := range tenantIDs {
		manifest, errorValue := service.ReadManifest(tenantID)
		if errorValue != nil {
			return nil, errorValue
		}
		manifests = append(manifests, manifest)
	}
	return manifests, nil
}

func (service Service) cloudflareTunnelRemoveManifests(options CloudflareTunnelRemoveOptions) ([]Manifest, error) {
	if len(options.Manifests) > 0 {
		return append([]Manifest{}, options.Manifests...), nil
	}
	return service.readCloudflareTunnelTenantManifests(options.TenantIDs)
}

type cloudflareTunnelAPIClient struct {
	httpClient *http.Client
	baseURL    string
	accountID  string
	tunnelID   string
	apiToken   string
}

func (client cloudflareTunnelAPIClient) fetchConfiguration(ctx context.Context) (cloudflareTunnelConfiguration, error) {
	request, errorValue := client.newRequest(ctx, http.MethodGet, nil)
	if errorValue != nil {
		return cloudflareTunnelConfiguration{}, errorValue
	}
	response, errorValue := client.httpClient.Do(request)
	if errorValue != nil {
		return cloudflareTunnelConfiguration{}, errorValue
	}
	defer response.Body.Close()
	var document cloudflareTunnelConfigurationResponse
	if errorValue := decodeCloudflareResponse(response, &document); errorValue != nil {
		return cloudflareTunnelConfiguration{}, errorValue
	}
	return document.Result.Configuration, nil
}

func (client cloudflareTunnelAPIClient) updateConfiguration(ctx context.Context, configuration cloudflareTunnelConfiguration) error {
	body, errorValue := json.Marshal(cloudflareTunnelConfigurationResource{Configuration: configuration})
	if errorValue != nil {
		return errorValue
	}
	request, errorValue := client.newRequest(ctx, http.MethodPut, bytes.NewReader(body))
	if errorValue != nil {
		return errorValue
	}
	response, errorValue := client.httpClient.Do(request)
	if errorValue != nil {
		return errorValue
	}
	defer response.Body.Close()
	var document cloudflareTunnelConfigurationResponse
	return decodeCloudflareResponse(response, &document)
}

func (client cloudflareTunnelAPIClient) newRequest(ctx context.Context, method string, body io.Reader) (*http.Request, error) {
	request, errorValue := http.NewRequestWithContext(ctx, method, client.configurationURL(), body)
	if errorValue != nil {
		return nil, errorValue
	}
	request.Header.Set("Authorization", "Bearer "+strings.TrimSpace(client.apiToken))
	request.Header.Set("Content-Type", "application/json")
	return request, nil
}

func (client cloudflareTunnelAPIClient) configurationURL() string {
	return strings.TrimRight(client.baseURL, "/") + "/accounts/" + url.PathEscape(client.accountID) + "/cfd_tunnel/" + url.PathEscape(client.tunnelID) + "/configurations"
}

func decodeCloudflareResponse(response *http.Response, value *cloudflareTunnelConfigurationResponse) error {
	document, errorValue := io.ReadAll(response.Body)
	if errorValue != nil {
		return errorValue
	}
	if errorValue := json.Unmarshal(document, value); errorValue != nil {
		return errorValue
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 || !value.Success {
		return errors.New("Cloudflare tunnel configuration request failed: " + cloudflareErrorMessage(value.Errors))
	}
	return nil
}

func cloudflareErrorMessage(messages []cloudflareAPIMessage) string {
	values := []string{}
	for _, message := range messages {
		if strings.TrimSpace(message.Message) != "" {
			values = append(values, strings.TrimSpace(message.Message))
		}
	}
	if len(values) == 0 {
		return "unknown error"
	}
	return strings.Join(values, "; ")
}

func mergedCloudflareTenantIngress(existingIngress []cloudflareTunnelIngress, manifests []Manifest, publicHostnameTemplate string) []cloudflareTunnelIngress {
	hostnames := targetCloudflareHostnames(manifests, publicHostnameTemplate)
	preservedIngress := []cloudflareTunnelIngress{}
	fallbackIngress := []cloudflareTunnelIngress{}
	for _, ingress := range existingIngress {
		if _, isTargetHostname := hostnames[ingress.Hostname]; isTargetHostname {
			continue
		}
		if strings.TrimSpace(ingress.Hostname) == "" {
			fallbackIngress = append(fallbackIngress, ingress)
			continue
		}
		preservedIngress = append(preservedIngress, ingress)
	}
	for _, manifest := range manifests {
		preservedIngress = append(preservedIngress, cloudflareTenantIngress(manifest, publicHostnameTemplate)...)
	}
	return append(preservedIngress, fallbackIngress...)
}

func removedCloudflareTenantIngress(existingIngress []cloudflareTunnelIngress, manifests []Manifest, publicHostnameTemplate string) ([]cloudflareTunnelIngress, int) {
	hostnames := targetCloudflareHostnames(manifests, publicHostnameTemplate)
	if len(hostnames) == 0 {
		return append([]cloudflareTunnelIngress{}, existingIngress...), 0
	}
	updatedIngress := []cloudflareTunnelIngress{}
	removedCount := 0
	for _, ingress := range existingIngress {
		if _, isTargetHostname := hostnames[ingress.Hostname]; isTargetHostname {
			removedCount++
			continue
		}
		updatedIngress = append(updatedIngress, ingress)
	}
	return updatedIngress, removedCount
}

func cloudflareTenantIngress(manifest Manifest, publicHostnameTemplate string) []cloudflareTunnelIngress {
	hostname := tenantCloudflareHostname(manifest, publicHostnameTemplate)
	admindURL := hostLocalURL(hostRuntimeConfiguration(manifest, RuntimePaths{}, HostRuntimeOptions{}).AdmindPort)
	mattermostURL := firstNonEmptyTenantString(manifest.MattermostInstance.InternalURL, hostLocalURL(manifest.MattermostInstance.Port))
	return []cloudflareTunnelIngress{
		{Hostname: hostname, Path: "/(admin|flow|memory|calendar|mail|attendance)(/.*)?", Service: admindURL},
		{Hostname: hostname, Path: "/(auth|_app|_internkim)(/.*)?", Service: admindURL},
		{Hostname: hostname, Path: "/(logo\\.svg|\\.well-known/caldav)", Service: admindURL},
		{Hostname: hostname, Service: mattermostURL},
	}
}

func targetCloudflareHostnames(manifests []Manifest, publicHostnameTemplate string) map[string]struct{} {
	hostnames := map[string]struct{}{}
	for _, manifest := range manifests {
		for _, hostname := range []string{tenantPublicHostname(manifest), tenantCloudflareHostname(manifest, publicHostnameTemplate)} {
			if strings.TrimSpace(hostname) != "" {
				hostnames[hostname] = struct{}{}
			}
		}
	}
	return hostnames
}

func manifestTenantIDs(manifests []Manifest) []string {
	tenantIDs := make([]string, 0, len(manifests))
	for _, manifest := range manifests {
		tenantIDs = append(tenantIDs, manifest.TenantID)
	}
	return tenantIDs
}

func tenantPublicHostname(manifest Manifest) string {
	value := firstNonEmptyTenantString(manifest.PublicURL, manifest.MattermostInstance.PublicURL)
	parsedURL, errorValue := url.Parse(value)
	if errorValue == nil && parsedURL.Hostname() != "" {
		return parsedURL.Hostname()
	}
	return strings.TrimPrefix(strings.TrimPrefix(value, "https://"), "http://")
}

func tenantCloudflareHostname(manifest Manifest, publicHostnameTemplate string) string {
	template := strings.TrimSpace(publicHostnameTemplate)
	if template == "" {
		return tenantPublicHostname(manifest)
	}
	return strings.ReplaceAll(template, "{tenant}", manifest.TenantID)
}
