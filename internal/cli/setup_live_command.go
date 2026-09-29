package cli

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	nostr "github.com/nbd-wtf/go-nostr"

	"gitlab.com/eastriver/internkim/internal/buzzidentity"
	setup "gitlab.com/eastriver/internkim/internal/provisioning/steps"
	"gitlab.com/eastriver/internkim/internal/runtime/locallm"
)

type setupLiveOptions struct {
	target                 commandTarget
	scriptDir              string
	sshpassBin             string
	setupBuildID           string
	requestedSSH           bool
	requestedSD            bool
	requestedCloudflareSSH bool
	nonInteractive         bool
	canRunWithoutSSH       bool
	cloudflareSSHHostname  string
}

type setupLiveRequest struct {
	requestedSSH           bool
	requestedSD            bool
	requestedCloudflareSSH bool
}

type setupBackendSelection struct {
	backend       setup.Backend
	target        commandTarget
	sshConnection *sshClient
	stagingRoot   string
	boardIP       string
}

func runSetupLive(messenger *msg) {
	configuration := loadConfig()
	scriptDir, _ := os.Getwd()
	setupBuildID := currentExecutableFingerprint()
	request := resolveSetupLiveRequest()
	if containsArg("--sim") {
		runSetupSimulation(setupControlArguments(os.Args[2:]))
		return
	}
	options := resolveSetupLiveOptions(configuration, scriptDir, setupBuildID, request)
	releaseSetupLock := acquireSetupLiveLock(options)
	defer releaseSetupLock()
	if runBlueclawPayloadDirectSetup(messenger, configuration, options) {
		return
	}
	backendSelection := resolveSetupBackend(messenger, configuration, options)
	printSetupBackendSelection(backendSelection)
	flowState := prepareSetupFlowState(messenger, configuration, options, backendSelection.sshConnection)
	pipelineContext := prepareSetupPipelineContext(messenger, options, backendSelection, flowState)
	selector := prepareSetupSelector(options.target.boardType, pipelineContext.Force)
	registry := setupRegistryForBoard(options.target.boardType)
	if errorValue := registry.Run(pipelineContext, selector); errorValue != nil {
		fatal(errorValue.Error())
	}
}

func resolveSetupLiveRequest() setupLiveRequest {
	requestedSSH := containsArg("--ssh")
	requestedSD := containsArg("--sd")
	requestedCloudflareSSH := containsArg("--remote-ssh")
	if requestedCloudflareSSH {
		requestedSSH = true
	}
	if requestedSSH && requestedSD {
		fatal("--ssh/--remote-ssh and --sd are mutually exclusive")
	}
	return setupLiveRequest{
		requestedSSH:           requestedSSH,
		requestedSD:            requestedSD,
		requestedCloudflareSSH: requestedCloudflareSSH,
	}
}

func resolveSetupLiveOptions(configuration config, scriptDir string, setupBuildID string, request setupLiveRequest) setupLiveOptions {
	target := resolveCommandTarget(os.Args[2:])
	target = resolveLabHostForCommandTarget(target, scriptDir)
	if target.boardType == setup.BoardJetsonOrinNano {
		request.requestedSSH = true
	}
	return setupLiveOptions{
		target:                 target,
		scriptDir:              scriptDir,
		sshpassBin:             filepath.Join(scriptDir, "bin", "sshpass"),
		setupBuildID:           setupBuildID,
		requestedSSH:           request.requestedSSH,
		requestedSD:            request.requestedSD,
		requestedCloudflareSSH: request.requestedCloudflareSSH,
		nonInteractive:         containsArg("--non-interactive"),
		canRunWithoutSSH:       setupCanRunWithoutSSH(os.Args[2:]),
		cloudflareSSHHostname:  savedRemoteSSHHostname(target),
	}
}

func acquireSetupLiveLock(options setupLiveOptions) func() {
	if containsArg("--plan") {
		return func() {}
	}
	lockDocument := setupLockDocument{
		Command:       commandLineForSetupLock(os.Args[2:]),
		SelectedSteps: setupLockSelectedSteps(os.Args[2:]),
		TargetHost:    firstNonEmptyString(options.target.host, options.cloudflareSSHHostname, loadState(options.target.stateDir, "ssh_hostname")),
		TargetURL:     options.target.deviceURL,
	}
	lockHandle, errorValue := acquireSetupLock(options.target.stateDir, lockDocument, containsArg("--wait-lock"))
	if errorValue != nil {
		fatal(errorValue.Error())
	}
	return lockHandle.release
}

func runBlueclawPayloadDirectSetup(messenger *msg, configuration config, options setupLiveOptions) bool {
	if !isBlueclawPayloadDirectOnlySetup(os.Args[2:]) {
		return false
	}
	target := options.target
	target.useRemoteSSH = false
	printCommandTargetEvidence(target)
	fmt.Printf("Backend: http maintenance\n")
	if containsArg("--plan") {
		fmt.Printf("  blueclaw-payload-direct run\n")
		return true
	}
	flowState := newSetupFlowState(
		messenger,
		configuration,
		collectSetupParameterValues(),
		options.target.stateDir,
		options.scriptDir,
		options.setupBuildID,
		nil,
		options.nonInteractive,
	)
	if errorValue := flowState.installBlueclawPayloadDirectHTTPS(); errorValue != nil {
		fatal(errorValue.Error())
	}
	return true
}

func resolveSetupBackend(messenger *msg, configuration config, options setupLiveOptions) setupBackendSelection {
	if containsArg("--plan") {
		return resolveSetupPlanBackend(options)
	}
	if options.canRunWithoutSSH {
		return resolveSetupWithoutSSHBackend(options)
	}
	if options.requestedSSH {
		return resolveRequestedSetupSSHBackend(messenger, configuration, options)
	}
	if options.requestedSD {
		return resolveRequestedSetupSDBackend(messenger, options)
	}
	return resolveAutomaticSetupBackend(messenger, configuration, options)
}

func resolveSetupPlanBackend(options setupLiveOptions) setupBackendSelection {
	selection := setupBackendSelection{target: options.target}
	if options.requestedSD {
		selection.backend = setup.BackendSD
		selection.stagingRoot = findSDStagingRoot()
		return selection
	}
	selection.backend = setup.BackendSSH
	selection.boardIP = firstNonEmptyString(options.target.host, options.cloudflareSSHHostname, loadState(options.target.stateDir, "ssh_hostname"))
	return selection
}

func resolveSetupWithoutSSHBackend(options setupLiveOptions) setupBackendSelection {
	return setupBackendSelection{
		backend: setup.BackendSSH,
		target:  options.target,
		boardIP: firstNonEmptyString(options.target.host, options.cloudflareSSHHostname, loadState(options.target.stateDir, "ssh_hostname")),
	}
}

func resolveRequestedSetupSSHBackend(messenger *msg, configuration config, options setupLiveOptions) setupBackendSelection {
	var selection setupBackendSelection
	var cloudflareSSHError error
	var sshReady bool
	if options.requestedCloudflareSSH {
		selection, cloudflareSSHError, sshReady = attemptSetupRemoteSSH(options)
	} else if selection, sshReady = attemptSetupBackend(options, setup.BackendSSH); !sshReady {
		selection, cloudflareSSHError, sshReady = attemptSetupRemoteSSH(options)
	}
	if sshReady {
		return selection
	}
	if cloudflareSSHError != nil {
		fatal(cloudflareSSHError.Error())
	}
	if options.target.boardType == setup.BoardJetsonOrinNano {
		fatalJetsonSSHFailure(messenger, options)
	}
	fatal(messenger.t("보드를 찾을 수 없습니다 (SSH).", "Board not reachable (SSH)."))
	return selection
}

func fatalJetsonSSHFailure(messenger *msg, options setupLiveOptions) {
	failureDetails := describeJetsonSSHFailure(options.sshpassBin, options.target.stateDir, options.target.sshUser, options.target.sshPassword)
	fatal(messenger.t(
		"Jetson을 SSH로 찾을 수 없습니다.\n"+failureDetails+"\nJetson 콘솔에서 `ip addr`, `nmcli device status`, `systemctl status ssh --no-pager`, `systemctl status internkim-wifi-recovery.timer --no-pager`, `journalctl -u internkim-wifi-recovery.service -n 80 --no-pager`, `tail /var/log/internkim-jetson-firstboot.log`를 확인하세요. IP를 알면 --host <ip>를 지정하면 됩니다.",
		"Jetson was not found over SSH.\n"+failureDetails+"\nOn the Jetson console, check `ip addr`, `nmcli device status`, `systemctl status ssh --no-pager`, `systemctl status internkim-wifi-recovery.timer --no-pager`, `journalctl -u internkim-wifi-recovery.service -n 80 --no-pager`, and `tail /var/log/internkim-jetson-firstboot.log`. If you know the IP, pass --host <ip>.",
	))
}

func resolveRequestedSetupSDBackend(messenger *msg, options setupLiveOptions) setupBackendSelection {
	selection, ok := attemptSetupBackend(options, setup.BackendSD)
	if ok {
		return selection
	}
	fatal(messenger.t("SD 카드가 꽂혀있지 않거나 internkim 디렉토리가 없습니다.", "No SD card mounted with an internkim staging dir."))
	return selection
}

func resolveAutomaticSetupBackend(messenger *msg, configuration config, options setupLiveOptions) setupBackendSelection {
	if selection, ok := attemptSetupBackend(options, setup.BackendSSH); ok {
		return selection
	}
	selection, cloudflareSSHError, cloudflareSSHReady := attemptSetupRemoteSSH(options)
	if cloudflareSSHReady {
		return selection
	}
	if selection, ok := attemptSetupBackend(options, setup.BackendSD); ok {
		return selection
	}
	if cloudflareSSHError != nil {
		fatal(cloudflareSSHError.Error())
	}
	fatal(messenger.t(
		"타겟을 찾을 수 없습니다 — 보드에 SSH도 안 되고, SD 카드도 없습니다.\n  --ssh 또는 --sd 를 명시하거나, 대상을 준비해 주세요.",
		"No target — board unreachable via SSH and no SD mounted.\n  Pass --ssh or --sd explicitly, or prepare a target.",
	))
	return setupBackendSelection{target: options.target}
}

func attemptSetupBackend(options setupLiveOptions, backend setup.Backend) (setupBackendSelection, bool) {
	selection := setupBackendSelection{backend: backend, target: options.target}
	return selection, setupBackendIsReady(options, &selection)
}

func setupBackendIsReady(options setupLiveOptions, selection *setupBackendSelection) bool {
	switch selection.backend {
	case setup.BackendSSH:
		return setupSSHBackendIsReady(options, selection)
	case setup.BackendSD:
		selection.stagingRoot = findSDStagingRoot()
		return selection.stagingRoot != ""
	default:
		return false
	}
}

func setupSSHBackendIsReady(options setupLiveOptions, selection *setupBackendSelection) bool {
	if options.target.host != "" {
		selection.boardIP = options.target.host
		candidateConnection := newSSH(options.sshpassBin, options.target.sshUser, options.target.sshPassword, selection.boardIP)
		if _, errorValue := candidateConnection.runResult("true"); errorValue != nil {
			return false
		}
		selection.sshConnection = candidateConnection
		return true
	}
	selection.boardIP = findBoardIPForCredentials(options.sshpassBin, options.target.stateDir, options.target.sshUser, options.target.sshPassword)
	if selection.boardIP == "" {
		return false
	}
	selection.sshConnection = newSSH(options.sshpassBin, options.target.sshUser, options.target.sshPassword, selection.boardIP)
	return true
}

func attemptSetupRemoteSSH(options setupLiveOptions) (setupBackendSelection, error, bool) {
	selection := setupBackendSelection{backend: setup.BackendSSH, target: options.target}
	remoteSSHHostname := options.cloudflareSSHHostname
	if remoteSSHHostname == "" {
		return selection, nil, false
	}
	selection.boardIP = remoteSSHHostname
	selection.sshConnection = newSSH(options.sshpassBin, options.target.sshUser, options.target.sshPassword, selection.boardIP)
	output, errorValue := selection.sshConnection.runResult("true")
	if errorValue != nil {
		return selection, remoteSSHError(selection.boardIP, output, errorValue), false
	}
	selection.target.useRemoteSSH = true
	return selection, nil, true
}

func printSetupBackendSelection(selection setupBackendSelection) {
	target := selection.target
	switch selection.backend {
	case setup.BackendSSH:
		target.host = selection.boardIP
		printCommandTargetEvidence(target)
		fmt.Printf("Backend: ssh\n")
	case setup.BackendSD:
		printCommandTargetEvidence(target)
		fmt.Printf("Backend: sd staging\n")
		fmt.Printf("Staging: %s\n", selection.stagingRoot)
	}
}

func prepareSetupFlowState(messenger *msg, configuration config, options setupLiveOptions, sshConnection *sshClient) *setupFlowState {
	return newSetupFlowState(
		messenger,
		configuration,
		collectSetupParameterValues(),
		options.target.stateDir,
		options.scriptDir,
		options.setupBuildID,
		sshConnection,
		options.nonInteractive,
	)
}

func prepareSetupPipelineContext(messenger *msg, options setupLiveOptions, selection setupBackendSelection, flowState *setupFlowState) *setup.Context {
	pipelineContext := &setup.Context{
		Backend:      selection.backend,
		Language:     messenger.lang,
		StateDir:     options.target.stateDir,
		ScriptDir:    options.scriptDir,
		BoardType:    options.target.boardType,
		BoardIP:      selection.boardIP,
		PublicURL:    options.target.deviceURL,
		RelayDomain:  relayDomainForTarget(options.target.stateDir),
		SetupCommand: commandLineForSetupLock(os.Args[2:]),
		SetupSteps:   setupLockSelectedSteps(os.Args[2:]),
		SetupLockID:  randomHexString(12),
		Force:        resolveSetupForce(options, selection),
		HTTP:         &http.Client{Timeout: 30 * time.Second},
		Callbacks:    flowState.callbacks(),
	}
	if selection.backend == setup.BackendSSH {
		if selection.sshConnection != nil {
			pipelineContext.SSH = sshBoardConnection{client: selection.sshConnection}
		}
		return pipelineContext
	}
	pipelineContext.SD = sdStagingTarget{stagingRoot: selection.stagingRoot}
	return pipelineContext
}

func resolveSetupForce(options setupLiveOptions, selection setupBackendSelection) bool {
	shouldForce := containsArg("--force") || containsArg("--force-all")
	if selection.backend != setup.BackendSD || options.setupBuildID == "" {
		return shouldForce
	}
	stageBuildIDPath := filepath.Join(selection.stagingRoot, "setup-build-id")
	stageBuildIDBytes, errorValue := os.ReadFile(stageBuildIDPath)
	if errorValue == nil && strings.TrimSpace(string(stageBuildIDBytes)) != options.setupBuildID {
		return true
	}
	return shouldForce
}

func prepareSetupSelector(boardType string, shouldForce bool) setup.Selector {
	selector := setup.Selector{
		Only:     setup.ParseNames(argString("--only", "")),
		From:     argString("--from", ""),
		Skip:     setup.ParseNames(argString("--skip", "")),
		Force:    shouldForce,
		ForceAll: containsArg("--force-all"),
		DryRun:   containsArg("--plan"),
	}
	return applySetupBoardDefaults(boardType, selector)
}

func setupRegistryForBoard(boardType string) setup.Registry {
	if boardType == setup.BoardJetsonOrinNano {
		return setup.JetsonRegistry()
	}
	return setup.DefaultRegistry()
}

func applySetupBoardDefaults(boardType string, selector setup.Selector) setup.Selector {
	if boardType == setup.BoardCloudShared {
		for _, name := range []string{"wifi", "local-llm"} {
			if !containsName(selector.Only, name) {
				selector.Skip = appendMissingName(selector.Skip, name)
			}
		}
	}
	return selector
}

func collectSetupParameterValues() setupParameterValues {
	return setupParameterValues{
		AdminEmail:       strings.TrimSpace(argString("--admin-email", "")),
		OpenRouterAPIKey: strings.TrimSpace(argString("--openrouter-api-key", "")),
		LiteRTModelPath:  strings.TrimSpace(argString("--litert-model-path", "")),
	}
}

func buildOpenRouterKeyCallback(messenger *msg, openRouterAPIKey string, nonInteractive bool) func(force bool) (string, error) {
	return func(force bool) (string, error) {
		if openRouterAPIKey != "" {
			return openRouterAPIKey, nil
		}
		if envKey := strings.TrimSpace(os.Getenv("OPENROUTER_API_KEY")); envKey != "" {
			return envKey, nil
		}
		if nonInteractive {
			return "", fmt.Errorf("openrouter API key is empty; pass --openrouter-api-key, or keep OPENROUTER_API_KEY in the vault")
		}
		promptedKey := strings.TrimSpace(readLine(messenger.t("  OpenRouter API 키: ", "  OpenRouter API key: ")))
		if promptedKey == "" {
			return "", nil
		}
		if errorValue := rememberInVault("OPENROUTER_API_KEY", promptedKey); errorValue != nil {
			fmt.Printf("  Warning: %v; the key serves this run only\n", errorValue)
		}
		return promptedKey, nil
	}
}

func buildBuzzKeySeedCallback() func(force bool) (string, error) {
	return func(force bool) (string, error) {
		_ = force
		return strings.TrimSpace(os.Getenv("INTERNKIM_BUZZ_KEY_SEED")), nil
	}
}

// chatd signs what the agent says, so it holds the agent's identity rather than
// the one that owns the relay.
func buildBuzzAgentSecretCallback() func() (string, error) {
	return func() (string, error) {
		seed, errorValue := buildBuzzKeySeedCallback()(false)
		if errorValue != nil {
			return "", errorValue
		}
		if strings.TrimSpace(seed) == "" {
			return "", errors.New("buzz key seed unavailable; cannot derive the agent secret")
		}
		return buzzidentity.Secret(seed, buzzidentity.AgentSubject), nil
	}
}

func buildBuzzRelayOwnerPubkeyCallback() func() (string, error) {
	return func() (string, error) {
		seed, errorValue := buildBuzzKeySeedCallback()(false)
		if errorValue != nil {
			return "", errorValue
		}
		if strings.TrimSpace(seed) == "" {
			return "", errors.New("buzz key seed unavailable; cannot derive relay owner pubkey")
		}
		return nostr.GetPublicKey(buzzidentity.Secret(seed, buzzidentity.BootstrapSubject))
	}
}

func buildLiteRTModelPathCallback(liteRTModelPath string) func(force bool) (string, error) {
	return func(force bool) (string, error) {
		_ = force
		if liteRTModelPath != "" {
			return liteRTModelPath, nil
		}
		if envPath := strings.TrimSpace(os.Getenv("INTERNKIM_LITERT_MODEL_PATH")); envPath != "" {
			return envPath, nil
		}
		return ensureCachedLiteRTModel()
	}
}

func ensureCachedLiteRTModel() (string, error) {
	repositoryRoot := repositoryRootForDependencyCache()
	if repositoryRoot == "" {
		return "", nil
	}
	cacheRelative := locallm.LiteRTCacheModelDir
	filename := locallm.LiteRTModelFilename
	sourceURL := locallm.LiteRTModelURL
	displayName := "LiteRT"
	if locallm.Default == locallm.BackendLlamaCpp {
		cacheRelative = locallm.LlamaCppCacheModelDir
		filename = locallm.LlamaCppModelFilename
		sourceURL = locallm.LlamaCppModelURL
		displayName = "llama.cpp"
	}
	cacheDirectory := filepath.Join(repositoryRoot, cacheRelative)
	cachedModelPath := filepath.Join(cacheDirectory, filename)
	if fileInfo, errorValue := os.Stat(cachedModelPath); errorValue == nil && fileInfo.Size() > 0 {
		return cachedModelPath, nil
	}
	if errorValue := os.MkdirAll(cacheDirectory, 0o755); errorValue != nil {
		return "", errorValue
	}
	temporaryPath := cachedModelPath + ".tmp"
	_ = os.Remove(temporaryPath)
	fmt.Printf("  downloading %s model into %s\n", displayName, cachedModelPath)
	command := exec.Command("curl", "-fL", "--retry", "3", "--progress-bar", "-o", temporaryPath, sourceURL)
	command.Stdout = os.Stdout
	command.Stderr = os.Stderr
	if errorValue := command.Run(); errorValue != nil {
		_ = os.Remove(temporaryPath)
		return "", errorValue
	}
	if errorValue := os.Rename(temporaryPath, cachedModelPath); errorValue != nil {
		return "", errorValue
	}
	return cachedModelPath, nil
}
