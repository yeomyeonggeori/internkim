package deployops

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"
)

func (server *Server) CheckStatus(contextValue context.Context, target Target) TargetStatus {
	var admin EndpointStatus
	var release releaseEndpointStatus
	var recovery RecoveryStatus
	var llm LLMModelStatus
	waitGroup := sync.WaitGroup{}
	waitGroup.Add(4)
	go func() {
		defer waitGroup.Done()
		admin = server.checkJSONEndpoint(contextValue, target.AdminURL+"/admin/api/health")
	}()
	go func() {
		defer waitGroup.Done()
		release = server.checkReleaseEndpoint(contextValue, target.AdminURL+"/admin/api/updates/status")
	}()
	go func() {
		defer waitGroup.Done()
		recovery = server.checkRecovery(contextValue, target)
	}()
	go func() {
		defer waitGroup.Done()
		llm = server.ReadLLMModel(contextValue, target)
	}()
	waitGroup.Wait()
	return TargetStatus{
		TargetID:  target.ID,
		CheckedAt: time.Now(),
		Admin:     admin,
		Release:   release.Endpoint,
		Recovery:  recovery,
		LLM:       llm,
		Versions:  buildVersionStatus(admin, release),
	}
}

func (server *Server) checkJSONEndpoint(contextValue context.Context, endpointURL string) EndpointStatus {
	request, errorValue := http.NewRequestWithContext(contextValue, http.MethodGet, endpointURL, nil)
	if errorValue != nil {
		return EndpointStatus{State: "failed", Message: errorValue.Error()}
	}
	response, errorValue := server.client.Do(request)
	if errorValue != nil {
		return EndpointStatus{State: "failed", Message: errorValue.Error()}
	}
	defer response.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(response.Body, 256*1024))
	status := EndpointStatus{Code: response.StatusCode}
	if response.StatusCode >= 200 && response.StatusCode < 300 {
		status.State = "ok"
		enrichEndpointStatus(&status, body)
		return status
	}
	status.State = "failed"
	if response.StatusCode == http.StatusFound || response.StatusCode == http.StatusTemporaryRedirect || response.StatusCode == http.StatusForbidden {
		status.State = "auth_required"
	}
	status.Message = strings.TrimSpace(string(body))
	return status
}

func (server *Server) checkReleaseEndpoint(contextValue context.Context, endpointURL string) releaseEndpointStatus {
	request, errorValue := http.NewRequestWithContext(contextValue, http.MethodGet, endpointURL, nil)
	if errorValue != nil {
		return releaseEndpointStatus{Endpoint: EndpointStatus{State: "failed", Message: errorValue.Error()}}
	}
	attachCloudflareAccessCookie(request)
	response, errorValue := server.client.Do(request)
	if errorValue != nil {
		return releaseEndpointStatus{Endpoint: EndpointStatus{State: "failed", Message: errorValue.Error()}}
	}
	defer response.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(response.Body, 512*1024))
	status := EndpointStatus{Code: response.StatusCode}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		status.State = "failed"
		if response.StatusCode == http.StatusFound || response.StatusCode == http.StatusTemporaryRedirect || response.StatusCode == http.StatusForbidden {
			status.State = "auth_required"
		}
		status.Message = strings.TrimSpace(string(body))
		return releaseEndpointStatus{Endpoint: status}
	}
	releaseStatus, errorValue := decodeReleaseStatus(body)
	if errorValue != nil {
		status.State = "not_json"
		status.Message = "release status did not return JSON"
		return releaseEndpointStatus{Endpoint: status}
	}
	status.State = "ok"
	status.Message = releaseStatus.State
	status.Release = releaseStatus.Current.ReleaseID
	status.CurrentRelease = releaseStatus.Current.ReleaseID
	status.LatestRelease = releaseStatus.Latest.ReleaseID
	status.UpdateAllowed = releaseStatus.UpdateAllowed
	return releaseEndpointStatus{Endpoint: status, Release: releaseStatus}
}

func (server *Server) checkRecovery(contextValue context.Context, target Target) RecoveryStatus {
	plan := recoveryCommand(server.options.RepositoryRootPath, server.options.ExecutablePath, target, "status")
	commandOutput, errorValue := runBufferedCommand(contextValue, plan)
	if errorValue != nil {
		return RecoveryStatus{State: "failed", Message: Redact(strings.TrimSpace(commandOutput + "\n" + errorValue.Error()))}
	}
	return RecoveryStatus{State: "ok", Message: Redact(strings.TrimSpace(commandOutput)), Services: parseRecoveryServices(commandOutput)}
}

func enrichEndpointStatus(status *EndpointStatus, body []byte) {
	var document map[string]any
	if errorValue := json.NewDecoder(bytes.NewReader(body)).Decode(&document); errorValue != nil {
		status.Message = strings.TrimSpace(string(body))
		return
	}
	enrichReleaseSummaryStatus(status, document)
	status.Message = valueString(document, "status")
	status.StartedAt = valueString(document, "startedAt")
	status.Release = firstNonEmpty(firstValueString(document, "admindBuildID", "currentRelease", "releaseID", "version", "build"), status.Release)
	if status.Message == "" {
		status.Message = "ok"
	}
}

func enrichReleaseSummaryStatus(status *EndpointStatus, document map[string]any) {
	currentRelease := nestedValueString(document, "current", "releaseID")
	latestRelease := nestedValueString(document, "latest", "releaseID")
	if currentRelease == "" && latestRelease == "" {
		return
	}
	status.State = firstNonEmpty(valueString(document, "state"), status.State)
	status.CurrentRelease = currentRelease
	status.LatestRelease = latestRelease
	status.Release = currentRelease
	status.UpdateAllowed = valueBool(document, "updateAllowed")
}

type releaseEndpointStatus struct {
	Endpoint EndpointStatus
	Release  releaseStatusDocument
}

type releaseStatusDocument struct {
	Current       ReleaseVersion `json:"current"`
	Latest        ReleaseVersion `json:"latest"`
	State         string         `json:"state"`
	UpdateAllowed bool           `json:"updateAllowed"`
}

func decodeReleaseStatus(body []byte) (releaseStatusDocument, error) {
	var rawStatus struct {
		Current       *releaseSummaryDocument `json:"current"`
		Latest        *releaseSummaryDocument `json:"latest"`
		State         string                  `json:"state"`
		UpdateAllowed bool                    `json:"updateAllowed"`
	}
	if errorValue := json.NewDecoder(bytes.NewReader(body)).Decode(&rawStatus); errorValue != nil {
		return releaseStatusDocument{}, errorValue
	}
	return releaseStatusDocument{
		Current:       releaseVersionFromSummary(rawStatus.Current),
		Latest:        releaseVersionFromSummary(rawStatus.Latest),
		State:         rawStatus.State,
		UpdateAllowed: rawStatus.UpdateAllowed,
	}, nil
}

type releaseSummaryDocument struct {
	ReleaseID  string                    `json:"releaseID"`
	Components map[string]ComponentBrief `json:"components"`
}

func releaseVersionFromSummary(summary *releaseSummaryDocument) ReleaseVersion {
	if summary == nil {
		return ReleaseVersion{}
	}
	components := map[string]ComponentBrief{}
	for componentName, component := range summary.Components {
		components[componentName] = component
	}
	version := ReleaseVersion{
		ReleaseID:  strings.TrimSpace(summary.ReleaseID),
		Components: components,
	}
	version.Admind = componentRevision(components, "admind")
	version.Capabilityd = componentRevision(components, "capabilityd")
	version.BlueclawPayload = componentRevision(components, "blueclawPayload")
	version.Skills = componentRevision(components, "skills")
	version.Web = componentRevision(components, "web")
	version.Runtime = runtimeRevisionLabel(version)
	return version
}

func componentRevision(components map[string]ComponentBrief, componentName string) string {
	return strings.TrimSpace(components[componentName].Revision)
}

func runtimeRevisionLabel(version ReleaseVersion) string {
	parts := []string{}
	for _, revision := range []string{version.Capabilityd, version.BlueclawPayload, version.Skills} {
		if revision != "" {
			parts = append(parts, shortRevision(revision))
		}
	}
	return strings.Join(parts, " / ")
}

func shortRevision(revision string) string {
	revision = strings.TrimSpace(revision)
	if len(revision) <= 12 {
		return revision
	}
	return revision[:12]
}

func buildVersionStatus(admin EndpointStatus, release releaseEndpointStatus) VersionStatus {
	current := release.Release.Current
	return VersionStatus{
		Admind:  firstNonEmpty(current.Admind, admin.Release),
		Runtime: current.Runtime,
		Web:     current.Web,
		Current: current,
		Latest:  release.Release.Latest,
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			return value
		}
	}
	return ""
}

func valueString(document map[string]any, key string) string {
	value, ok := document[key]
	if !ok {
		return ""
	}
	switch typedValue := value.(type) {
	case string:
		return typedValue
	case float64:
		return fmt.Sprintf("%.0f", typedValue)
	default:
		return ""
	}
}

func firstValueString(document map[string]any, keys ...string) string {
	for _, key := range keys {
		value := valueString(document, key)
		if value != "" {
			return value
		}
	}
	return ""
}

func nestedValueString(document map[string]any, objectKey string, valueKey string) string {
	value, ok := document[objectKey]
	if !ok {
		return ""
	}
	object, ok := value.(map[string]any)
	if !ok {
		return ""
	}
	return valueString(object, valueKey)
}

func valueBool(document map[string]any, key string) bool {
	value, ok := document[key]
	if !ok {
		return false
	}
	typedValue, ok := value.(bool)
	return ok && typedValue
}

func parseRecoveryServices(output string) map[string]string {
	services := map[string]string{}
	for _, line := range strings.Split(output, "\n") {
		trimmedLine := strings.TrimSpace(line)
		for _, serviceName := range []string{"ssh", "cloudflared-node-ssh", "cloudflared"} {
			if strings.HasPrefix(trimmedLine, serviceName) {
				services[serviceName] = strings.TrimSpace(strings.TrimPrefix(trimmedLine, serviceName))
			}
		}
	}
	return services
}

func formatStatus(status TargetStatus) string {
	return fmt.Sprintf(
		"admin=%s release=%s recovery=%s",
		status.Admin.State,
		status.Release.State,
		status.Recovery.State,
	)
}

var cloudflareAccessTokenMutex sync.Mutex
var cloudflareAccessTokenByHost = map[string]string{}

const cloudflareAccessServiceTokenPath = ".local/secrets/cloudflare-access-service-token.json"

// A deploy that runs unattended cannot answer a browser login. The service
// token, provisioned by tools/provision-cloudflare-ssh-service-token into one
// 0600 file, authenticates without one and lasts a year; the day-lived
// cloudflared login token serves only an operator who holds no service token.
func attachCloudflareAccessCookie(request *http.Request) {
	if clientID, clientSecret := cloudflareAccessServiceToken(cloudflareAccessServiceTokenPath); clientID != "" && clientSecret != "" {
		request.Header.Set("CF-Access-Client-Id", clientID)
		request.Header.Set("CF-Access-Client-Secret", clientSecret)
		return
	}
	token := cloudflareAccessToken(request.URL.String())
	if token == "" {
		return
	}
	request.AddCookie(&http.Cookie{Name: "CF_Authorization", Value: token})
}

func cloudflareAccessServiceToken(path string) (string, string) {
	document, errorValue := os.ReadFile(path)
	if errorValue != nil {
		return "", ""
	}
	var held struct {
		ClientID     string `json:"clientID"`
		ClientSecret string `json:"clientSecret"`
	}
	if json.Unmarshal(document, &held) != nil {
		return "", ""
	}
	return strings.TrimSpace(held.ClientID), strings.TrimSpace(held.ClientSecret)
}

func cloudflareAccessToken(applicationURL string) string {
	parsedURL, errorValue := url.Parse(applicationURL)
	if errorValue != nil {
		return ""
	}
	host := parsedURL.Host
	if token := cachedCloudflareAccessToken(host); token != "" {
		return token
	}
	contextValue, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	command := exec.CommandContext(contextValue, "cloudflared", "access", "token", "--app="+applicationURL)
	output, errorValue := command.Output()
	if errorValue != nil {
		return ""
	}
	token := strings.TrimSpace(string(output))
	if token != "" {
		storeCloudflareAccessToken(host, token)
	}
	return token
}

func cachedCloudflareAccessToken(host string) string {
	cloudflareAccessTokenMutex.Lock()
	defer cloudflareAccessTokenMutex.Unlock()
	return strings.TrimSpace(cloudflareAccessTokenByHost[host])
}

func storeCloudflareAccessToken(host string, token string) {
	cloudflareAccessTokenMutex.Lock()
	defer cloudflareAccessTokenMutex.Unlock()
	cloudflareAccessTokenByHost[host] = token
}
