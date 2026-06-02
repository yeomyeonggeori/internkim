package admind

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const recoverySignatureTTL = 5 * time.Minute
const recoveryNonceDirectoryName = "recovery-nonces"

type sshRecoveryRequest struct {
	Action    string `json:"action"`
	DeviceID  string `json:"deviceID"`
	Nonce     string `json:"nonce"`
	Timestamp string `json:"timestamp"`
	Signature string `json:"signature"`
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
	payload.Action = strings.TrimSpace(payload.Action)
	payload.DeviceID = strings.TrimSpace(payload.DeviceID)
	payload.Nonce = strings.TrimSpace(payload.Nonce)
	payload.Timestamp = strings.TrimSpace(payload.Timestamp)
	payload.Signature = strings.TrimSpace(payload.Signature)
	if !isAllowedSSHRecoveryAction(payload.Action) {
		return fmt.Errorf("unsupported recovery action")
	}
	if payload.DeviceID == "" || payload.Nonce == "" || payload.Timestamp == "" || payload.Signature == "" {
		return fmt.Errorf("missing recovery signature fields")
	}
	if payload.DeviceID != strings.TrimSpace(readTrimmedFile(service.Configuration.FleetIDPath)) {
		return fmt.Errorf("recovery device mismatch")
	}
	createdAt, errorValue := time.Parse(time.RFC3339, payload.Timestamp)
	if errorValue != nil {
		return fmt.Errorf("invalid recovery timestamp")
	}
	if time.Since(createdAt) > recoverySignatureTTL || time.Until(createdAt) > recoverySignatureTTL {
		return fmt.Errorf("stale recovery request")
	}
	if errorValue := service.claimSSHRecoveryNonce(payload.Nonce); errorValue != nil {
		return errorValue
	}
	secret := strings.TrimSpace(readTrimmedFile(service.Configuration.FleetSecretPath))
	if secret == "" {
		return fmt.Errorf("recovery secret is not configured")
	}
	expectedSignature := signSSHRecoveryPayload(secret, payload.Action, payload.DeviceID, payload.Nonce, payload.Timestamp)
	if !hmac.Equal([]byte(expectedSignature), []byte(payload.Signature)) {
		return fmt.Errorf("invalid recovery signature")
	}
	return nil
}

func (service *Service) claimSSHRecoveryNonce(nonce string) error {
	if strings.Contains(nonce, "/") || strings.Contains(nonce, "\\") || strings.Contains(nonce, "..") {
		return fmt.Errorf("invalid recovery nonce")
	}
	directoryPath := filepath.Join(service.Configuration.StateDirectory, recoveryNonceDirectoryName)
	if errorValue := os.MkdirAll(directoryPath, 0o700); errorValue != nil {
		return fmt.Errorf("failed to prepare recovery nonce store")
	}
	noncePath := filepath.Join(directoryPath, nonce)
	file, errorValue := os.OpenFile(noncePath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if errorValue != nil {
		if os.IsExist(errorValue) {
			return fmt.Errorf("replayed recovery request")
		}
		return fmt.Errorf("failed to record recovery nonce")
	}
	_ = file.Close()
	return nil
}

func isAllowedSSHRecoveryAction(action string) bool {
	switch action {
	case "status", "restart-ssh", "restart-cloudflared-node-ssh", "journal-tail":
		return true
	default:
		return false
	}
}

func signSSHRecoveryPayload(secret string, action string, deviceID string, nonce string, timestamp string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(strings.Join([]string{action, deviceID, nonce, timestamp}, "\n")))
	return hex.EncodeToString(mac.Sum(nil))
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
