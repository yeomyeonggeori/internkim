package cli

import (
	"bytes"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type recoveryRequest struct {
	Action    string `json:"action"`
	DeviceID  string `json:"deviceID"`
	Nonce     string `json:"nonce"`
	Timestamp string `json:"timestamp"`
	Signature string `json:"signature"`
}

type recoveryResponse struct {
	Status      string            `json:"status"`
	Action      string            `json:"action"`
	Services    map[string]string `json:"services"`
	Results     []recoveryResult  `json:"results"`
	JournalTail string            `json:"journalTail"`
	Snapshot    string            `json:"snapshot"`
	NextStep    string            `json:"nextStep"`
	ObservedAt  time.Time         `json:"observedAt"`
}

type recoveryResult struct {
	Name   string `json:"name"`
	Status string `json:"status"`
	Output string `json:"output"`
}

const recoveryResponseBodyLimitBytes = 256 * 1024

func runRecover() {
	if errorValue := runRecoverArguments(os.Args[2:]); errorValue != nil {
		fatal(errorValue.Error())
	}
}

func runRecoverArguments(arguments []string) error {
	subcommand := "ssh"
	if len(arguments) > 0 && !strings.HasPrefix(arguments[0], "-") {
		subcommand = arguments[0]
		arguments = arguments[1:]
	}
	switch subcommand {
	case "ssh":
		return runRecoverSSH(arguments)
	default:
		return fmt.Errorf("unknown recover subcommand: %s", subcommand)
	}
}

func runRecoverSSH(arguments []string) error {
	flagSet := flag.NewFlagSet("recover ssh", flag.ContinueOnError)
	action := flagSet.String("action", "restart-cloudflared-node-ssh", "Recovery action: status, snapshot, restart-cloudflared-node-ssh, restart-ssh, journal-tail, unlock-mattermost-admin, limit-blueclaw, restart-blueclaw, blueclaw-boot-diagnose, blueclaw-journal")
	host := flagSet.String("host", "", "Board host")
	user := flagSet.String("user", "", "SSH user")
	password := flagSet.String("password", "", "SSH password")
	node := flagSet.String("node", "", "Fleet node target")
	board := flagSet.String("board", "", "Board target")
	diagnose := flagSet.Bool("diagnose", false, "Also check the local admind route when SSH is available")
	if errorValue := flagSet.Parse(arguments); errorValue != nil {
		return errorValue
	}
	repositoryRootPath, errorValue := resolveRepositoryRootPath()
	if errorValue != nil {
		return errorValue
	}
	target := resolveCommandTarget(verifyTargetArguments(*host, *user, *password, *node, true, *board, false))
	target = resolveLabHostForCommandTarget(target, repositoryRootPath)
	return runSSHRecoveryForTarget(newMsg("ko"), loadConfig(), filepath.Join(repositoryRootPath, "bin", "sshpass"), target, *action, *diagnose)
}

func runSSHRecoveryForTarget(m *msg, configuration config, sshpassBin string, target commandTarget, action string, diagnose bool) error {
	action = strings.TrimSpace(action)
	if !isAllowedCLIRecoveryAction(action) {
		return fmt.Errorf("unsupported recovery action: %s", action)
	}
	response, errorValue := performSSHRecoveryRequest(target, action)
	if errorValue != nil {
		return errorValue
	}
	printCommandTargetEvidence(target)
	fmt.Printf("Recovery action: %s\n", response.Action)
	for _, serviceName := range []string{"ssh", "cloudflared-node-ssh", "cloudflared", "blueclaw", "buzz-relay", "buzz-relay-stunnel", "chatd", "relay-tls-443"} {
		if serviceState := strings.TrimSpace(response.Services[serviceName]); serviceState != "" {
			fmt.Printf("  %-22s %s\n", serviceName, serviceState)
		}
	}
	for _, result := range response.Results {
		fmt.Printf("  %-22s %s\n", result.Name, result.Status)
		if strings.TrimSpace(result.Output) != "" {
			fmt.Printf("%s\n", strings.TrimSpace(result.Output))
		}
	}
	if strings.TrimSpace(response.JournalTail) != "" {
		fmt.Printf("\n--- recovery journal tail ---\n%s\n-----------------------------\n", response.JournalTail)
	}
	if strings.TrimSpace(response.Snapshot) != "" {
		fmt.Printf("\n--- recovery snapshot ---\n%s\n-------------------------\n", response.Snapshot)
	}
	if diagnose {
		printSSHRecoveryLocalDiagnostics(configuration, sshpassBin, target)
	}
	if action == "status" || action == "snapshot" || action == "journal-tail" || action == "limit-blueclaw" || action == "restart-blueclaw" || action == "blueclaw-boot-diagnose" || action == "blueclaw-journal" || action == "buzz-mirror-status" || action == "buzz-orphan-inspect" || action == "buzz-snapshot" || action == "buzz-membership-recover" || action == "buzz-restore" || action == "buzz-repair-dryrun" || action == "buzz-repair-apply" || action == "buzz-reimport" || action == "buzz-reimport-log" {
		return nil
	}
	if action == "reboot" {
		fmt.Println(m.t("재부팅이 예약되었습니다. 약 2분 후 `internkim status`로 확인하세요.", "Reboot scheduled. Check `internkim status` in about two minutes."))
		return nil
	}
	if connection, _, retryError := resolveCloudflareSSHConnection(configuration, sshpassBin, target, false); retryError == nil && connection != nil {
		fmt.Println(m.t("SSH 복구 확인 완료", "SSH recovery verified"))
		return nil
	}
	if action == "restart-cloudflared-node-ssh" {
		return errors.New("cloudflared-node-ssh restart was requested, but SSH still did not recover; try `./internkim recover ssh --action restart-ssh`")
	}
	return errors.New("SSH recovery action completed, but SSH still did not recover")
}

func printSSHRecoveryLocalDiagnostics(configuration config, sshpassBin string, target commandTarget) {
	connection, _, errorValue := resolveCloudflareSSHConnection(configuration, sshpassBin, target, false)
	if errorValue != nil || connection == nil {
		fmt.Printf("  %-22s %s\n", "local admind", "SSH unavailable")
		return
	}
	output, runError := connection.runResult("curl -fsS http://127.0.0.1:18080/admin/api/health")
	if runError != nil {
		fmt.Printf("  %-22s %s\n", "local admind", strings.TrimSpace(output))
		return
	}
	fmt.Printf("  %-22s %s\n", "local admind", strings.TrimSpace(output))
}

func isAllowedCLIRecoveryAction(action string) bool {
	switch action {
	case "status", "snapshot", "restart-cloudflared-node-ssh", "restart-ssh", "journal-tail", "unlock-mattermost-admin", "reboot", "stop-tenant-pilots", "remove-tenant-pilots", "limit-blueclaw", "restart-blueclaw", "blueclaw-boot-diagnose", "blueclaw-journal", "blueclaw-workspace-repair", "blueclaw-postgres-salvage", "repair-buzz-relay", "buzz-relay-journal", "enable-buzz-mirror", "buzz-mirror-status", "buzz-orphan-inspect", "buzz-snapshot", "buzz-membership-recover", "buzz-restore", "buzz-repair-dryrun", "buzz-repair-apply", "buzz-reimport", "buzz-reimport-log":
		return true
	default:
		return false
	}
}

func performSSHRecoveryRequest(target commandTarget, action string) (recoveryResponse, error) {
	var response recoveryResponse
	deviceURL := strings.TrimSpace(target.deviceURL)
	if deviceURL == "" {
		return response, errors.New("device URL is not configured; run setup on the device network first")
	}
	fleetID := strings.TrimSpace(loadState(target.stateDir, "fleet_id"))
	fleetSecret := strings.TrimSpace(loadState(target.stateDir, "fleet_secret"))
	if fleetID == "" || fleetSecret == "" {
		return response, errors.New("fleet identity is not configured in local device state")
	}
	endpointURL, errorValue := publicEndpointURL(deviceURL, "/admin/api/recovery/ssh-tunnel/restart")
	if errorValue != nil {
		return response, errorValue
	}
	payload := signedRecoveryRequestPayload(fleetSecret, action, fleetID)
	document, errorValue := json.Marshal(payload)
	if errorValue != nil {
		return response, errorValue
	}
	request, errorValue := http.NewRequest(http.MethodPost, endpointURL, bytes.NewReader(document))
	if errorValue != nil {
		return response, errorValue
	}
	request.Header.Set("Content-Type", "application/json")
	httpResponse, errorValue := statusHTTPClient.Do(request)
	if errorValue != nil {
		return response, errorValue
	}
	defer httpResponse.Body.Close()
	responseBody, _ := io.ReadAll(io.LimitReader(httpResponse.Body, recoveryResponseBodyLimitBytes))
	if httpResponse.StatusCode < 200 || httpResponse.StatusCode >= 300 {
		return response, recoveryHTTPStatusError(target, httpResponse, string(responseBody))
	}
	if errorValue := json.NewDecoder(bytes.NewReader(responseBody)).Decode(&response); errorValue != nil {
		return response, errorValue
	}
	return response, nil
}

func recoveryHTTPStatusError(target commandTarget, response *http.Response, body string) error {
	location := strings.TrimSpace(response.Header.Get("Location"))
	healthStatus, healthBody, healthError := fetchPublicEndpoint(target.deviceURL, "/admin/api/health")
	healthSummary := fmt.Sprintf("health=HTTP %d %s", healthStatus, strings.TrimSpace(healthBody))
	if healthError != nil {
		healthSummary = "health=" + healthError.Error()
	}
	switch response.StatusCode {
	case http.StatusFound, http.StatusTemporaryRedirect, http.StatusPermanentRedirect:
		return fmt.Errorf("recovery endpoint redirected: HTTP %d location=%q %s; public route is not reaching the recovery handler", response.StatusCode, location, healthSummary)
	case http.StatusForbidden:
		return fmt.Errorf("recovery endpoint rejected the signed request: HTTP 403 %s; check fleet id, fleet secret, timestamp, and nonce", healthSummary)
	case http.StatusNotFound:
		return fmt.Errorf("recovery endpoint not found: HTTP 404 %s; admind route is missing or reverse proxy path is wrong", healthSummary)
	default:
		return fmt.Errorf("recovery endpoint returned HTTP %d body=%q %s", response.StatusCode, strings.TrimSpace(body), healthSummary)
	}
}

func signedRecoveryRequestPayload(secret string, action string, deviceID string) recoveryRequest {
	timestamp := time.Now().UTC().Format(time.RFC3339)
	nonce := randomRecoveryNonce()
	return recoveryRequest{
		Action:    action,
		DeviceID:  deviceID,
		Nonce:     nonce,
		Timestamp: timestamp,
		Signature: signCLIRecoveryPayload(secret, action, deviceID, nonce, timestamp),
	}
}

func randomRecoveryNonce() string {
	document := make([]byte, 16)
	if _, errorValue := rand.Read(document); errorValue != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(document)
}

func signCLIRecoveryPayload(secret string, action string, deviceID string, nonce string, timestamp string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(strings.Join([]string{action, deviceID, nonce, timestamp}, "\n")))
	return hex.EncodeToString(mac.Sum(nil))
}
