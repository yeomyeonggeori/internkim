package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/anthropic-lab/internkim/internal/capabilities"
	companionruntime "github.com/anthropic-lab/internkim/internal/companion"
)

func main() {
	if len(os.Args) > 1 && isCompanionSubcommand(os.Args[1]) {
		commandName := os.Args[1]
		switch commandName {
		case "pair":
			exit(runPair(os.Args[2:], http.DefaultClient))
			return
		case "run":
			exit(runCompanion(os.Args[2:], http.DefaultClient))
			return
		case "status":
			exit(runStatus(os.Args[2:]))
			return
		}
	}
	exit(runServer(os.Args[1:]))
}

func isCompanionSubcommand(commandName string) bool {
	switch commandName {
	case "pair", "run", "status":
		return true
	default:
		return false
	}
}

func runServer(arguments []string) error {
	flags := flag.NewFlagSet("serve", flag.ContinueOnError)
	listenAddress := flags.String("listen", "127.0.0.1:7979", "companion listen address")
	localOnly := flags.Bool("local-only", false, "advertise local-only mode")
	devMockLLM := flags.Bool("dev-mock-llm", false, "serve deterministic local LLM responses for development")
	if errorValue := flags.Parse(arguments); errorValue != nil {
		return errorValue
	}

	multiplexer := http.NewServeMux()
	multiplexer.HandleFunc("GET /health", func(responseWriter http.ResponseWriter, request *http.Request) {
		_ = request
		responseWriter.WriteHeader(http.StatusOK)
		_, _ = responseWriter.Write([]byte("ok\n"))
	})
	multiplexer.HandleFunc("GET /v1/capabilities", func(responseWriter http.ResponseWriter, request *http.Request) {
		_ = request
		writeJSON(responseWriter, capabilities.RegistryResponse{
			LocalOnly:    *localOnly,
			Capabilities: defaultCapabilities(*localOnly, *devMockLLM),
		})
	})
	multiplexer.HandleFunc("POST /v1/llm/structured", llmHandler(*devMockLLM, true))
	multiplexer.HandleFunc("POST /v1/llm/text", llmHandler(*devMockLLM, false))
	multiplexer.HandleFunc("POST /v1/tools/invoke", notImplemented)
	multiplexer.HandleFunc("POST /v1/tools/{toolName}/invoke", notImplemented)

	return http.ListenAndServe(*listenAddress, multiplexer)
}

type companionState struct {
	DeviceURL    string                    `json:"deviceURL"`
	CompanionID  string                    `json:"companionID"`
	Token        string                    `json:"token"`
	LocalOnly    bool                      `json:"localOnly"`
	Capabilities []capabilities.Descriptor `json:"capabilities"`
}

type companionJob struct {
	JobID   string                         `json:"jobID"`
	Status  string                         `json:"status"`
	Request capabilities.ToolInvokeRequest `json:"request"`
}

type companionStatusDocument struct {
	Paired       bool                      `json:"paired"`
	DeviceURL    string                    `json:"deviceURL,omitempty"`
	CompanionID  string                    `json:"companionID,omitempty"`
	LocalOnly    bool                      `json:"localOnly,omitempty"`
	Capabilities []capabilities.Descriptor `json:"capabilities,omitempty"`
}

func runPair(arguments []string, httpClient *http.Client) error {
	flags := flag.NewFlagSet("pair", flag.ContinueOnError)
	deviceURL := flags.String("device-url", "", "InternKim device URL")
	code := flags.String("code", "", "pairing code")
	statePath := flags.String("state", defaultStatePath(), "companion state path")
	localOnly := flags.Bool("local-only", false, "advertise local-only mode")
	devMockLLM := flags.Bool("dev-mock-llm", false, "advertise development mock LLM")
	if errorValue := flags.Parse(arguments); errorValue != nil {
		return errorValue
	}
	if flags.NArg() > 0 && (*deviceURL == "" || *code == "") {
		parsedDeviceURL, parsedCode, errorValue := parsePairingURL(flags.Arg(0))
		if errorValue != nil {
			return errorValue
		}
		*deviceURL = firstNonEmpty(*deviceURL, parsedDeviceURL)
		*code = firstNonEmpty(*code, parsedCode)
	}
	if *deviceURL == "" || *code == "" {
		return fmt.Errorf("pair requires --device-url and --code")
	}
	capabilityList := defaultCapabilities(*localOnly, *devMockLLM)
	requestBody := map[string]any{
		"code":         strings.TrimSpace(*code),
		"displayName":  defaultDisplayName(),
		"publicKey":    "development-public-key",
		"capabilities": capabilityList,
		"localOnly":    *localOnly,
	}
	var response struct {
		CompanionID string `json:"companionID"`
		Token       string `json:"token"`
	}
	if errorValue := postJSON(httpClient, strings.TrimRight(*deviceURL, "/")+"/_internkim/companion/pair", nil, requestBody, &response); errorValue != nil {
		return errorValue
	}
	state := companionState{
		DeviceURL:    strings.TrimRight(*deviceURL, "/"),
		CompanionID:  response.CompanionID,
		Token:        response.Token,
		LocalOnly:    *localOnly,
		Capabilities: capabilityList,
	}
	if errorValue := saveState(*statePath, state); errorValue != nil {
		return errorValue
	}
	fmt.Println("paired with " + state.DeviceURL)
	return nil
}

func runStatus(arguments []string) error {
	flags := flag.NewFlagSet("status", flag.ContinueOnError)
	statePath := flags.String("state", defaultStatePath(), "companion state path")
	jsonOutput := flags.Bool("json", false, "print machine-readable status")
	if errorValue := flags.Parse(arguments); errorValue != nil {
		return errorValue
	}
	state, errorValue := loadState(*statePath)
	if errorValue != nil {
		if *jsonOutput && errors.Is(errorValue, os.ErrNotExist) {
			writeJSONDocument(os.Stdout, companionStatusDocument{Paired: false})
			return nil
		}
		return errorValue
	}
	if *jsonOutput {
		writeJSONDocument(os.Stdout, companionStatusFromState(state))
		return nil
	}
	fmt.Println("device: " + state.DeviceURL)
	fmt.Println("companion: " + state.CompanionID)
	fmt.Printf("localOnly: %t\n", state.LocalOnly)
	fmt.Printf("capabilities: %d\n", len(state.Capabilities))
	return nil
}

func runCompanion(arguments []string, httpClient *http.Client) error {
	flags := flag.NewFlagSet("run", flag.ContinueOnError)
	statePath := flags.String("state", defaultStatePath(), "companion state path")
	runOnce := flags.Bool("once", false, "process one polling cycle")
	devMockLLM := flags.Bool("dev-mock-llm", false, "serve deterministic local LLM responses")
	allowStdinPrompts := flags.Bool("allow-stdin-prompts", false, "allow terminal prompts for user input capabilities")
	if errorValue := flags.Parse(arguments); errorValue != nil {
		return errorValue
	}
	state, errorValue := loadState(*statePath)
	if errorValue != nil {
		return errorValue
	}
	if *devMockLLM {
		state.Capabilities = defaultCapabilities(state.LocalOnly, true)
	}
	executor := companionruntime.Executor{
		DevMockLLM:    *devMockLLM,
		BrowserOpener: companionruntime.SystemBrowserOpener{},
	}
	if *allowStdinPrompts {
		executor.PromptHandler = companionruntime.TerminalPromptHandler{Reader: os.Stdin, Writer: os.Stdout}
	}
	for {
		if errorValue := sendHeartbeat(httpClient, state); errorValue != nil {
			return errorValue
		}
		job, errorValue := nextJob(httpClient, state)
		if errorValue != nil {
			return errorValue
		}
		if job == nil {
			if *runOnce {
				return nil
			}
			continue
		}
		response, executionError := executor.Execute(context.Background(), job.Request)
		if executionError != nil {
			_ = failJob(httpClient, state, job.JobID, executionError.Error())
		} else {
			_ = completeJob(httpClient, state, job.JobID, response)
		}
		if *runOnce {
			return executionError
		}
	}
}

func defaultCapabilities(localOnly bool, devMockLLM bool) []capabilities.Descriptor {
	descriptors := capabilities.CompanionToolDescriptors()
	for index, descriptor := range descriptors {
		if strings.HasPrefix(descriptor.Name, "browser.") {
			descriptors[index].WorksOffline = localOnly
		}
	}
	if devMockLLM {
		descriptors = append(descriptors, capabilities.CompanionLLMDescriptors()...)
	}
	return descriptors
}

func llmHandler(isEnabled bool, isStructured bool) http.HandlerFunc {
	return func(responseWriter http.ResponseWriter, request *http.Request) {
		if !isEnabled {
			notImplemented(responseWriter, request)
			return
		}
		response := map[string]any{
			"provider":        "companion",
			"model":           "mock-local",
			"selectedBackend": capabilities.LLMBackendCompanionLocal,
			"constraintMode":  "prompt_validation",
			"content":         "ok",
		}
		if isStructured {
			document, _ := io.ReadAll(request.Body)
			response["content"] = companionruntime.MockStructuredContent(document)
		}
		writeJSON(responseWriter, response)
	}
}

func notImplemented(responseWriter http.ResponseWriter, request *http.Request) {
	_ = request
	http.Error(responseWriter, "companion capability is not implemented in this daemon slice", http.StatusNotImplemented)
}

func writeJSON(responseWriter http.ResponseWriter, response any) {
	responseWriter.Header().Set("Content-Type", "application/json")
	writeJSONDocument(responseWriter, response)
}

func writeJSONDocument(writer io.Writer, response any) {
	_ = json.NewEncoder(writer).Encode(response)
}

func sendHeartbeat(httpClient *http.Client, state companionState) error {
	return postJSON(httpClient, state.DeviceURL+"/_internkim/companion/heartbeat", companionHeaders(state), map[string]any{
		"capabilities": state.Capabilities,
		"localOnly":    state.LocalOnly,
	}, &map[string]any{})
}

func nextJob(httpClient *http.Client, state companionState) (*companionJob, error) {
	request, errorValue := http.NewRequest(http.MethodGet, state.DeviceURL+"/_internkim/companion/jobs/next", nil)
	if errorValue != nil {
		return nil, errorValue
	}
	for key, value := range companionHeaders(state) {
		request.Header.Set(key, value)
	}
	response, errorValue := httpClient.Do(request)
	if errorValue != nil {
		return nil, errorValue
	}
	defer response.Body.Close()
	if response.StatusCode >= http.StatusBadRequest {
		body, _ := io.ReadAll(response.Body)
		return nil, errors.New(string(body))
	}
	var job companionJob
	if errorValue := json.NewDecoder(response.Body).Decode(&job); errorValue != nil {
		return nil, errorValue
	}
	if job.Status == "empty" || job.JobID == "" {
		return nil, nil
	}
	return &job, nil
}

func completeJob(httpClient *http.Client, state companionState, jobID string, response capabilities.ToolInvokeResponse) error {
	return postJSON(httpClient, state.DeviceURL+"/_internkim/companion/jobs/"+url.PathEscape(jobID)+"/complete", companionHeaders(state), response, &map[string]any{})
}

func failJob(httpClient *http.Client, state companionState, jobID string, errorMessage string) error {
	return postJSON(httpClient, state.DeviceURL+"/_internkim/companion/jobs/"+url.PathEscape(jobID)+"/fail", companionHeaders(state), map[string]string{"error": errorMessage}, &map[string]any{})
}

func postJSON(httpClient *http.Client, endpoint string, headers map[string]string, requestBody any, responseBody any) error {
	document, errorValue := json.Marshal(requestBody)
	if errorValue != nil {
		return errorValue
	}
	request, errorValue := http.NewRequest(http.MethodPost, endpoint, bytes.NewReader(document))
	if errorValue != nil {
		return errorValue
	}
	request.Header.Set("Content-Type", "application/json")
	for key, value := range headers {
		request.Header.Set(key, value)
	}
	response, errorValue := httpClient.Do(request)
	if errorValue != nil {
		return errorValue
	}
	defer response.Body.Close()
	if response.StatusCode >= http.StatusBadRequest {
		body, _ := io.ReadAll(response.Body)
		return errors.New(string(body))
	}
	return json.NewDecoder(response.Body).Decode(responseBody)
}

func companionHeaders(state companionState) map[string]string {
	return map[string]string{
		"X-InternKim-Companion-ID":    state.CompanionID,
		"X-InternKim-Companion-Token": state.Token,
	}
}

func saveState(path string, state companionState) error {
	document, errorValue := json.MarshalIndent(state, "", "  ")
	if errorValue != nil {
		return errorValue
	}
	if errorValue := os.MkdirAll(filepath.Dir(path), 0o700); errorValue != nil {
		return errorValue
	}
	return os.WriteFile(path, document, 0o600)
}

func loadState(path string) (companionState, error) {
	document, errorValue := os.ReadFile(path)
	if errorValue != nil {
		return companionState{}, errorValue
	}
	var state companionState
	if errorValue := json.Unmarshal(document, &state); errorValue != nil {
		return companionState{}, errorValue
	}
	return state, nil
}

func companionStatusFromState(state companionState) companionStatusDocument {
	return companionStatusDocument{
		Paired:       state.DeviceURL != "" && state.CompanionID != "" && state.Token != "",
		DeviceURL:    state.DeviceURL,
		CompanionID:  state.CompanionID,
		LocalOnly:    state.LocalOnly,
		Capabilities: state.Capabilities,
	}
}

func defaultStatePath() string {
	homeDirectory, errorValue := os.UserHomeDir()
	if errorValue != nil || homeDirectory == "" {
		return ".internkim-companion.json"
	}
	return filepath.Join(homeDirectory, ".internkim-companion", "state.json")
}

func defaultDisplayName() string {
	hostname, errorValue := os.Hostname()
	if errorValue != nil || strings.TrimSpace(hostname) == "" {
		return "Companion"
	}
	return hostname
}

func parsePairingURL(value string) (string, string, error) {
	parsedURL, errorValue := url.Parse(strings.TrimSpace(value))
	if errorValue != nil {
		return "", "", errorValue
	}
	if parsedURL.Scheme != "internkim" || parsedURL.Host != "pair" {
		return "", "", fmt.Errorf("pairing URL must start with internkim://pair")
	}
	deviceURL := strings.TrimSpace(parsedURL.Query().Get("device_url"))
	code := strings.TrimSpace(parsedURL.Query().Get("code"))
	if deviceURL == "" || code == "" {
		return "", "", fmt.Errorf("pairing URL requires device_url and code")
	}
	return deviceURL, code, nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		trimmedValue := strings.TrimSpace(value)
		if trimmedValue != "" {
			return trimmedValue
		}
	}
	return ""
}

func exit(errorValue error) {
	if errorValue == nil {
		return
	}
	fmt.Fprintln(os.Stderr, errorValue.Error())
	os.Exit(1)
}
