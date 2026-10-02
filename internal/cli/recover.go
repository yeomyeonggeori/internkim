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
	"net/url"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/yeomyeonggeori/internkim/internal/admind"
)

type recoveryRequest struct {
	Action    string `json:"action"`
	Target    string `json:"target,omitempty"`
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

var (
	healthHTTPClient = &http.Client{
		Timeout: 8 * time.Second,
		CheckRedirect: func(request *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	recoveryHTTPClient = &http.Client{
		Timeout: 10 * time.Minute,
		CheckRedirect: func(request *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
)

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
	action := flagSet.String("action", "status", "Recovery action, one of: "+strings.Join(admind.SSHRecoveryActions, ", "))
	host := flagSet.String("host", "", "Board host")
	user := flagSet.String("user", "", "SSH user")
	password := flagSet.String("password", "", "SSH password")
	actionTarget := flagSet.String("target", "", "What the action acts on, for the actions that name one")
	if errorValue := flagSet.Parse(arguments); errorValue != nil {
		return errorValue
	}
	target := resolveCommandTarget(targetFlagArguments(*host, *user, *password))
	return runSSHRecoveryForTarget(target, *action, *actionTarget)
}

func runSSHRecoveryForTarget(target commandTarget, action string, actionTarget string) error {
	action = strings.TrimSpace(action)
	if !isAllowedCLIRecoveryAction(action) {
		return fmt.Errorf("unsupported recovery action: %s", action)
	}
	response, errorValue := performSSHRecoveryRequest(target, action, actionTarget)
	if errorValue != nil {
		return errorValue
	}
	printCommandTargetEvidence(target)
	fmt.Printf("Recovery action: %s\n", response.Action)
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
	return nil
}

func isAllowedCLIRecoveryAction(action string) bool {
	return slices.Contains(admind.SSHRecoveryActions, action)
}

func performSSHRecoveryRequest(target commandTarget, action string, actionTarget string) (recoveryResponse, error) {
	var response recoveryResponse
	deviceURL := strings.TrimSpace(target.deviceURL)
	if deviceURL == "" {
		return response, errors.New("device URL is not configured; run setup on the device network first")
	}
	fleetID, fleetSecret, errorValue := target.fleetIdentity()
	if errorValue != nil {
		return response, errorValue
	}
	endpointURL, errorValue := publicEndpointURL(deviceURL, "/admin/api/recovery/ssh-tunnel/restart")
	if errorValue != nil {
		return response, errorValue
	}
	payload := signedRecoveryRequestPayload(fleetSecret, action, actionTarget, fleetID)
	document, errorValue := json.Marshal(payload)
	if errorValue != nil {
		return response, errorValue
	}
	request, errorValue := http.NewRequest(http.MethodPost, endpointURL, bytes.NewReader(document))
	if errorValue != nil {
		return response, errorValue
	}
	request.Header.Set("Content-Type", "application/json")
	httpResponse, errorValue := recoveryHTTPClient.Do(request)
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

func signedRecoveryRequestPayload(secret string, action string, target string, deviceID string) recoveryRequest {
	timestamp := time.Now().UTC().Format(time.RFC3339)
	nonce := randomRecoveryNonce()
	return recoveryRequest{
		Action:    action,
		Target:    target,
		DeviceID:  deviceID,
		Nonce:     nonce,
		Timestamp: timestamp,
		Signature: signCLIRecoveryPayload(secret, action, target, deviceID, nonce, timestamp),
	}
}

func randomRecoveryNonce() string {
	document := make([]byte, 16)
	if _, errorValue := rand.Read(document); errorValue != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(document)
}

func signCLIRecoveryPayload(secret string, action string, target string, deviceID string, nonce string, timestamp string) string {
	fields := []string{action, deviceID, nonce, timestamp}
	if strings.TrimSpace(target) != "" {
		fields = append(fields, strings.TrimSpace(target))
	}
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(strings.Join(fields, "\n")))
	return hex.EncodeToString(mac.Sum(nil))
}

func publicEndpointURL(deviceURL string, path string) (string, error) {
	parsedURL, errorValue := url.Parse(strings.TrimSpace(deviceURL))
	if errorValue != nil {
		return "", errorValue
	}
	parsedURL.Path = "/" + strings.TrimLeft(path, "/")
	parsedURL.RawQuery = ""
	parsedURL.Fragment = ""
	return parsedURL.String(), nil
}

func fetchPublicEndpoint(deviceURL string, path string) (int, string, error) {
	endpointURL, errorValue := publicEndpointURL(deviceURL, path)
	if errorValue != nil {
		return 0, "", errorValue
	}
	response, errorValue := healthHTTPClient.Get(endpointURL)
	if errorValue != nil {
		return 0, "", errorValue
	}
	defer response.Body.Close()
	responseBody, errorValue := io.ReadAll(io.LimitReader(response.Body, 4096))
	if errorValue != nil {
		return response.StatusCode, "", errorValue
	}
	return response.StatusCode, string(responseBody), nil
}
