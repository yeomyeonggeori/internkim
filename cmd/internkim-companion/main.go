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

	browserruntime "gitlab.com/eastriver/internkim/internal/browser"
	"gitlab.com/eastriver/internkim/internal/capabilities"
	companionruntime "gitlab.com/eastriver/internkim/internal/companion"
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
		case "runtime-model", "remote-model":
			exit(runRuntimeModel(os.Args[2:], http.DefaultClient, companionruntime.NewDefaultSecureStore()))
			return
		case "disconnect":
			exit(runDisconnect(os.Args[2:], http.DefaultClient, companionruntime.NewDefaultSecureStore()))
			return
		case "status":
			exit(runStatus(os.Args[2:], http.DefaultClient, companionruntime.NewDefaultSecureStore()))
			return
		}
	}
	exit(runServer(os.Args[1:]))
}

func isCompanionSubcommand(commandName string) bool {
	switch commandName {
	case "pair", "run", "runtime-model", "remote-model", "disconnect", "status":
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
	localLLMFlags := registerLocalLLMFlags(flags)
	if errorValue := flags.Parse(arguments); errorValue != nil {
		return errorValue
	}

	settings := newDynamicLocalLLM(localLLMFlags.settings(http.DefaultClient))

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
			Capabilities: companionruntime.DefaultCapabilities(*localOnly, *devMockLLM || settings.currentSettings().Enabled),
		})
	})
	multiplexer.HandleFunc("POST /v1/llm/structured", llmHandler(settings, *devMockLLM, true))
	multiplexer.HandleFunc("POST /v1/llm/text", llmHandler(settings, *devMockLLM, false))
	multiplexer.HandleFunc("POST /v1/llm/stream", llmStreamHandler(settings))
	multiplexer.HandleFunc("POST /v1/embedding/create", embeddingHandler(settings, *devMockLLM))
	multiplexer.HandleFunc("POST /v1/audio/in", reservedNotImplemented)
	multiplexer.HandleFunc("POST /v1/audio/out", reservedNotImplemented)
	multiplexer.HandleFunc("POST /v1/tools/invoke", notImplemented)
	multiplexer.HandleFunc("POST /v1/tools/{toolName}/invoke", notImplemented)

	listener, errorValue := listenLoopbackOnly("tcp", *listenAddress, "companion server")
	if errorValue != nil {
		return errorValue
	}
	return http.Serve(listener, multiplexer)
}

type companionStatusDocument struct {
	Paired                    bool                      `json:"paired"`
	AuthStatus                string                    `json:"authStatus"`
	DeviceURL                 string                    `json:"deviceURL,omitempty"`
	CompanionID               string                    `json:"companionID,omitempty"`
	LocalOnly                 bool                      `json:"localOnly,omitempty"`
	Capabilities              []capabilities.Descriptor `json:"capabilities,omitempty"`
	ExtensionAutomationStatus string                    `json:"extensionAutomationStatus,omitempty"`
	ExtensionAutomationError  string                    `json:"extensionAutomationError,omitempty"`
	AgentBrowserCLIStatus     string                    `json:"agentBrowserCLIStatus,omitempty"`
	AgentBrowserCLIPath       string                    `json:"agentBrowserCLIPath,omitempty"`
}

const (
	companionAuthStatusUnpaired          = "unpaired"
	companionAuthStatusLocalOnly         = "local-only"
	companionAuthStatusMissingSigningKey = "missing-signing-key"
	companionAuthStatusVerified          = "verified"
	companionAuthStatusReconnectRequired = "reconnect-required"
	companionAuthStatusUnknown           = "unknown"
)

type browserAutoApprovalHandler struct{}

func (handler browserAutoApprovalHandler) Approve(ctx context.Context, request companionruntime.ApprovalRequest) (companionruntime.ApprovalDecision, error) {
	_ = ctx
	if request.CapabilityScope == "browser" {
		return companionruntime.ApprovalDecision{Allowed: true}, nil
	}
	return companionruntime.ApprovalDecision{Allowed: false, SuggestedConstraint: "automatic approval is limited to browser grants"}, nil
}

func registerStateFlag(flags *flag.FlagSet) *string {
	return flags.String("state", defaultStatePath(), "companion state path")
}

func runPair(arguments []string, httpClient *http.Client) error {
	return runPairWithStore(arguments, httpClient, companionruntime.NewDefaultSecureStore())
}

func runPairWithStore(arguments []string, httpClient *http.Client, secureStore companionruntime.SecureStore) error {
	flags := flag.NewFlagSet("pair", flag.ContinueOnError)
	deviceURL := flags.String("device-url", "", "InternKim device URL")
	code := flags.String("code", "", "pairing code")
	statePath := registerStateFlag(flags)
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
	keyPair, errorValue := companionruntime.GenerateKeyPair()
	if errorValue != nil {
		return errorValue
	}
	capabilityList := companionruntime.DefaultCapabilities(*localOnly, *devMockLLM)
	requestBody := map[string]any{
		"code":         strings.TrimSpace(*code),
		"displayName":  defaultDisplayName(),
		"publicKey":    keyPair.PublicKey,
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
	privateKeyID := companionPrivateKeyID(response.CompanionID)
	if errorValue := secureStore.Put(context.Background(), privateKeyID, keyPair.PrivateKey); errorValue != nil {
		return errorValue
	}
	state := companionruntime.State{
		DeviceURL:    strings.TrimRight(*deviceURL, "/"),
		CompanionID:  response.CompanionID,
		Token:        response.Token,
		PublicKey:    keyPair.PublicKey,
		PrivateKeyID: privateKeyID,
		LocalOnly:    *localOnly,
		Capabilities: capabilityList,
	}
	if errorValue := saveState(*statePath, state); errorValue != nil {
		return errorValue
	}
	fmt.Println("paired with " + state.DeviceURL)
	return nil
}

func runDisconnect(arguments []string, httpClient *http.Client, secureStore companionruntime.SecureStore) error {
	flags := flag.NewFlagSet("disconnect", flag.ContinueOnError)
	statePath := registerStateFlag(flags)
	if errorValue := flags.Parse(arguments); errorValue != nil {
		return errorValue
	}
	state, errorValue := loadStateAndMigrateSecrets(context.Background(), *statePath, secureStore)
	if errorValue == nil {
		privateKey, keyError := secureStore.Get(context.Background(), state.PrivateKeyID)
		if keyError == nil {
			deviceClient := companionruntime.DeviceClient{HTTPClient: httpClient, State: state, PrivateKey: privateKey}
			_ = deviceClient.PostSignedJSON(state.DeviceURL+"/_internkim/companion/disconnect", map[string]any{}, nil)
		}
	}
	if state.PrivateKeyID != "" {
		if deleteError := secureStore.Delete(context.Background(), state.PrivateKeyID); deleteError != nil {
			return deleteError
		}
	}
	if removeError := os.Remove(*statePath); removeError != nil && !errors.Is(removeError, os.ErrNotExist) {
		return removeError
	}
	_ = os.Remove(defaultMountStatePath(*statePath))
	_ = os.Remove(defaultHandoffStatePath(*statePath))
	fmt.Println("disconnected")
	return nil
}

func runStatus(arguments []string, httpClient *http.Client, secureStore companionruntime.SecureStore) error {
	flags := flag.NewFlagSet("status", flag.ContinueOnError)
	statePath := registerStateFlag(flags)
	jsonOutput := flags.Bool("json", false, "print machine-readable status")
	verifyAuth := flags.Bool("verify-auth", false, "verify companion auth with the paired device")
	browserExecutablePath := flags.String("browser-executable", defaultBrowserExecutablePath(), "browser executable path")
	browserExtensionPath := flags.String("browser-extension-path", "", "companion browser extension directory path")
	agentBrowserPath := flags.String("agent-browser-path", "", "legacy agent-browser CLI path (status-only; browser automation no longer uses it)")
	if errorValue := flags.Parse(arguments); errorValue != nil {
		return errorValue
	}
	state, errorValue := loadState(*statePath)
	if errorValue != nil {
		if *jsonOutput && errors.Is(errorValue, os.ErrNotExist) {
			writeJSONDocument(os.Stdout, companionStatusDocument{Paired: false, AuthStatus: companionAuthStatusUnpaired})
			return nil
		}
		return errorValue
	}
	report := companionBrowserAutomationReadiness(*browserExecutablePath, *browserExtensionPath, *agentBrowserPath)
	authStatus := companionAuthStatusFromState(state, *verifyAuth, *statePath, httpClient, secureStore)
	document := companionStatusFromState(state, report.Extension, authStatus)
	document.AgentBrowserCLIPath = report.AgentBrowserPath
	document.AgentBrowserCLIStatus = agentBrowserCLIStatusLabel(report.AgentBrowserFound)
	if *jsonOutput {
		writeJSONDocument(os.Stdout, document)
		return nil
	}
	printCompanionStatusText(state, document)
	return nil
}

func agentBrowserCLIStatusLabel(found bool) string {
	if found {
		return "found"
	}
	return "not_found"
}

func printCompanionStatusText(state companionruntime.State, document companionStatusDocument) {
	fmt.Println("device: " + state.DeviceURL)
	fmt.Println("companion: " + state.CompanionID)
	fmt.Printf("localOnly: %t\n", state.LocalOnly)
	fmt.Printf("capabilities: %d\n", len(state.Capabilities))
	fmt.Println("extension automation: " + firstNonEmpty(document.ExtensionAutomationStatus, "unknown"))
	fmt.Println("agent-browser CLI (legacy/status-only): " + document.AgentBrowserCLIStatus)
}

func companionAuthStatusFromState(state companionruntime.State, shouldVerify bool, statePath string, httpClient *http.Client, secureStore companionruntime.SecureStore) string {
	if state.DeviceURL == "" || state.CompanionID == "" || state.Token == "" {
		if state.LocalOnly {
			return companionAuthStatusLocalOnly
		}
		return companionAuthStatusUnpaired
	}
	if !shouldVerify {
		return companionAuthStatusUnknown
	}
	verifiedState, privateKey, errorValue := loadCompanionStateAndPrivateKey(statePath, secureStore)
	if errorValue != nil {
		return companionAuthStatusMissingSigningKey
	}
	var response map[string]string
	endpoint := verifiedState.DeviceURL + "/_internkim/companion/auth/check"
	deviceClient := companionruntime.DeviceClient{HTTPClient: httpClient, State: verifiedState, PrivateKey: privateKey}
	errorValue = deviceClient.SignedJSONRequest(http.MethodGet, endpoint, nil, &response)
	if errorValue == nil {
		return companionAuthStatusVerified
	}
	if companionruntime.IsCompanionAuthRequiredError(errorValue) {
		return companionAuthStatusReconnectRequired
	}
	return companionAuthStatusUnknown
}

func runCompanion(arguments []string, httpClient *http.Client) error {
	return runCompanionWithStore(arguments, httpClient, companionruntime.NewDefaultSecureStore())
}

func runCompanionWithStore(arguments []string, httpClient *http.Client, secureStore companionruntime.SecureStore) error {
	flags := flag.NewFlagSet("run", flag.ContinueOnError)
	statePath := registerStateFlag(flags)
	runOnce := flags.Bool("once", false, "process one polling cycle")
	devMockLLM := flags.Bool("dev-mock-llm", false, "serve deterministic local LLM responses")
	allowStdinPrompts := flags.Bool("allow-stdin-prompts", false, "allow terminal prompts for user input capabilities")
	shellBridgeURL := flags.String("shell-bridge-url", "", "local companion shell bridge URL")
	shellBridgeToken := flags.String("shell-bridge-token", "", "local companion shell bridge token")
	controlListenAddress := flags.String("control-listen", "", "local companion shell control address")
	browserExecutablePath := flags.String("browser-executable", defaultBrowserExecutablePath(), "browser executable path")
	browserProfilePath := flags.String("browser-profile", defaultBrowserProfilePath(), "InternKim companion browser profile path")
	browserExtensionPath := flags.String("browser-extension-path", "", "companion browser extension directory path")
	developmentAutoApproveBrowser := flags.Bool("development-auto-approve-browser", false, "automatically approve browser grants for local E2E")
	localLLMFlags := registerLocalLLMFlags(flags)
	preferCompanionBrowser := flags.Bool("prefer-companion-browser", false, "ask the device to route browser tools to this companion")
	if errorValue := flags.Parse(arguments); errorValue != nil {
		return errorValue
	}
	localLLMConfiguration := localLLMFlags.settings(httpClient)
	localLLM := newDynamicLocalLLM(localLLMConfiguration)
	state, errorValue := loadStateAndMigrateSecrets(context.Background(), *statePath, secureStore)
	if errorValue != nil {
		return errorValue
	}
	privateKey, errorValue := secureStore.Get(context.Background(), state.PrivateKeyID)
	if errorValue != nil {
		return errorValue
	}
	if *devMockLLM {
		state.Capabilities = companionruntime.DefaultCapabilities(state.LocalOnly, true)
	}
	resolvedBrowserExtensionPath := resolveBrowserExtensionPath(*browserExtensionPath)
	inputSynthesizer, errorValue := browserruntime.NewPlatformInputSynthesizer()
	if errorValue != nil {
		return errorValue
	}
	browserRuntime := &browserruntime.ExtensionInputRuntime{
		ChromeExecutablePath: *browserExecutablePath,
		ExtensionPath:        resolvedBrowserExtensionPath,
		ProfilePath:          *browserProfilePath,
		SessionName:          "internkim",
		InputSynthesizer:     inputSynthesizer,
	}
	readiness := extensionBrowserRuntimeReadiness(*browserExecutablePath, resolvedBrowserExtensionPath)
	if readiness.Status != "ready" {
		state.Capabilities = companionruntime.CapabilitiesWithoutBrowser(state.Capabilities)
	}
	grantStore := companionruntime.NewMemoryGrantStore()
	mountStore := companionruntime.NewMountStore(defaultMountStatePath(*statePath))
	handoffStore := companionruntime.NewPersistentBrowserHandoffStore(defaultHandoffStatePath(*statePath))
	runtimeStatus := &runtimeState{}
	if localLLMConfiguration.Enabled {
		runtimeStatus.replaceLocalLLM(localLLMConfiguration)
	}
	executor := companionruntime.NewExecutor(*devMockLLM, localLLM, localLLM, browserRuntime, handoffStore, mountStore, grantStore)
	if readiness.Status != "ready" {
		executor.BrowserRuntime = nil
	}
	deviceClient := companionruntime.DeviceClient{HTTPClient: httpClient, State: state, PrivateKey: privateKey}
	handoffCompletionHandler := companionruntime.JobRunner{DeviceClient: deviceClient}.CompleteHandoff
	controlServer, errorValue := startControlServer(*controlListenAddress, grantStore, mountStore, handoffStore, executor.BrowserRuntime, handoffCompletionHandler, runtimeStatus, localLLM, httpClient)
	if errorValue != nil {
		return errorValue
	}
	if controlServer != nil {
		defer controlServer.Close()
	}
	if *allowStdinPrompts {
		executor.PromptHandler = companionruntime.TerminalPromptHandler{Reader: os.Stdin, Writer: os.Stdout}
	}
	if *developmentAutoApproveBrowser {
		executor.ApprovalHandler = browserAutoApprovalHandler{}
	}
	if strings.TrimSpace(*shellBridgeURL) != "" {
		shellBridgeHandler := companionruntime.ShellBridgePromptHandler{
			BaseURL:    *shellBridgeURL,
			Token:      *shellBridgeToken,
			HTTPClient: httpClient,
		}
		executor.PromptHandler = shellBridgeHandler
		executor.ApprovalHandler = shellBridgeHandler
		executor.FilePicker = shellBridgeHandler
		executor.DirectoryPicker = shellBridgeHandler
		executor.FileUploader = companionruntime.DeviceFileUploader{DeviceClient: deviceClient}
	}
	jobRunner := companionruntime.JobRunner{
		DeviceClient:           deviceClient,
		Executor:               executor,
		Runtime:                runtimeStatus,
		MountStore:             mountStore,
		PreferCompanionBrowser: *preferCompanionBrowser,
		RunOnce:                *runOnce,
	}
	heartbeatContext, stopHeartbeatLoop := context.WithCancel(context.Background())
	defer stopHeartbeatLoop()
	if !*runOnce {
		go jobRunner.RunHeartbeatLoop(heartbeatContext)
	}
	return jobRunner.Run(context.Background())
}

func writeJSON(responseWriter http.ResponseWriter, response any) {
	responseWriter.Header().Set("Content-Type", "application/json")
	writeJSONDocument(responseWriter, response)
}

func writeJSONDocument(writer io.Writer, response any) {
	_ = json.NewEncoder(writer).Encode(response)
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
	return companionruntime.DecodeJSONResponse(endpoint, response, responseBody)
}

func saveState(path string, state companionruntime.State) error {
	document, errorValue := json.MarshalIndent(state, "", "  ")
	if errorValue != nil {
		return errorValue
	}
	if errorValue := os.MkdirAll(filepath.Dir(path), 0o700); errorValue != nil {
		return errorValue
	}
	return os.WriteFile(path, document, 0o600)
}

func loadState(path string) (companionruntime.State, error) {
	document, errorValue := os.ReadFile(path)
	if errorValue != nil {
		return companionruntime.State{}, errorValue
	}
	var state companionruntime.State
	if errorValue := json.Unmarshal(document, &state); errorValue != nil {
		return companionruntime.State{}, errorValue
	}
	return state, nil
}

func loadStateAndMigrateSecrets(ctx context.Context, path string, secureStore companionruntime.SecureStore) (companionruntime.State, error) {
	state, errorValue := loadState(path)
	if errorValue != nil {
		return companionruntime.State{}, errorValue
	}
	if state.PrivateKey == "" {
		return state, nil
	}
	privateKeyID := firstNonEmpty(state.PrivateKeyID, companionPrivateKeyID(state.CompanionID))
	if errorValue := secureStore.Put(ctx, privateKeyID, state.PrivateKey); errorValue != nil {
		return companionruntime.State{}, errorValue
	}
	state.PrivateKeyID = privateKeyID
	state.PrivateKey = ""
	if errorValue := saveState(path, state); errorValue != nil {
		return companionruntime.State{}, errorValue
	}
	return state, nil
}

func companionStatusFromState(state companionruntime.State, readiness browserruntime.RuntimeReadiness, authStatus string) companionStatusDocument {
	capabilityList := state.Capabilities
	if readiness.Status != "" && readiness.Status != "ready" {
		capabilityList = companionruntime.CapabilitiesWithoutBrowser(capabilityList)
	}
	return companionStatusDocument{
		Paired:                    state.DeviceURL != "" && state.CompanionID != "" && state.Token != "",
		AuthStatus:                firstNonEmpty(authStatus, companionAuthStatusUnknown),
		DeviceURL:                 state.DeviceURL,
		CompanionID:               state.CompanionID,
		LocalOnly:                 state.LocalOnly,
		Capabilities:              capabilityList,
		ExtensionAutomationStatus: readiness.Status,
		ExtensionAutomationError:  readiness.Error,
	}
}

func defaultStatePath() string {
	homeDirectory, errorValue := os.UserHomeDir()
	if errorValue != nil || homeDirectory == "" {
		return ".internkim-companion.json"
	}
	return filepath.Join(homeDirectory, ".internkim-companion", "state.json")
}

func defaultMountStatePath(statePath string) string {
	trimmedPath := strings.TrimSpace(statePath)
	if trimmedPath != "" {
		return filepath.Join(filepath.Dir(trimmedPath), "mounts.json")
	}
	homeDirectory, errorValue := os.UserHomeDir()
	if errorValue != nil || homeDirectory == "" {
		return ".internkim-companion-mounts.json"
	}
	return filepath.Join(homeDirectory, ".internkim-companion", "mounts.json")
}

func defaultHandoffStatePath(statePath string) string {
	trimmedPath := strings.TrimSpace(statePath)
	if trimmedPath != "" {
		return filepath.Join(filepath.Dir(trimmedPath), "browser-handoff.json")
	}
	homeDirectory, errorValue := os.UserHomeDir()
	if errorValue != nil || homeDirectory == "" {
		return ".internkim-companion-browser-handoff.json"
	}
	return filepath.Join(homeDirectory, ".internkim-companion", "browser-handoff.json")
}

func companionPrivateKeyID(companionID string) string {
	return "internkim-companion-" + companionID + "-signing-key"
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
