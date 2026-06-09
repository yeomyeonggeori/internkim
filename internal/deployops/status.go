package deployops

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

func (server *Server) CheckStatus(contextValue context.Context, target Target) TargetStatus {
	return TargetStatus{
		TargetID:   target.ID,
		CheckedAt:  time.Now(),
		Admin:      server.checkJSONEndpoint(contextValue, target.AdminURL+"/admin/api/health"),
		Mattermost: server.checkJSONEndpoint(contextValue, target.AdminURL+"/api/v4/system/ping"),
		Release:    server.checkJSONEndpoint(contextValue, target.AdminURL+"/admin/api/updates/status"),
		Recovery:   server.checkRecovery(contextValue, target),
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
	status.Message = valueString(document, "status")
	status.StartedAt = valueString(document, "startedAt")
	status.Release = firstValueString(document, "currentRelease", "releaseID", "version", "build")
	if status.Message == "" {
		status.Message = "ok"
	}
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
		"admin=%s mattermost=%s release=%s recovery=%s",
		status.Admin.State,
		status.Mattermost.State,
		status.Release.State,
		status.Recovery.State,
	)
}
