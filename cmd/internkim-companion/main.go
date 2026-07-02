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
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	browserruntime "gitlab.com/eastriver/internkim/internal/browser"
	"gitlab.com/eastriver/internkim/internal/capabilities"
	companionruntime "gitlab.com/eastriver/internkim/internal/companion"
	"gitlab.com/eastriver/internkim/internal/llmbackend"
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

	return http.ListenAndServe(*listenAddress, multiplexer)
}

type companionStatusDocument struct {
	Paired               bool                      `json:"paired"`
	AuthStatus           string                    `json:"authStatus"`
	DeviceURL            string                    `json:"deviceURL,omitempty"`
	CompanionID          string                    `json:"companionID,omitempty"`
	LocalOnly            bool                      `json:"localOnly,omitempty"`
	Capabilities         []capabilities.Descriptor `json:"capabilities,omitempty"`
	BrowserRuntimeStatus string                    `json:"browserRuntimeStatus,omitempty"`
	BrowserRuntimeError  string                    `json:"browserRuntimeError,omitempty"`
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

func runPair(arguments []string, httpClient *http.Client) error {
	return runPairWithStore(arguments, httpClient, companionruntime.NewDefaultSecureStore())
}

func runPairWithStore(arguments []string, httpClient *http.Client, secureStore companionruntime.SecureStore) error {
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
	statePath := flags.String("state", defaultStatePath(), "companion state path")
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
	statePath := flags.String("state", defaultStatePath(), "companion state path")
	jsonOutput := flags.Bool("json", false, "print machine-readable status")
	verifyAuth := flags.Bool("verify-auth", false, "verify companion auth with the paired device")
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
	if *jsonOutput {
		agentBrowserPath := resolveAgentBrowserPath("")
		readiness := browserruntime.AgentBrowserRuntime{
			CommandPath:          agentBrowserPath,
			Engine:               browserruntime.BrowserEngineChrome,
			EngineExecutablePath: defaultBrowserExecutablePath(),
		}.Check(context.Background())
		authStatus := companionAuthStatusFromState(state, *verifyAuth, *statePath, httpClient, secureStore)
		writeJSONDocument(os.Stdout, companionStatusFromState(state, readiness, authStatus))
		return nil
	}
	fmt.Println("device: " + state.DeviceURL)
	fmt.Println("companion: " + state.CompanionID)
	fmt.Printf("localOnly: %t\n", state.LocalOnly)
	fmt.Printf("capabilities: %d\n", len(state.Capabilities))
	agentBrowserPath := resolveAgentBrowserPath("")
	readiness := browserruntime.AgentBrowserRuntime{
		CommandPath:          agentBrowserPath,
		Engine:               browserruntime.BrowserEngineChrome,
		EngineExecutablePath: defaultBrowserExecutablePath(),
	}.Check(context.Background())
	fmt.Println("browserRuntime: " + firstNonEmpty(readiness.Status, "unknown"))
	return nil
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
	statePath := flags.String("state", defaultStatePath(), "companion state path")
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
	_ = preferCompanionBrowser
	executor := companionruntime.Executor{
		DevMockLLM:     *devMockLLM,
		LLMChain:       localLLM,
		EmbeddingChain: localLLM,
		BrowserRuntime: browserRuntime,
		HandoffStore:   handoffStore,
		GrantStore:     grantStore,
		MountStore:     mountStore,
	}
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

func llmHandler(settings *dynamicLocalLLM, devMock bool, isStructured bool) http.HandlerFunc {
	return func(responseWriter http.ResponseWriter, request *http.Request) {
		if devMock {
			respondWithDevMock(responseWriter, request, isStructured)
			return
		}
		if !settings.currentSettings().Enabled {
			notImplemented(responseWriter, request)
			return
		}
		if isStructured {
			handleStructuredLLM(responseWriter, request, settings)
			return
		}
		handleTextLLM(responseWriter, request, settings)
	}
}

func respondWithDevMock(responseWriter http.ResponseWriter, request *http.Request, isStructured bool) {
	response := map[string]any{
		"provider":        "companion",
		"model":           "mock-local",
		"selectedBackend": capabilities.LLMBackendCompanionLocal,
		"constraintMode":  llmbackend.ConstraintModeOpenAIJSONSchema,
		"content":         "ok",
	}
	if isStructured {
		document, _ := io.ReadAll(request.Body)
		response["content"] = companionruntime.MockStructuredContent(document)
	}
	writeJSON(responseWriter, response)
}

func handleStructuredLLM(responseWriter http.ResponseWriter, request *http.Request, settings *dynamicLocalLLM) {
	var structuredRequest llmbackend.StructuredRequest
	if errorValue := json.NewDecoder(request.Body).Decode(&structuredRequest); errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusBadRequest)
		return
	}
	providerSet, isEnabled := settings.providerSetFor(structuredRequest.Provider, structuredRequest.Accelerator)
	if !isEnabled {
		notImplemented(responseWriter, request)
		return
	}
	response, errorValue := providerSet.Provider.CompleteStructured(request.Context(), structuredRequest)
	if errorValue != nil {
		respondWithBackendError(responseWriter, errorValue, providerSet.Backends)
		return
	}
	response.SelectedBackend = capabilities.LLMBackendCompanionLocal
	writeJSON(responseWriter, response)
}

func handleTextLLM(responseWriter http.ResponseWriter, request *http.Request, settings *dynamicLocalLLM) {
	var textRequest llmbackend.TextRequest
	if errorValue := json.NewDecoder(request.Body).Decode(&textRequest); errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusBadRequest)
		return
	}
	providerSet, isEnabled := settings.providerSetFor(textRequest.Provider, textRequest.Accelerator)
	if !isEnabled {
		notImplemented(responseWriter, request)
		return
	}
	response, errorValue := providerSet.Provider.CompleteText(request.Context(), textRequest)
	if errorValue != nil {
		respondWithBackendError(responseWriter, errorValue, providerSet.Backends)
		return
	}
	response.SelectedBackend = capabilities.LLMBackendCompanionLocal
	writeJSON(responseWriter, response)
}

func embeddingHandler(settings *dynamicLocalLLM, devMock bool) http.HandlerFunc {
	return func(responseWriter http.ResponseWriter, request *http.Request) {
		if devMock {
			writeJSON(responseWriter, map[string]any{
				"provider":        "companion",
				"model":           llmbackend.DefaultEmbeddingGemmaModel,
				"selectedBackend": capabilities.LLMBackendCompanionLocal,
				"embedding":       []float64{1, 0, 0},
			})
			return
		}
		if !settings.currentSettings().Enabled {
			notImplemented(responseWriter, request)
			return
		}
		handleEmbedding(responseWriter, request, settings)
	}
}

func handleEmbedding(responseWriter http.ResponseWriter, request *http.Request, settings *dynamicLocalLLM) {
	var embeddingRequest llmbackend.EmbeddingRequest
	if errorValue := json.NewDecoder(request.Body).Decode(&embeddingRequest); errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusBadRequest)
		return
	}
	providerSet, isEnabled := settings.embeddingProviderSetFor(embeddingRequest.Provider)
	if !isEnabled {
		notImplemented(responseWriter, request)
		return
	}
	response, errorValue := providerSet.Provider.CreateEmbedding(request.Context(), embeddingRequest)
	if errorValue != nil {
		respondWithEmbeddingBackendError(responseWriter, errorValue, providerSet.Backends)
		return
	}
	response.SelectedBackend = capabilities.LLMBackendCompanionLocal
	writeJSON(responseWriter, response)
}

func respondWithBackendError(responseWriter http.ResponseWriter, errorValue error, backends []llmbackend.Backend) {
	hint := buildBackendHint(backends)
	responseWriter.Header().Set("Content-Type", "application/json")
	responseWriter.WriteHeader(http.StatusServiceUnavailable)
	writeJSONDocument(responseWriter, map[string]any{
		"error": errorValue.Error(),
		"hint":  hint,
	})
}

func respondWithEmbeddingBackendError(responseWriter http.ResponseWriter, errorValue error, backends []llmbackend.EmbeddingBackend) {
	hint := buildEmbeddingBackendHint(backends)
	responseWriter.Header().Set("Content-Type", "application/json")
	responseWriter.WriteHeader(http.StatusServiceUnavailable)
	writeJSONDocument(responseWriter, map[string]any{
		"error": errorValue.Error(),
		"hint":  hint,
	})
}

func buildBackendHint(backends []llmbackend.Backend) string {
	if len(backends) == 0 {
		return "no local backend configured"
	}
	return "check that one of these backends is reachable: " + localLLMProviderNames(backends)
}

func buildEmbeddingBackendHint(backends []llmbackend.EmbeddingBackend) string {
	if len(backends) == 0 {
		return "no local embedding backend configured"
	}
	return "check that one of these embedding backends is reachable: " + localEmbeddingProviderNames(backends)
}

func notImplemented(responseWriter http.ResponseWriter, request *http.Request) {
	_ = request
	http.Error(responseWriter, "companion capability is not implemented in this daemon slice", http.StatusNotImplemented)
}

func reservedNotImplemented(responseWriter http.ResponseWriter, request *http.Request) {
	_ = request
	http.Error(responseWriter, "reserved endpoint; not yet implemented", http.StatusNotImplemented)
}

func llmStreamHandler(settings *dynamicLocalLLM) http.HandlerFunc {
	return func(responseWriter http.ResponseWriter, request *http.Request) {
		var textRequest llmbackend.TextRequest
		if errorValue := json.NewDecoder(request.Body).Decode(&textRequest); errorValue != nil {
			http.Error(responseWriter, errorValue.Error(), http.StatusBadRequest)
			return
		}
		providerSet, isEnabled := settings.providerSetFor(textRequest.Provider, textRequest.Accelerator)
		if !isEnabled {
			notImplemented(responseWriter, request)
			return
		}
		flusher, supportsFlush := responseWriter.(http.Flusher)
		if !supportsFlush {
			http.Error(responseWriter, "streaming not supported by responder", http.StatusInternalServerError)
			return
		}
		responseWriter.Header().Set("Content-Type", "text/event-stream")
		responseWriter.Header().Set("Cache-Control", "no-cache")
		responseWriter.Header().Set("Connection", "keep-alive")
		errorValue := llmbackend.StreamFirstStreamingBackend(request.Context(), providerSet.Backends, textRequest, func(token string) {
			payload, _ := json.Marshal(map[string]string{"token": token})
			_, _ = responseWriter.Write([]byte("data: "))
			_, _ = responseWriter.Write(payload)
			_, _ = responseWriter.Write([]byte("\n\n"))
			flusher.Flush()
		})
		if errorValue != nil {
			payload, _ := json.Marshal(map[string]string{"error": errorValue.Error()})
			_, _ = responseWriter.Write([]byte("event: error\ndata: "))
			_, _ = responseWriter.Write(payload)
			_, _ = responseWriter.Write([]byte("\n\n"))
			flusher.Flush()
			return
		}
		_, _ = responseWriter.Write([]byte("event: done\ndata: {}\n\n"))
		flusher.Flush()
	}
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
		Paired:               state.DeviceURL != "" && state.CompanionID != "" && state.Token != "",
		AuthStatus:           firstNonEmpty(authStatus, companionAuthStatusUnknown),
		DeviceURL:            state.DeviceURL,
		CompanionID:          state.CompanionID,
		LocalOnly:            state.LocalOnly,
		Capabilities:         capabilityList,
		BrowserRuntimeStatus: readiness.Status,
		BrowserRuntimeError:  readiness.Error,
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

func defaultBrowserProfilePath() string {
	configurationDirectory, errorValue := os.UserConfigDir()
	if errorValue == nil && strings.TrimSpace(configurationDirectory) != "" {
		return filepath.Join(configurationDirectory, "InternKim", "BrowserProfile")
	}
	homeDirectory, homeError := os.UserHomeDir()
	if homeError == nil && strings.TrimSpace(homeDirectory) != "" {
		return filepath.Join(homeDirectory, ".internkim-companion", "browser-profile")
	}
	return filepath.Join(os.TempDir(), "internkim-companion-browser-profile")
}

func defaultBrowserExecutablePath() string {
	for _, value := range []string{
		os.Getenv("INTERNKIM_BROWSER_EXECUTABLE_PATH"),
		os.Getenv("AGENT_BROWSER_EXECUTABLE_PATH"),
	} {
		if path := executableBrowserPath(value); path != "" {
			return path
		}
	}
	for _, path := range defaultBrowserExecutableCandidates() {
		if executablePath := executableBrowserPath(path); executablePath != "" {
			return executablePath
		}
	}
	return ""
}

func defaultBrowserExecutableCandidates() []string {
	switch runtime.GOOS {
	case "darwin":
		return []string{
			"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome",
			filepath.Join(os.Getenv("HOME"), "Applications", "Google Chrome.app", "Contents", "MacOS", "Google Chrome"),
		}
	case "windows":
		return []string{
			filepath.Join(os.Getenv("PROGRAMFILES"), "Google", "Chrome", "Application", "chrome.exe"),
			filepath.Join(os.Getenv("PROGRAMFILES(X86)"), "Google", "Chrome", "Application", "chrome.exe"),
			filepath.Join(os.Getenv("LOCALAPPDATA"), "Google", "Chrome", "Application", "chrome.exe"),
		}
	default:
		candidates := []string{}
		for _, name := range []string{"google-chrome", "google-chrome-stable"} {
			if path, errorValue := exec.LookPath(name); errorValue == nil {
				candidates = append(candidates, path)
			}
		}
		return candidates
	}
}

func executableBrowserPath(path string) string {
	trimmedPath := strings.TrimSpace(path)
	if trimmedPath == "" {
		return ""
	}
	if isExecutableFile(trimmedPath) {
		return trimmedPath
	}
	return ""
}

// extensionBrowserRuntimeReadiness checks the two local prerequisites for
// browserruntime.ExtensionInputRuntime: a Chrome binary to launch with
// --load-extension, and the unpacked extension directory that flag points
// to. There is no agent-browser doctor/install step to run here (unlike
// AgentBrowserRuntime.EnsureInstalled) since neither piece is a companion-
// managed download.
func extensionBrowserRuntimeReadiness(chromeExecutablePath string, extensionPath string) browserruntime.RuntimeReadiness {
	if !isExecutableFile(chromeExecutablePath) {
		return browserruntime.RuntimeReadiness{Status: "not_ready", Error: "Google Chrome is not installed"}
	}
	if !isDirectory(extensionPath) {
		return browserruntime.RuntimeReadiness{Status: "not_ready", Error: "companion browser extension is not bundled"}
	}
	return browserruntime.RuntimeReadiness{Status: "ready"}
}

// resolveBrowserExtensionPath locates the unpacked companion/browser-extension
// directory. The Tauri shell (companion/src/lib/sidecar.ts) resolves the
// bundled resource path via resolveResource() and always passes it through
// --browser-extension-path, so this fallback chain only matters when the
// companion binary runs outside the Tauri shell (packaged binary invoked
// directly, or a repository-checkout dev run).
func resolveBrowserExtensionPath(flagValue string) string {
	if strings.TrimSpace(flagValue) != "" {
		return strings.TrimSpace(flagValue)
	}
	if environmentValue := strings.TrimSpace(os.Getenv("INTERNKIM_BROWSER_EXTENSION_PATH")); environmentValue != "" {
		return environmentValue
	}
	if bundledPath := bundledBrowserExtensionPath(); bundledPath != "" {
		return bundledPath
	}
	if developmentPath := filepath.Join("companion", "browser-extension"); isDirectory(developmentPath) {
		return developmentPath
	}
	return ""
}

// bundledBrowserExtensionPath checks the packaged-app locations where Tauri's
// bundle.resources places companion/browser-extension: next to the executable
// (Windows NSIS/MSI and Linux AppImage put resources alongside the binary,
// matching how bundledAgentBrowserPath finds sidecars), and one level up under
// Resources/ (the macOS .app bundle keeps Contents/MacOS/<binary> separate
// from Contents/Resources/<resource>).
func bundledBrowserExtensionPath() string {
	executablePath, errorValue := os.Executable()
	if errorValue != nil || strings.TrimSpace(executablePath) == "" {
		return ""
	}
	executableDirectory := filepath.Dir(executablePath)
	candidatePaths := []string{
		filepath.Join(executableDirectory, "browser-extension"),
		filepath.Join(executableDirectory, "..", "Resources", "browser-extension"),
	}
	for _, candidatePath := range candidatePaths {
		if isDirectory(candidatePath) {
			return candidatePath
		}
	}
	return ""
}

func resolveAgentBrowserPath(flagValue string) string {
	if strings.TrimSpace(flagValue) != "" {
		return strings.TrimSpace(flagValue)
	}
	if environmentValue := strings.TrimSpace(os.Getenv("INTERNKIM_AGENT_BROWSER_PATH")); environmentValue != "" {
		return environmentValue
	}
	if bundledPath := bundledAgentBrowserPath(); bundledPath != "" {
		return bundledPath
	}
	if lookupPath, errorValue := exec.LookPath("agent-browser"); errorValue == nil {
		return lookupPath
	}
	return "agent-browser"
}

func bundledAgentBrowserPath() string {
	executablePath, errorValue := os.Executable()
	if errorValue != nil || strings.TrimSpace(executablePath) == "" {
		return ""
	}
	executableDirectory := filepath.Dir(executablePath)
	for _, filename := range bundledAgentBrowserFilenames() {
		path := filepath.Join(executableDirectory, filename)
		if isExecutableFile(path) {
			return path
		}
	}
	return ""
}

func bundledAgentBrowserFilenames() []string {
	names := []string{"agent-browser"}
	switch runtime.GOOS {
	case "darwin":
		if runtime.GOARCH == "arm64" {
			names = append(names, "agent-browser-aarch64-apple-darwin", "agent-browser-darwin-arm64")
		} else {
			names = append(names, "agent-browser-x86_64-apple-darwin", "agent-browser-darwin-x64")
		}
	case "linux":
		if runtime.GOARCH == "arm64" {
			names = append(names, "agent-browser-aarch64-unknown-linux-gnu", "agent-browser-linux-arm64")
		} else {
			names = append(names, "agent-browser-x86_64-unknown-linux-gnu", "agent-browser-linux-x64")
		}
	case "windows":
		names = append(names, "agent-browser.exe", "agent-browser-x86_64-pc-windows-msvc.exe", "agent-browser-win32-x64.exe")
	}
	return names
}

func isExecutableFile(path string) bool {
	information, errorValue := os.Stat(path)
	return errorValue == nil && !information.IsDir() && information.Mode()&0o111 != 0
}

func isDirectory(path string) bool {
	information, errorValue := os.Stat(path)
	return errorValue == nil && information.IsDir()
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
