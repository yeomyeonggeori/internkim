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
	"sort"
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
}

type recoveryResult struct {
	Name   string `json:"name"`
	Status string `json:"status"`
	Output string `json:"output"`
}

type recoveryDevice struct {
	URL         string
	FleetID     string
	FleetSecret string
}

const recoveryResponseBodyLimitBytes = 256 * 1024

var recoveryHTTPClient = &http.Client{
	Timeout: 10 * time.Minute,
	CheckRedirect: func(request *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	},
}

func runRecover() {
	if errorValue := runRecoverArguments(os.Args[2:]); errorValue != nil {
		fatal(errorValue.Error())
	}
}

func runRecoverArguments(arguments []string) error {
	flagSet := flag.NewFlagSet("recover", flag.ContinueOnError)
	action := flagSet.String("action", "status", "Recovery action, one of: "+strings.Join(admind.SSHRecoveryActions, ", "))
	actionTarget := flagSet.String("target", "", "What the action acts on, for the actions that name one")
	if errorValue := flagSet.Parse(arguments); errorValue != nil {
		return errorValue
	}
	if !slices.Contains(admind.SSHRecoveryActions, *action) {
		return fmt.Errorf("unsupported recovery action: %s", *action)
	}
	device, errorValue := recoveryDeviceFromEnvironment()
	if errorValue != nil {
		return errorValue
	}
	response, errorValue := performRecoveryRequest(device, *action, *actionTarget)
	if errorValue != nil {
		return errorValue
	}
	printRecoveryResponse(response)
	return nil
}

func recoveryDeviceFromEnvironment() (recoveryDevice, error) {
	device := recoveryDevice{
		URL:         strings.TrimRight(strings.TrimSpace(os.Getenv("INTERNKIM_DEVICE_URL")), "/"),
		FleetID:     strings.ToLower(strings.TrimSpace(os.Getenv("INTERNKIM_FLEET_ID"))),
		FleetSecret: strings.TrimSpace(os.Getenv("INTERNKIM_FLEET_SECRET")),
	}
	if device.URL == "" || device.FleetID == "" || device.FleetSecret == "" {
		return device, errors.New("recover needs INTERNKIM_DEVICE_URL, INTERNKIM_FLEET_ID and INTERNKIM_FLEET_SECRET; run it as ./internkim @production recover")
	}
	if !strings.HasPrefix(device.URL, "http://") && !strings.HasPrefix(device.URL, "https://") {
		device.URL = "https://" + device.URL
	}
	return device, nil
}

func printRecoveryResponse(response recoveryResponse) {
	fmt.Printf("Recovery action: %s (%s)\n", response.Action, response.Status)
	serviceNames := make([]string, 0, len(response.Services))
	for name := range response.Services {
		serviceNames = append(serviceNames, name)
	}
	sort.Strings(serviceNames)
	for _, name := range serviceNames {
		fmt.Printf("  %-22s %s\n", name, strings.TrimSpace(response.Services[name]))
	}
	for _, result := range response.Results {
		fmt.Printf("  %-22s %s\n", result.Name, result.Status)
		if output := strings.TrimSpace(result.Output); output != "" {
			fmt.Println(output)
		}
	}
	if journal := strings.TrimSpace(response.JournalTail); journal != "" {
		fmt.Printf("\n--- recovery journal tail ---\n%s\n", journal)
	}
	if snapshot := strings.TrimSpace(response.Snapshot); snapshot != "" {
		fmt.Printf("\n--- recovery snapshot ---\n%s\n", snapshot)
	}
	if nextStep := strings.TrimSpace(response.NextStep); nextStep != "" {
		fmt.Printf("\nNext: %s\n", nextStep)
	}
}

func performRecoveryRequest(device recoveryDevice, action string, actionTarget string) (recoveryResponse, error) {
	var response recoveryResponse
	endpointURL, errorValue := url.JoinPath(device.URL, "/admin/api/recovery/ssh-tunnel/restart")
	if errorValue != nil {
		return response, errorValue
	}
	document, errorValue := json.Marshal(signedRecoveryRequestPayload(device.FleetSecret, action, actionTarget, device.FleetID))
	if errorValue != nil {
		return response, errorValue
	}
	httpResponse, errorValue := recoveryHTTPClient.Post(endpointURL, "application/json", bytes.NewReader(document))
	if errorValue != nil {
		return response, errorValue
	}
	defer httpResponse.Body.Close()
	responseBody, _ := io.ReadAll(io.LimitReader(httpResponse.Body, recoveryResponseBodyLimitBytes))
	if httpResponse.StatusCode < 200 || httpResponse.StatusCode >= 300 {
		return response, fmt.Errorf("recovery endpoint %s answered HTTP %d: %s", endpointURL, httpResponse.StatusCode, strings.TrimSpace(string(responseBody)))
	}
	return response, json.Unmarshal(responseBody, &response)
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
