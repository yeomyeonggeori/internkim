package admind

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"
)

type sshRecoveryRequest struct {
	fleetSignedRequest
}

type sshRecoveryResponse struct {
	Status      string                     `json:"status"`
	Action      string                     `json:"action"`
	Services    map[string]string          `json:"services"`
	Results     []sshRecoveryCommandResult `json:"results,omitempty"`
	JournalTail string                     `json:"journalTail,omitempty"`
	NextStep    string                     `json:"nextStep"`
	Rejected    string                     `json:"rejected,omitempty"`
	ObservedAt  time.Time                  `json:"observedAt"`
}

type sshRecoveryCommandResult struct {
	Name   string `json:"name"`
	Status string `json:"status"`
	Output string `json:"output,omitempty"`
}

func (service *Service) handleSSHRecovery(responseWriter http.ResponseWriter, request *http.Request, path string) {
	if request.Method != http.MethodPost || path != "/recovery/ssh-tunnel/restart" {
		http.NotFound(responseWriter, request)
		return
	}
	var payload sshRecoveryRequest
	if errorValue := json.NewDecoder(request.Body).Decode(&payload); errorValue != nil {
		http.Error(responseWriter, "invalid recovery request body", http.StatusBadRequest)
		return
	}
	if errorValue := service.validateSSHRecoveryRequest(payload); errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusForbidden)
		return
	}
	response := service.runSSHRecovery(request.Context(), payload.Action)
	service.writeJSON(responseWriter, response)
}

func (service *Service) validateSSHRecoveryRequest(payload sshRecoveryRequest) error {
	return service.validateFleetSignedRequest(payload.fleetSignedRequest, isAllowedSSHRecoveryAction)
}

func isAllowedSSHRecoveryAction(action string) bool {
	switch action {
	case "status", "restart-ssh", "restart-cloudflared-node-ssh", "journal-tail":
		return true
	default:
		return false
	}
}

func (service *Service) runSSHRecovery(ctx context.Context, action string) sshRecoveryResponse {
	response := sshRecoveryResponse{
		Status:     "ok",
		Action:     action,
		Services:   service.sshRecoveryServiceStates(ctx),
		ObservedAt: time.Now().UTC(),
		NextStep:   "Run `internkim verify api --cloudflare-ssh` again.",
	}
	switch action {
	case "status":
		return response
	case "restart-ssh":
		response.Results = append(response.Results, service.runSSHRecoveryCommand(ctx, "restart ssh", "systemctl", "restart", "ssh"))
	case "restart-cloudflared-node-ssh":
		response.Results = append(response.Results, service.runSSHRecoveryCommand(ctx, "restart cloudflared-node-ssh", "systemctl", "restart", "cloudflared-node-ssh"))
	case "journal-tail":
		response.JournalTail = service.sshRecoveryJournalTail(ctx)
	}
	response.Services = service.sshRecoveryServiceStates(ctx)
	response.JournalTail = service.sshRecoveryJournalTail(ctx)
	return response
}

func (service *Service) sshRecoveryServiceStates(ctx context.Context) map[string]string {
	return map[string]string{
		"ssh":                  service.sshRecoveryCommandOutput(ctx, "systemctl", "is-active", "ssh"),
		"cloudflared-node-ssh": service.sshRecoveryCommandOutput(ctx, "systemctl", "is-active", "cloudflared-node-ssh"),
		"cloudflared":          service.sshRecoveryCommandOutput(ctx, "systemctl", "is-active", "cloudflared"),
	}
}

func (service *Service) runSSHRecoveryCommand(ctx context.Context, label string, name string, arguments ...string) sshRecoveryCommandResult {
	output, errorValue := service.runCommand(ctx, name, arguments...)
	status := "ok"
	if errorValue != nil {
		status = "error"
	}
	return sshRecoveryCommandResult{
		Name:   label,
		Status: status,
		Output: redactRecoveryOutput(string(output)),
	}
}

func (service *Service) sshRecoveryCommandOutput(ctx context.Context, name string, arguments ...string) string {
	output, errorValue := service.runCommand(ctx, name, arguments...)
	if errorValue != nil {
		return strings.TrimSpace(redactRecoveryOutput(string(output)))
	}
	return strings.TrimSpace(redactRecoveryOutput(string(output)))
}

func (service *Service) sshRecoveryJournalTail(ctx context.Context) string {
	output, _ := service.runCommand(ctx, "journalctl", "-u", "ssh", "-u", "cloudflared-node-ssh", "-n", "80", "--no-pager")
	return redactRecoveryOutput(string(output))
}

func redactRecoveryOutput(value string) string {
	lines := strings.Split(value, "\n")
	for index, line := range lines {
		lines[index] = redactRecoveryLine(line)
	}
	return strings.TrimSpace(strings.Join(lines, "\n"))
}

func redactRecoveryLine(value string) string {
	fields := strings.Fields(value)
	for index, field := range fields {
		normalizedField := strings.ToLower(field)
		if strings.Contains(normalizedField, "authorization:") ||
			strings.Contains(normalizedField, "bearer") ||
			strings.Contains(normalizedField, "token") ||
			strings.Contains(normalizedField, "secret") {
			fields[index] = "[redacted]"
		}
	}
	return strings.Join(fields, " ")
}
