package blueclaw

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"gitlab.com/eastriver/internkim/internal/capabilities"
	"gitlab.com/eastriver/internkim/internal/runtime/locallm"
	"gitlab.com/eastriver/internkim/pkg/capabilityprotocol"
)

func TestCapabilityContractUsesCurrentDefinitions(t *testing.T) {
	contract := CurrentCapabilityContract()
	if contract.Version != 3 {
		t.Fatalf("contract version = %d, want 3", contract.Version)
	}
	if contract.ProtocolIdentity != capabilityprotocol.GeneratedProtocolIdentity() {
		t.Fatalf("contract protocol identity does not match generated protocol")
	}
	if !reflect.DeepEqual(contract.ToolDescriptors, capabilities.DefaultToolDescriptors()) {
		t.Fatalf("contract tool descriptors do not match current capabilities")
	}
	if !reflect.DeepEqual(contract.RoutingCandidates, capabilities.RoutingCandidates()) {
		t.Fatalf("contract routing candidates do not match current capabilities")
	}
	if !reflect.DeepEqual(contract.PolicyResourceDefaults, defaultResourceAccessPolicies()) {
		t.Fatalf("contract policy defaults do not match current Blueclaw policy")
	}
	for legacyToolName, currentToolName := range capabilities.LegacyToolNameReplacements() {
		legacyResource := "tool:" + legacyToolName
		currentResource := "tool:" + currentToolName
		if contract.PolicyResourceReplacements[legacyResource] != currentResource {
			t.Fatalf("policy replacement for %q = %q, want %q", legacyResource, contract.PolicyResourceReplacements[legacyResource], currentResource)
		}
	}
}

func TestCapabilityContractDocumentIsDeterministic(t *testing.T) {
	firstDocument, errorValue := CapabilityContractDocument()
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	secondDocument, errorValue := CapabilityContractDocument()
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if firstDocument != secondDocument {
		t.Fatal("expected deterministic capability contract document")
	}
}

func TestBlueclawRuntimeConfigDirectExecutionUsesNativeUnixSocketRuntime(t *testing.T) {
	document, errorValue := BlueclawRuntimeConfigDocumentWithOptions(RuntimeConfigOptions{
		DirectExecution:          true,
		CapabilitySocketPath:     "/run/internkim/capability.sock",
		DatabaseConnectionString: "postgres://internkim@postgres/tenant_01?sslmode=disable",
		WorkspaceRootPath:        "/workspace",
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}

	var runtimeConfiguration map[string]any
	if errorValue := json.Unmarshal([]byte(document), &runtimeConfiguration); errorValue != nil {
		t.Fatal(errorValue)
	}

	capabilityConfiguration := runtimeConfiguration["capabilities"].(map[string]any)
	expectedIdentity := capabilityprotocol.GeneratedProtocolIdentity()
	if capabilityConfiguration["protocolVersion"] != expectedIdentity.ProtocolVersion {
		t.Fatalf("unexpected capability protocol version: %+v", capabilityConfiguration)
	}
	if capabilityConfiguration["aggregateProtocolHash"] != expectedIdentity.AggregateProtocolHash {
		t.Fatalf("unexpected capability aggregate hash: %+v", capabilityConfiguration)
	}
	if capabilityConfiguration["transport"] != "" {
		t.Fatalf("expected empty transport for direct execution, got %q", capabilityConfiguration["transport"])
	}
	if capabilityConfiguration["unixSocketPath"] != "/run/internkim/capability.sock" {
		t.Fatalf("expected unix socket transport, got %q", capabilityConfiguration["unixSocketPath"])
	}

	terminalConfiguration := runtimeConfiguration["terminal"].(map[string]any)
	if terminalConfiguration["mode"] != "native" {
		t.Fatalf("expected native terminal mode, got %q", terminalConfiguration["mode"])
	}
	if terminalConfiguration["posixHelperPath"] != "" {
		t.Fatalf("expected POSIX synchronization skipped, got %q", terminalConfiguration["posixHelperPath"])
	}

	languageModel := runtimeConfiguration["languageModel"].(map[string]any)
	if languageModel["defaultProvider"] != "capabilityLLM" {
		t.Fatalf("expected direct execution to reach the model through capabilityd, got %+v", languageModel)
	}
	if _, namesLLMD := languageModel["llmd"]; namesLLMD {
		t.Fatalf("expected no llmd endpoint, because naming one makes blueclaw check a bridge that is gone: %+v", languageModel)
	}
	capabilityLanguageModel := languageModel["capability"].(map[string]any)
	if capabilityLanguageModel["executionMode"] != "remote" {
		t.Fatalf("expected remote inference for direct execution, got %q", capabilityLanguageModel["executionMode"])
	}

	memoryConfiguration := runtimeConfiguration["memory"].(map[string]any)
	if memoryConfiguration["graphitiEndpoint"] != "" {
		t.Fatalf("expected graphiti disabled for direct execution, got %q", memoryConfiguration["graphitiEndpoint"])
	}

	databaseConfiguration := runtimeConfiguration["database"].(map[string]any)
	if databaseConfiguration["connectionString"] != "postgres://internkim@postgres/tenant_01?sslmode=disable" {
		t.Fatalf("expected tenant database connection string, got %q", databaseConfiguration["connectionString"])
	}
}

func TestBlueclawRuntimeConfigUsesCapabilityBoundary(t *testing.T) {
	t.Setenv(LocalOnlyEnvironment, "")
	document, errorValue := BlueclawRuntimeConfigDocument("")
	if errorValue != nil {
		t.Fatal(errorValue)
	}

	var runtimeConfiguration map[string]any
	if errorValue := json.Unmarshal([]byte(document), &runtimeConfiguration); errorValue != nil {
		t.Fatal(errorValue)
	}

	languageModel := runtimeConfiguration["languageModel"].(map[string]any)
	if languageModel["defaultProvider"] != "capabilityLLM" {
		t.Fatalf("expected capability default provider, got %q", languageModel["defaultProvider"])
	}
	capabilityConfiguration := runtimeConfiguration["capabilities"].(map[string]any)
	if capabilityConfiguration["transport"] != "vsock" {
		t.Fatalf("expected capability vsock transport, got %q", capabilityConfiguration["transport"])
	}
	if capabilityConfiguration["timeoutSecond"] != float64(BlueclawCapabilityTimeoutSecond) {
		t.Fatalf("expected capability timeout %d, got %v", BlueclawCapabilityTimeoutSecond, capabilityConfiguration["timeoutSecond"])
	}
	if capabilityConfiguration["unixSocketPath"] != "" {
		t.Fatalf("expected capability unix socket path to be omitted for guest runtime, got %q", capabilityConfiguration["unixSocketPath"])
	}
	if _, hasToolNames := capabilityConfiguration["toolNames"]; hasToolNames {
		t.Fatalf("expected descriptor-only capability configuration, got %+v", capabilityConfiguration)
	}
	capabilityToolDescriptors := capabilityConfiguration["toolDescriptors"].([]any)
	if !containsDescriptor(capabilityToolDescriptors, "browser_open", "inputSchema") {
		t.Fatalf("expected browser_open descriptor with input schema, got %+v", capabilityToolDescriptors)
	}
	if containsDescriptor(capabilityToolDescriptors, "user_confirm", "requiresApproval") {
		t.Fatalf("expected user_confirm to avoid recursive approval, got %+v", capabilityToolDescriptors)
	}
	if !containsCompletionEvidence(capabilityToolDescriptors, "message_send", "success", "send_message", "message") {
		t.Fatalf("expected platform message send descriptor to preserve completion evidence, got %+v", capabilityToolDescriptors)
	}
	if !containsCompletionEvidence(capabilityToolDescriptors, "mail_message_send", "success", "send_email", "email") {
		t.Fatalf("expected mail send descriptor to preserve completion evidence, got %+v", capabilityToolDescriptors)
	}
	if !containsResultContract(capabilityToolDescriptors, "task_add", "taskID", "task", "created") {
		t.Fatalf("expected task_add canonical result contract, got %+v", capabilityToolDescriptors)
	}
	routing := capabilityConfiguration["routing"].(map[string]any)
	if routing["localOnly"] != false {
		t.Fatalf("expected default routing to allow remote fallback, got %v", routing["localOnly"])
	}
	capabilityLanguageModel := languageModel["capability"].(map[string]any)
	if capabilityLanguageModel["executionMode"] != "auto" {
		t.Fatalf("expected automatic execution mode, got %q", capabilityLanguageModel["executionMode"])
	}
	if capabilityLanguageModel["model"] != BlueclawDefaultModelName {
		t.Fatalf("expected default runtime model %q, got %+v", BlueclawDefaultModelName, capabilityLanguageModel)
	}
	for sectionName, section := range map[string]map[string]any{
		"languageModel":            languageModel,
		"languageModel.capability": capabilityLanguageModel,
		"capabilities":             capabilityConfiguration,
	} {
		if _, hasBackend := section["backend"]; hasBackend {
			t.Fatalf("expected %s to omit backend selection, got %+v", sectionName, section)
		}
	}
	if _, hasHighModel := capabilityLanguageModel["highModel"]; hasHighModel {
		t.Fatalf("expected no per-tier highModel in production config; tier defaults are owned by blueclaw, got %+v", capabilityLanguageModel)
	}
	if capabilityLanguageModel["contextWindowTokens"] != float64(BlueclawDefaultModelContextTokens) {
		t.Fatalf("expected default runtime context window %d, got %+v", BlueclawDefaultModelContextTokens, capabilityLanguageModel)
	}
	agentConfiguration := runtimeConfiguration["agent"].(map[string]any)
	if _, isFound := agentConfiguration["generationOptions"]; isFound {
		t.Fatal("expected generation options to be omitted by default")
	}
	if _, isFound := languageModel["openRouter"]; isFound {
		t.Fatal("expected OpenRouter runtime details to be omitted")
	}
	if _, isFound := languageModel["liteRTLM"]; isFound {
		t.Fatal("expected LiteRT runtime details to be omitted")
	}
	memory := runtimeConfiguration["memory"].(map[string]any)
	if memory["graphitiEndpoint"] != GraphitiEndpoint {
		t.Fatalf("expected Graphiti endpoint, got %q", memory["graphitiEndpoint"])
	}
	if memory["graphitiKuzuPath"] != "/workspace/.blueclaw/graphiti/kuzu" {
		t.Fatalf("expected Graphiti Kuzu path, got %q", memory["graphitiKuzuPath"])
	}
	if memory["pinnedMemoryRootPath"] != "/workspace/.blueclaw/memory" {
		t.Fatalf("expected pinned memory path, got %q", memory["pinnedMemoryRootPath"])
	}
	if memory["pinnedMemoryHardLimitCharacterCount"] != float64(6000) {
		t.Fatalf("expected pinned memory hard limit, got %v", memory["pinnedMemoryHardLimitCharacterCount"])
	}
	if memory["pinnedMemoryCompressionTargetCharacterCount"] != float64(3500) {
		t.Fatalf("expected pinned memory compression target, got %v", memory["pinnedMemoryCompressionTargetCharacterCount"])
	}
	if memory["timeoutSecond"] != float64(60) {
		t.Fatalf("expected Graphiti timeout, got %v", memory["timeoutSecond"])
	}
	agent := runtimeConfiguration["agent"].(map[string]any)
	intake := agent["intake"].(map[string]any)
	if intake["enabled"] != true {
		t.Fatalf("expected agent intake enabled, got %v", intake["enabled"])
	}
	if _, isFound := intake["model"]; isFound {
		t.Fatalf("expected agent intake to omit model override, got %+v", intake)
	}
	if intake["executionMode"] != "auto" {
		t.Fatalf("expected agent intake execution mode, got %v", intake["executionMode"])
	}
	if agent["defaultTaskLevel"] != "low" {
		t.Fatalf("expected default task level, got %v", agent["defaultTaskLevel"])
	}
	if agent["skillTaskLevelFloor"] != "high" {
		t.Fatalf("expected high skill task level floor, got %v", agent["skillTaskLevelFloor"])
	}
	if agent["toolResultMaxBytes"] != float64(32768) {
		t.Fatalf("expected agent tool result limit, got %v", agent["toolResultMaxBytes"])
	}
	database := runtimeConfiguration["database"].(map[string]any)
	if database["connectionString"] != BlueclawGuestDatabaseConnectionString {
		t.Fatalf("expected keyword-value Unix socket database connection string, got %q", database["connectionString"])
	}
	failureRecovery := agent["failureRecovery"].(map[string]any)
	if failureRecovery["failureDebtFinalizationGate"] != true {
		t.Fatalf("expected failure debt finalization gate, got %+v", failureRecovery)
	}
	if failureRecovery["attemptFingerprint"] != "tool_input_error_code" {
		t.Fatalf("expected tool input error fingerprint mode, got %+v", failureRecovery)
	}
	recoveryBudget := failureRecovery["recoveryBudget"].(map[string]any)
	expectedRecoveryBudget := map[string]float64{
		"correctedRetry": 1,
		"alternateRoute": 1,
		"adjacentTool":   2,
		"noToolFallback": 1,
	}
	for key, expectedValue := range expectedRecoveryBudget {
		if recoveryBudget[key] != expectedValue {
			t.Fatalf("expected recovery budget %s=%v, got %+v", key, expectedValue, recoveryBudget)
		}
	}
	if runtimeConfiguration["agentProfiles"] != nil {
		t.Fatalf("expected Blueclaw to own its default tool profile, got %+v", runtimeConfiguration["agentProfiles"])
	}
	capabilityConfiguration = runtimeConfiguration["capabilities"].(map[string]any)
	if _, hasToolNames := capabilityConfiguration["toolNames"]; hasToolNames {
		t.Fatalf("expected descriptor-only capability configuration, got %+v", capabilityConfiguration)
	}
	terminal := runtimeConfiguration["terminal"].(map[string]any)
	if terminal["mode"] != "virtualMachineGuest" {
		t.Fatalf("expected virtual machine guest terminal mode, got %q", terminal["mode"])
	}
	if terminal["posixHelperPath"] != BlueclawPOSIXHelperPath {
		t.Fatalf("expected POSIX helper path, got %q", terminal["posixHelperPath"])
	}
	if _, hasAllowlist := terminal["allowedExecutableNames"]; hasAllowlist {
		t.Fatalf("expected no executable allowlist; POSIX permissions are the execution boundary, got %+v", terminal["allowedExecutableNames"])
	}
	if _, hasPathDenylist := terminal["deniedPathPrefixes"]; hasPathDenylist {
		t.Fatalf("expected no path denylist; POSIX permissions are the path boundary, got %+v", terminal["deniedPathPrefixes"])
	}
	requesterWorkspace := terminal["requesterWorkspace"].(map[string]any)
	if requesterWorkspace["taskTemporaryEnvironmentVariable"] != "BLUECLAW_TASK_TMP" {
		t.Fatalf("expected requester task temporary environment contract, got %+v", requesterWorkspace)
	}
	if requesterWorkspace["taskTemporaryDirectoryTemplate"] != "/workspace/private/people/{personID}/tmp/{taskID}" {
		t.Fatalf("expected requester task temporary directory template, got %+v", requesterWorkspace)
	}
	if requesterWorkspace["requesterArtifactsEnvironmentVariable"] != "BLUECLAW_REQUESTER_ARTIFACTS" {
		t.Fatalf("expected requester artifacts environment contract, got %+v", requesterWorkspace)
	}
	if terminal["outputMaxBytes"] != float64(32768) || terminal["sessionMaxCount"] != float64(4) {
		t.Fatalf("expected terminal caps, got %+v", terminal)
	}
	guest := runtimeConfiguration["guest"].(map[string]any)
	if guest["vcpuCount"] != float64(BlueclawGuestDefaultVirtualCPUCount) || guest["memoryMiB"] != float64(BlueclawGuestDefaultMemoryMiB) {
		t.Fatalf("expected bounded guest resources, got %+v", guest)
	}
	outboundNetwork := guest["outboundNetwork"].(map[string]any)
	if outboundNetwork["enabled"] != true {
		t.Fatalf("expected outbound network enabled, got %+v", outboundNetwork)
	}
	if outboundNetwork["hostDeviceName"] != "bctap0" || outboundNetwork["networkCIDR"] != "172.31.0.0/30" {
		t.Fatalf("expected deterministic outbound network, got %+v", outboundNetwork)
	}
	if _, isFound := terminal["commandRewrite"]; isFound {
		t.Fatalf("expected RTK hook not to be exposed as runtime config, got %+v", terminal)
	}
	connectors := runtimeConfiguration["connectors"].(map[string]any)
	mattermost := connectors["mattermost"].(map[string]any)
	if _, isFound := mattermost["botTokenPath"]; isFound {
		t.Fatal("expected Mattermost bot token path to be omitted")
	}
	forbiddenFragments := []string{"apiKeyPath", "botTokenPath", "signingSecretPath", "wrapperPath", "modelPath"}
	for _, fragment := range forbiddenFragments {
		if strings.Contains(document, fragment) {
			t.Fatalf("expected runtime config to omit %q", fragment)
		}
	}
	for _, fragment := range []string{"maxWallClockSecond", "maxIterationsPerRequest", "maxToolCallsPerRequest"} {
		if strings.Contains(document, fragment) {
			t.Fatalf("expected runtime config to omit raw limit field %q", fragment)
		}
	}
}

func TestBlueclawRuntimeConfigCanIncludeGenerationOptions(t *testing.T) {
	seed := int64(41)
	temperature := 0.0
	document, errorValue := BlueclawRuntimeConfigDocumentWithOptions(RuntimeConfigOptions{
		GenerationSeed:        &seed,
		GenerationTemperature: &temperature,
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}

	var runtimeConfiguration map[string]any
	if errorValue := json.Unmarshal([]byte(document), &runtimeConfiguration); errorValue != nil {
		t.Fatal(errorValue)
	}

	agentConfiguration := runtimeConfiguration["agent"].(map[string]any)
	generationOptions := agentConfiguration["generationOptions"].(map[string]any)
	if generationOptions["seed"] != float64(seed) {
		t.Fatalf("expected generation seed %d, got %+v", seed, generationOptions)
	}
	if generationOptions["temperature"] != temperature {
		t.Fatalf("expected generation temperature %v, got %+v", temperature, generationOptions)
	}
}

func TestBlueclawRuntimeConfigOptionsCanLoadGenerationOptionsFromEnvironment(t *testing.T) {
	t.Setenv(BlueclawTestModelEnvironment, "google/test-model")
	t.Setenv(BlueclawTestModelTierEnvironment, "xlow")
	t.Setenv(BlueclawTestMaximumModelTierEnvironment, "xlow")
	t.Setenv(BlueclawTestGenerationSeedEnvironment, "41")
	t.Setenv(BlueclawTestGenerationTemperatureEnvironment, "0")
	t.Setenv(BlueclawAdminTaskDiagnosticEnvironment, "true")

	options, errorValue := BlueclawRuntimeConfigOptionsFromEnvironment()
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if options.ModelName != "google/test-model" {
		t.Fatalf("expected model from environment, got %+v", options)
	}
	if !options.ShouldUseModelForAllTiers {
		t.Fatalf("expected environment model to apply to all model tiers, got %+v", options)
	}
	if options.DefaultTaskLevel != "xlow" {
		t.Fatalf("expected task level from environment, got %+v", options)
	}
	if options.MaximumModelTier != "xlow" {
		t.Fatalf("expected maximum model tier from environment, got %+v", options)
	}
	if options.GenerationSeed == nil || *options.GenerationSeed != 41 {
		t.Fatalf("expected seed from environment, got %+v", options)
	}
	if options.GenerationTemperature == nil || *options.GenerationTemperature != 0 {
		t.Fatalf("expected temperature from environment, got %+v", options)
	}
	if !options.AllowAdminTaskDiagnostic {
		t.Fatalf("expected admin task diagnostic from environment, got %+v", options)
	}
}

func TestBlueclawRuntimeConfigRejectsInvalidAdminTaskDiagnosticEnvironment(t *testing.T) {
	t.Setenv(BlueclawAdminTaskDiagnosticEnvironment, "invalid")

	_, errorValue := BlueclawRuntimeConfigOptionsFromEnvironment()
	if errorValue == nil || !strings.Contains(errorValue.Error(), BlueclawAdminTaskDiagnosticEnvironment) {
		t.Fatalf("expected diagnostic environment error, got %v", errorValue)
	}
}

func TestBlueclawRuntimeConfigGatesAdminTaskDiagnostic(t *testing.T) {
	document, errorValue := BlueclawRuntimeConfigDocumentWithOptions(RuntimeConfigOptions{AllowAdminTaskDiagnostic: true})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	var runtimeConfiguration map[string]any
	if errorValue := json.Unmarshal([]byte(document), &runtimeConfiguration); errorValue != nil {
		t.Fatal(errorValue)
	}
	agentConfiguration := runtimeConfiguration["agent"].(map[string]any)
	if agentConfiguration["allowAdminTaskDiagnostic"] != true {
		t.Fatalf("expected admin task diagnostic gate, got %+v", agentConfiguration)
	}
	agentProfiles := runtimeConfiguration["agentProfiles"].([]any)
	if len(agentProfiles) != 1 {
		t.Fatalf("expected diagnostic profile, got %+v", agentProfiles)
	}
	diagnosticProfile := agentProfiles[0].(map[string]any)
	if diagnosticProfile["name"] != BlueclawModelPathDiagnosticProfileName {
		t.Fatalf("expected model-path diagnostic profile, got %+v", diagnosticProfile)
	}
	allowedToolNames := diagnosticProfile["allowedToolNames"].([]any)
	if len(allowedToolNames) != 1 || allowedToolNames[0] != BlueclawModelPathDiagnosticToolSentinel {
		t.Fatalf("expected diagnostic deny-all sentinel, got %+v", diagnosticProfile)
	}
}

func TestInvalidLocalOnlyEnvironmentFailsClosed(t *testing.T) {
	t.Setenv(LocalOnlyEnvironment, "invalid")

	if !LocalOnlyEnabled() {
		t.Fatal("expected invalid local-only environment to disable remote routing")
	}
	if serviceDocument := CapabilitydServiceUnit(); !strings.Contains(serviceDocument, "--local-only") {
		t.Fatalf("expected invalid local-only environment to fail closed, got %s", serviceDocument)
	}
}

func TestLocalOnlyEnvironmentConfiguresRuntimeAndServices(t *testing.T) {
	t.Setenv(LocalOnlyEnvironment, "true")

	options, errorValue := BlueclawRuntimeConfigOptionsFromEnvironment()
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	document, errorValue := BlueclawRuntimeConfigDocumentWithOptions(options)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	var runtimeConfiguration map[string]any
	if errorValue := json.Unmarshal([]byte(document), &runtimeConfiguration); errorValue != nil {
		t.Fatal(errorValue)
	}
	capabilityConfiguration := runtimeConfiguration["capabilities"].(map[string]any)
	routing := capabilityConfiguration["routing"].(map[string]any)
	if routing["localOnly"] != true {
		t.Fatalf("expected local-only capability routing, got %+v", routing)
	}
	if !strings.Contains(CapabilitydServiceUnit(), " --local-only") {
		t.Fatalf("expected capabilityd local-only flag, got %s", CapabilitydServiceUnit())
	}
}

func TestBlueclawRuntimeConfigUsesRequestedDefaultTaskLevel(t *testing.T) {
	document, errorValue := BlueclawRuntimeConfigDocumentWithOptions(RuntimeConfigOptions{DefaultTaskLevel: "xlow"})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	var runtimeConfiguration map[string]any
	if errorValue := json.Unmarshal([]byte(document), &runtimeConfiguration); errorValue != nil {
		t.Fatal(errorValue)
	}
	agentConfiguration := runtimeConfiguration["agent"].(map[string]any)
	if agentConfiguration["defaultTaskLevel"] != "xlow" {
		t.Fatalf("expected xlow default task level, got %+v", agentConfiguration)
	}
}

func TestBlueclawRuntimeConfigIncludesMaximumModelTier(t *testing.T) {
	document, errorValue := BlueclawRuntimeConfigDocumentWithOptions(RuntimeConfigOptions{MaximumModelTier: "low"})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	var runtimeConfiguration map[string]any
	if errorValue := json.Unmarshal([]byte(document), &runtimeConfiguration); errorValue != nil {
		t.Fatal(errorValue)
	}
	languageModel := runtimeConfiguration["languageModel"].(map[string]any)
	capabilityLanguageModel := languageModel["capability"].(map[string]any)
	if capabilityLanguageModel["maximumModelTier"] != "low" {
		t.Fatalf("expected low maximum model tier, got %+v", capabilityLanguageModel)
	}
}

func TestBlueclawRuntimeConfigRejectsInvalidMaximumModelTierEnvironment(t *testing.T) {
	t.Setenv(BlueclawTestMaximumModelTierEnvironment, "coding")
	_, errorValue := BlueclawRuntimeConfigOptionsFromEnvironment()
	if errorValue == nil || !strings.Contains(errorValue.Error(), "maximum model tier") {
		t.Fatalf("expected maximum model tier error, got %v", errorValue)
	}
}

func TestBlueclawRuntimeConfigOptionsRejectInvalidGenerationEnvironment(t *testing.T) {
	t.Setenv(BlueclawTestGenerationSeedEnvironment, "not-a-seed")

	_, errorValue := BlueclawRuntimeConfigOptionsFromEnvironment()
	if errorValue == nil || !strings.Contains(errorValue.Error(), BlueclawTestGenerationSeedEnvironment) {
		t.Fatalf("expected seed environment error, got %v", errorValue)
	}
}

func TestBlueclawRuntimeConfigSupportsOptionalModelOverride(t *testing.T) {
	document, errorValue := BlueclawRuntimeConfigDocument("google/custom-model")
	if errorValue != nil {
		t.Fatal(errorValue)
	}

	var runtimeConfiguration map[string]any
	if errorValue := json.Unmarshal([]byte(document), &runtimeConfiguration); errorValue != nil {
		t.Fatal(errorValue)
	}

	languageModel := runtimeConfiguration["languageModel"].(map[string]any)
	capabilityLanguageModel := languageModel["capability"].(map[string]any)
	if capabilityLanguageModel["model"] != "google/custom-model" {
		t.Fatalf("expected explicit model override, got %+v", capabilityLanguageModel)
	}
	for _, tierModelField := range []string{"highModel", "mediumModel", "lowModel", "xlowModel"} {
		if _, isFound := capabilityLanguageModel[tierModelField]; isFound {
			t.Fatalf("expected ordinary model override to omit %s, got %+v", tierModelField, capabilityLanguageModel)
		}
	}
	if capabilityLanguageModel["contextWindowTokens"] != float64(BlueclawDefaultModelContextTokens) {
		t.Fatalf("expected context window to remain tied to default runtime model, got %+v", capabilityLanguageModel)
	}
}

func TestBlueclawRuntimeConfigCanApplyModelOverrideToAllTiers(t *testing.T) {
	document, errorValue := BlueclawRuntimeConfigDocumentWithOptions(RuntimeConfigOptions{
		ModelName:                 "google/test-model",
		ShouldUseModelForAllTiers: true,
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}

	var runtimeConfiguration map[string]any
	if errorValue := json.Unmarshal([]byte(document), &runtimeConfiguration); errorValue != nil {
		t.Fatal(errorValue)
	}

	languageModel := runtimeConfiguration["languageModel"].(map[string]any)
	capabilityLanguageModel := languageModel["capability"].(map[string]any)
	for _, tierModelField := range []string{"model", "highModel", "lowModel", "xlowModel"} {
		if capabilityLanguageModel[tierModelField] != "google/test-model" {
			t.Fatalf("expected %s to use test model, got %+v", tierModelField, capabilityLanguageModel)
		}
	}
	if capabilityLanguageModel["mediumModel"] != BlueclawTestEscalationModelName {
		t.Fatalf("expected mediumModel to stay on the escalation model so the low tier keeps a healthy fallback target, got %+v", capabilityLanguageModel)
	}
}

func TestBlueclawRuntimeConfigSupportsTenantRuntimeIsolation(t *testing.T) {
	document, errorValue := BlueclawRuntimeConfigDocumentWithOptions(RuntimeConfigOptions{
		ModelName:                "x-ai/grok-4.3",
		BaseURL:                  "http://127.0.0.1:18100",
		CapabilitySocketPath:     "/srv/internkim/tenants/pilot-01/internkim/run/capability.sock",
		CapabilityVSockPort:      17100,
		GraphitiEndpoint:         "http://127.0.0.1:18791",
		MattermostBaseURL:        "http://127.0.0.1:18065",
		HostWorkspacePath:        "/srv/internkim/tenants/pilot-01/blueclaw/workspace",
		RootFilesystemImagePath:  "/srv/internkim/tenants/pilot-01/blueclaw/guest/rootfs.ext4",
		WorkspaceImagePath:       "/srv/internkim/tenants/pilot-01/blueclaw/guest/workspace.ext4",
		HostHTTPListenAddress:    "127.0.0.1:18100",
		HealthPortOrService:      "18102",
		GuestHTTPPortOrService:   "18101",
		LogDirectoryPath:         "/srv/internkim/tenants/pilot-01/blueclaw/logs/supervisor",
		RuntimeDirectoryPath:     "/srv/internkim/tenants/pilot-01/blueclaw/guest/runtime",
		OutboundHostDeviceName:   "bctap101",
		OutboundGuestMACAddress:  "AA:FC:00:00:01:01",
		OutboundNetworkCIDR:      "172.31.101.0/30",
		OutboundHostAddressCIDR:  "172.31.101.1/30",
		OutboundGuestAddressCIDR: "172.31.101.2/30",
		OutboundGuestGateway:     "172.31.101.1",
		BridgeListenAddress:      "127.0.0.1:17781",
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}

	var runtimeConfiguration map[string]any
	if errorValue := json.Unmarshal([]byte(document), &runtimeConfiguration); errorValue != nil {
		t.Fatal(errorValue)
	}

	assertNestedValue(t, runtimeConfiguration, []string{"baseURL"}, "http://127.0.0.1:18100")
	assertNestedValue(t, runtimeConfiguration, []string{"capabilities", "vsockPort"}, float64(17100))
	guestListenerProxies := runtimeConfiguration["guest"].(map[string]any)["guestListenerProxies"].([]any)
	if len(guestListenerProxies) != 1 {
		t.Fatalf("expected tenant capability vsock listener proxy, got %+v", guestListenerProxies)
	}
	firstGuestListenerProxy := guestListenerProxies[0].(map[string]any)
	if firstGuestListenerProxy["guestPort"] != float64(17100) || firstGuestListenerProxy["targetUnixSocketPath"] != "/srv/internkim/tenants/pilot-01/internkim/run/capability.sock" {
		t.Fatalf("unexpected tenant capability listener proxy: %+v", firstGuestListenerProxy)
	}
	assertNestedValue(t, runtimeConfiguration, []string{"languageModel", "capability", "model"}, "x-ai/grok-4.3")
	assertNestedValue(t, runtimeConfiguration, []string{"memory", "graphitiEndpoint"}, "http://127.0.0.1:18791")
	assertNestedValue(t, runtimeConfiguration, []string{"connectors", "mattermost", "baseURL"}, "http://127.0.0.1:18065")
	assertNestedValue(t, runtimeConfiguration, []string{"guest", "hostWorkspacePath"}, "/srv/internkim/tenants/pilot-01/blueclaw/workspace")
	assertNestedValue(t, runtimeConfiguration, []string{"guest", "workspaceImagePath"}, "/srv/internkim/tenants/pilot-01/blueclaw/guest/workspace.ext4")
	assertNestedValue(t, runtimeConfiguration, []string{"guest", "hostHTTPListenAddress"}, "127.0.0.1:18100")
	assertNestedValue(t, runtimeConfiguration, []string{"guest", "outboundNetwork", "hostDeviceName"}, "bctap101")
	assertNestedValue(t, runtimeConfiguration, []string{"bridge", "listenAddress"}, "127.0.0.1:17781")
}

func TestBlueclawPolicyDocumentSeedsResourceFirstCircles(t *testing.T) {
	document, errorValue := BlueclawPolicyDocument("owner@example.com")
	if errorValue != nil {
		t.Fatal(errorValue)
	}

	var policyDocument map[string]any
	if errorValue := json.Unmarshal([]byte(document), &policyDocument); errorValue != nil {
		t.Fatal(errorValue)
	}

	people := policyDocument["people"].([]any)
	adminPerson := people[0].(map[string]any)
	adminCircles := adminPerson["circles"].([]any)
	if !containsStringValue(adminCircles, "member") || !containsStringValue(adminCircles, "admin") {
		t.Fatalf("expected admin person member/admin circles, got %+v", adminCircles)
	}
	circles := policyDocument["circles"].([]any)
	for _, expectedCircle := range []string{"member", "c-level", "representative", "admin", "hr"} {
		if !containsPolicyCircle(circles, expectedCircle) {
			t.Fatalf("expected circle %q, got %+v", expectedCircle, circles)
		}
	}
	circleSync := policyDocument["circleSync"].(map[string]any)
	mattermostPrivateChannels := circleSync["mattermostPrivateChannels"].([]any)
	for _, expectedChannel := range []string{"circle-c-level", "circle-representative", "circle-admin", "circle-hr"} {
		if !containsPolicyMattermostChannel(mattermostPrivateChannels, expectedChannel) {
			t.Fatalf("expected Mattermost circle channel %q, got %+v", expectedChannel, mattermostPrivateChannels)
		}
	}
	resourceAccess := policyDocument["resourceAccess"].([]any)
	if !containsPolicyResource(resourceAccess, "file:circle:c-level", "c-level") {
		t.Fatalf("expected c-level file resource rule, got %+v", resourceAccess)
	}
	if !containsPolicyResource(resourceAccess, "file:circle:hr", "hr") {
		t.Fatalf("expected HR compensation file resource rule, got %+v", resourceAccess)
	}
	if !containsPolicyResource(resourceAccess, "api:flow.summary", "member") {
		t.Fatalf("expected member Flow summary API rule, got %+v", resourceAccess)
	}
	if !containsPolicyResource(resourceAccess, "api:flow.task", "member") {
		t.Fatalf("expected member Flow task API rule, got %+v", resourceAccess)
	}
	if !containsPolicyResource(resourceAccess, "api:flow.definition", "admin") {
		t.Fatalf("expected admin Flow definition API rule, got %+v", resourceAccess)
	}
	if !containsPolicyResource(resourceAccess, "tool:task_add", "member") {
		t.Fatalf("expected member Flow tool rule, got %+v", resourceAccess)
	}
	if !containsPolicyResource(resourceAccess, "tool:task_list", "member") {
		t.Fatalf("expected member Flow task list tool rule, got %+v", resourceAccess)
	}
	if !containsPolicyResource(resourceAccess, "tool:task_update", "member") {
		t.Fatalf("expected member Flow update tool rule, got %+v", resourceAccess)
	}
	for _, toolName := range []string{"message_context", "message_search", "message_send", "message_update", "message_delete"} {
		if !containsPolicyResource(resourceAccess, "tool:"+toolName, "member") {
			t.Fatalf("expected member %s tool rule, got %+v", toolName, resourceAccess)
		}
	}
	if !containsPolicyResource(resourceAccess, "tool:mail_message_search", "member") {
		t.Fatalf("expected member mail search tool rule, got %+v", resourceAccess)
	}
	for _, toolName := range []string{"site_serve", "site_list", "site_unserve"} {
		if !containsPolicyResource(resourceAccess, "tool:"+toolName, "member") {
			t.Fatalf("expected member %s tool rule, got %+v", toolName, resourceAccess)
		}
	}
	for _, toolName := range []string{"site.create", "site.status", "site.preview", "site.publish", "site.delete", "site.history", "site.diff", "site.logs", "site.restore", "site.repair", "site.rollback", "site.unpublish"} {
		if containsPolicyResource(resourceAccess, "tool:"+toolName, "member") {
			t.Fatalf("expected removed %s policy to be absent, got %+v", toolName, resourceAccess)
		}
	}
	if !containsPolicyResource(resourceAccess, "tool:company.broadcast.send", "representative") {
		t.Fatalf("expected representative broadcast tool rule, got %+v", resourceAccess)
	}
}

func assertNestedValue(t *testing.T, document map[string]any, path []string, expectedValue any) {
	t.Helper()
	var currentValue any = document
	for _, key := range path {
		currentDocument, isDocument := currentValue.(map[string]any)
		if !isDocument {
			t.Fatalf("expected document at %v, got %+v", path, currentValue)
		}
		currentValue = currentDocument[key]
	}
	if currentValue != expectedValue {
		t.Fatalf("expected %v at %v, got %+v", expectedValue, path, currentValue)
	}
}

func containsStringValue(values []any, expectedValue string) bool {
	for _, value := range values {
		if value == expectedValue {
			return true
		}
	}
	return false
}

func containsDescriptor(values []any, expectedName string, expectedField string) bool {
	for _, value := range values {
		descriptor, ok := value.(map[string]any)
		if !ok {
			continue
		}
		if descriptor["name"] == expectedName {
			_, isFound := descriptor[expectedField]
			return isFound
		}
	}
	return false
}

func containsCompletionEvidence(values []any, expectedName string, expectedMode string, expectedAction string, expectedTargetKind string) bool {
	for _, value := range values {
		descriptor, ok := value.(map[string]any)
		if !ok || descriptor["name"] != expectedName {
			continue
		}
		evidence, ok := descriptor["completionEvidence"].(map[string]any)
		if !ok {
			return false
		}
		return evidence["mode"] == expectedMode && evidence["action"] == expectedAction && evidence["targetKind"] == expectedTargetKind
	}
	return false
}

func containsResultContract(values []any, expectedName string, expectedProperty string, expectedObjectType string, expectedEffect string) bool {
	for _, value := range values {
		descriptor, ok := value.(map[string]any)
		if !ok || descriptor["name"] != expectedName {
			continue
		}
		contract, ok := descriptor["resultContract"].(map[string]any)
		if !ok {
			return false
		}
		schema, _ := contract["schema"].(map[string]any)
		properties, _ := schema["properties"].(map[string]any)
		effects, _ := contract["effects"].([]any)
		if _, hasProperty := properties[expectedProperty]; !hasProperty || len(effects) != 1 {
			return false
		}
		effect, _ := effects[0].(map[string]any)
		return effect["objectType"] == expectedObjectType &&
			effect["effect"] == expectedEffect &&
			effect["resultField"] == expectedProperty &&
			effect["effectIdentity"] == "id"
	}
	return false
}

func containsPolicyCircle(values []any, expectedCircleID string) bool {
	for _, value := range values {
		circle, isCircle := value.(map[string]any)
		if !isCircle || circle["circleID"] != expectedCircleID {
			continue
		}
		return circle["workspaceDirectoryPath"] == "/workspace/circles/"+expectedCircleID
	}
	return false
}

func containsPolicyMattermostChannel(values []any, expectedChannelName string) bool {
	for _, value := range values {
		channel, isChannel := value.(map[string]any)
		if isChannel && channel["channelName"] == expectedChannelName {
			return true
		}
	}
	return false
}

func containsPolicyResource(values []any, expectedResource string, expectedCircle string) bool {
	for _, value := range values {
		resourceAccess, isResourceAccess := value.(map[string]any)
		if !isResourceAccess || resourceAccess["resource"] != expectedResource {
			continue
		}
		circles, isCircles := resourceAccess["circles"].([]any)
		return isCircles && containsStringValue(circles, expectedCircle)
	}
	return false
}

func TestBlueclawServiceDoesNotExposeOpenRouterKeyAsEnvironmentFile(t *testing.T) {
	serviceDocument := BlueclawServiceUnit()
	if strings.Contains(serviceDocument, "EnvironmentFile=") {
		t.Fatal("expected Blueclaw service to avoid OpenRouter key environment files")
	}
	if !strings.Contains(serviceDocument, BlueclawSupervisorBinaryPath) {
		t.Fatal("expected Blueclaw service to run the guest supervisor")
	}
	if strings.Contains(serviceDocument, "ExecStart="+BlueclawBinaryPath+" ") {
		t.Fatal("expected Blueclaw service not to run the host blueclaw binary directly")
	}
}

func TestCapabilitydServiceUsesOpenRouterFirstAutoRouting(t *testing.T) {
	serviceDocument := CapabilitydServiceUnit()
	if strings.Contains(serviceDocument, "--prefer-companion-llm") {
		t.Fatalf("expected capabilityd service not to prefer companion LLM by default, got %s", serviceDocument)
	}
	if !strings.Contains(serviceDocument, "--companion-url http://127.0.0.1:18080/_internkim/companion") {
		t.Fatalf("expected capabilityd service to keep companion URL without making it first, got %s", serviceDocument)
	}
	for _, forbiddenValue := range []string{"workers", "--openrouter-gateway-secret"} {
		if strings.Contains(serviceDocument, forbiddenValue) {
			t.Fatalf("expected physical Jetson capabilityd service to avoid fronting gateway value %q, got %s", forbiddenValue, serviceDocument)
		}
	}
}

func TestCapabilitydServiceOrdersAfterAdmind(t *testing.T) {
	serviceDocument := CapabilitydServiceUnit()
	afterLine := ""
	wantsLine := ""
	for _, line := range strings.Split(serviceDocument, "\n") {
		if strings.HasPrefix(line, "After=") {
			afterLine = line
		}
		if strings.HasPrefix(line, "Wants=") {
			wantsLine = line
		}
	}
	if !strings.Contains(afterLine, "internkim-admind.service") {
		t.Fatalf("expected capabilityd to boot after admind, got %s", afterLine)
	}
	if !strings.Contains(wantsLine, "internkim-admind.service") {
		t.Fatalf("expected capabilityd to want admind, got %s", wantsLine)
	}
}

func TestCapabilitydServiceCanUseTestModelFromEnvironment(t *testing.T) {
	const testModelName = "google/test-model"
	t.Setenv(BlueclawTestModelEnvironment, testModelName)

	serviceDocument := CapabilitydServiceUnit()
	if !strings.Contains(serviceDocument, "--openrouter-model "+testModelName) {
		t.Fatalf("expected capabilityd service to use test model, got %s", serviceDocument)
	}
	if strings.Contains(serviceDocument, "--force-openrouter-model") {
		t.Fatalf("expected capabilityd service to keep explicit tier models so escalation fallback can reach a healthy model, got %s", serviceDocument)
	}
}

func TestLlamaCppServiceUnitRunsLocalServer(t *testing.T) {
	serviceDocument := LlamaCppServiceUnit()
	for _, expectedValue := range []string{
		"ExecStart=" + locallm.LlamaCppBinaryPath,
		locallm.LlamaCppModelPath,
		"--host " + locallm.LlamaCppHost,
		"--port " + locallm.LlamaCppPort,
		"LD_LIBRARY_PATH=" + locallm.LlamaCppLibraryDir,
		"Restart=on-failure",
	} {
		if !strings.Contains(serviceDocument, expectedValue) {
			t.Fatalf("expected llama.cpp service unit to contain %q, got %s", expectedValue, serviceDocument)
		}
	}
}

func TestLlamaCppEmbeddingServiceUnitRunsEmbeddingServer(t *testing.T) {
	serviceDocument := LlamaCppEmbeddingServiceUnit()
	for _, expectedValue := range []string{
		"ExecStart=" + locallm.LlamaCppBinaryPath,
		locallm.LlamaCppEmbeddingModelPath,
		"--host " + locallm.LlamaCppHost,
		"--port " + locallm.LlamaCppEmbeddingPort,
		"--embeddings",
		"--pooling cls",
		"--batch-size " + locallm.LlamaCppEmbeddingBatchSize,
		"--ubatch-size " + locallm.LlamaCppEmbeddingUBatchSize,
		"LD_LIBRARY_PATH=" + locallm.LlamaCppLibraryDir,
		"Restart=on-failure",
	} {
		if !strings.Contains(serviceDocument, expectedValue) {
			t.Fatalf("expected llama.cpp embedding service unit to contain %q, got %s", expectedValue, serviceDocument)
		}
	}
}

func TestAMacGetsTheMonitorItHasAndNoHostTap(t *testing.T) {
	testCases := []struct {
		monitor                    string
		expectedMonitor            string
		expectedOutboundNetworking bool
		expectedEnforcement        string
	}{
		{"", CloudHypervisorMonitorName, true, "hostBindMount"},
		{CloudHypervisorMonitorName, CloudHypervisorMonitorName, true, "hostBindMount"},
		{VfkitMonitorName, VfkitMonitorName, false, "immutableFlags"},
	}

	for _, testCase := range testCases {
		document, errorValue := BlueclawRuntimeConfigDocumentWithOptions(RuntimeConfigOptions{
			ModelName:             "x-ai/grok-4.3",
			VirtualMachineMonitor: testCase.monitor,
		})
		if errorValue != nil {
			t.Fatal(errorValue)
		}
		var runtimeConfiguration map[string]any
		if errorValue := json.Unmarshal([]byte(document), &runtimeConfiguration); errorValue != nil {
			t.Fatal(errorValue)
		}
		guest := runtimeConfiguration["guest"].(map[string]any)

		if guest["virtualMachineMonitor"] != testCase.expectedMonitor {
			t.Fatalf("expected %q, got %+v", testCase.expectedMonitor, guest["virtualMachineMonitor"])
		}
		if guest["vfkitPath"] != BlueclawVfkitPath {
			t.Fatalf("a document naming vfkit without its path cannot start one: %+v", guest)
		}
		if guest["deliveryReadOnlyEnforcement"] != testCase.expectedEnforcement {
			t.Fatalf("expected %q, got %+v", testCase.expectedEnforcement, guest["deliveryReadOnlyEnforcement"])
		}
		outboundNetwork := guest["outboundNetwork"].(map[string]any)
		if outboundNetwork["enabled"] != testCase.expectedOutboundNetworking {
			t.Fatalf("a Mac has no tap device to build, so %q must not ask for one: %+v", testCase.monitor, outboundNetwork)
		}
	}
}

func TestAHostThatIsNotADeviceNamesItsOwnPaths(t *testing.T) {
	document, errorValue := BlueclawRuntimeConfigDocumentWithOptions(RuntimeConfigOptions{
		ModelName:             "x-ai/grok-4.3",
		VirtualMachineMonitor: VfkitMonitorName,
		KernelImagePath:       "/Users/someone/.internkim/blueclaw/vmlinux.bin",
		VfkitPath:             "/opt/homebrew/bin/vfkit",
		DeliveryDirectoryPath: "/Users/someone/.internkim/blueclaw/delivery",
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	var runtimeConfiguration map[string]any
	if errorValue := json.Unmarshal([]byte(document), &runtimeConfiguration); errorValue != nil {
		t.Fatal(errorValue)
	}
	guest := runtimeConfiguration["guest"].(map[string]any)

	for field, expected := range map[string]string{
		"kernelImagePath":       "/Users/someone/.internkim/blueclaw/vmlinux.bin",
		"vfkitPath":             "/opt/homebrew/bin/vfkit",
		"deliveryDirectoryPath": "/Users/someone/.internkim/blueclaw/delivery",
	} {
		if guest[field] != expected {
			t.Fatalf("a Mac has none of the device paths, so %s must be the one given: %+v", field, guest[field])
		}
	}
}

func TestADeviceKeepsEveryPathItAlwaysHad(t *testing.T) {
	document, errorValue := BlueclawRuntimeConfigDocumentWithOptions(RuntimeConfigOptions{ModelName: "x-ai/grok-4.3"})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	var runtimeConfiguration map[string]any
	if errorValue := json.Unmarshal([]byte(document), &runtimeConfiguration); errorValue != nil {
		t.Fatal(errorValue)
	}
	guest := runtimeConfiguration["guest"].(map[string]any)

	for field, expected := range map[string]string{
		"kernelImagePath":       BlueclawKernelImagePath,
		"vfkitPath":             BlueclawVfkitPath,
		"deliveryDirectoryPath": BlueclawDeliveryReadOnlyPath,
	} {
		if guest[field] != expected {
			t.Fatalf("a device that names no paths keeps its own %s, got %+v", field, guest[field])
		}
	}
}

func TestAHostWithNoCapabilityDaemonAsksForNoProxyToIt(t *testing.T) {
	for _, testCase := range []struct {
		hostRunsNoCapabilityDaemon bool
		expectedProxyCount         int
	}{
		{false, 1},
		{true, 0},
	} {
		document, errorValue := BlueclawRuntimeConfigDocumentWithOptions(RuntimeConfigOptions{
			ModelName:                  "x-ai/grok-4.3",
			HostRunsNoCapabilityDaemon: testCase.hostRunsNoCapabilityDaemon,
		})
		if errorValue != nil {
			t.Fatal(errorValue)
		}
		var runtimeConfiguration map[string]any
		if errorValue := json.Unmarshal([]byte(document), &runtimeConfiguration); errorValue != nil {
			t.Fatal(errorValue)
		}
		proxies := runtimeConfiguration["guest"].(map[string]any)["guestListenerProxies"].([]any)

		if len(proxies) != testCase.expectedProxyCount {
			t.Fatalf("a proxy to a socket nobody serves reports a failure every boot for work nobody asked for, got %d", len(proxies))
		}
	}
}
