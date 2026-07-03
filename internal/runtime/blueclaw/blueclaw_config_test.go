package blueclaw

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gitlab.com/eastriver/internkim/internal/capabilities"
	"gitlab.com/eastriver/internkim/internal/runtime/locallm"
)

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
	capabilityToolNames := capabilityConfiguration["toolNames"].([]any)
	if !containsStringValue(capabilityToolNames, "user.confirm") {
		t.Fatalf("expected companion capability tools, got %+v", capabilityToolNames)
	}
	capabilityToolDescriptors := capabilityConfiguration["toolDescriptors"].([]any)
	if !containsDescriptor(capabilityToolDescriptors, "browser.open", "inputSchema") {
		t.Fatalf("expected browser.open descriptor with input schema, got %+v", capabilityToolDescriptors)
	}
	if !containsDescriptor(capabilityToolDescriptors, "user.confirm", "requiresApproval") {
		t.Fatalf("expected user.confirm descriptor to require approval, got %+v", capabilityToolDescriptors)
	}
	if !containsCompletionEvidence(capabilityToolDescriptors, "message.send", "success", "send_message", "message") {
		t.Fatalf("expected platform message send descriptor to preserve completion evidence, got %+v", capabilityToolDescriptors)
	}
	if !containsCompletionEvidence(capabilityToolDescriptors, "mail.message.send", "success", "send_email", "email") {
		t.Fatalf("expected mail send descriptor to preserve completion evidence, got %+v", capabilityToolDescriptors)
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
	if agent["defaultEffortLevel"] != "standard" {
		t.Fatalf("expected default effort level, got %v", agent["defaultEffortLevel"])
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
	agentProfiles := runtimeConfiguration["agentProfiles"].([]any)
	defaultProfile := agentProfiles[0].(map[string]any)
	allowedToolNames := defaultProfile["allowedToolNames"].([]any)
	for _, expectedToolName := range blueclawNativeToolNames {
		if !containsStringValue(allowedToolNames, expectedToolName) {
			t.Fatalf("expected default agent profile to allow internal tool %q, got %+v", expectedToolName, allowedToolNames)
		}
	}
	for _, disabledToolName := range []string{"google.docs.create", "google.sheets.create", "google.gmail.send", "google.calendar.event", "google.calendar.list", "google.drive.import_pptx"} {
		if containsStringValue(allowedToolNames, disabledToolName) {
			t.Fatalf("expected default profile to omit disabled Google Workspace tool %q, got %+v", disabledToolName, allowedToolNames)
		}
	}
	capabilityConfiguration = runtimeConfiguration["capabilities"].(map[string]any)
	capabilityToolNames = capabilityConfiguration["toolNames"].([]any)
	for _, expectedToolName := range capabilities.DefaultToolNames() {
		if !containsStringValue(capabilityToolNames, expectedToolName) {
			t.Fatalf("expected capability tool list to include default tool %q, got %+v", expectedToolName, capabilityToolNames)
		}
	}
	for _, disabledToolName := range []string{"google.docs.create", "google.sheets.create", "google.gmail.send", "google.calendar.event", "google.calendar.list", "google.drive.import_pptx"} {
		if containsStringValue(capabilityToolNames, disabledToolName) {
			t.Fatalf("expected capability tool list to omit disabled Google Workspace tool %q, got %+v", disabledToolName, capabilityToolNames)
		}
	}
	terminal := runtimeConfiguration["terminal"].(map[string]any)
	if terminal["mode"] != "firecrackerGuest" {
		t.Fatalf("expected firecracker guest terminal mode, got %q", terminal["mode"])
	}
	if terminal["posixHelperPath"] != BlueclawPOSIXHelperPath {
		t.Fatalf("expected POSIX helper path, got %q", terminal["posixHelperPath"])
	}
	allowedExecutableNames := terminal["allowedExecutableNames"].([]any)
	if !containsStringValue(allowedExecutableNames, "capability") {
		t.Fatalf("expected capability CLI executable to be allowed, got %+v", allowedExecutableNames)
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
	firecracker := runtimeConfiguration["firecracker"].(map[string]any)
	if firecracker["vcpuCount"] != float64(BlueclawFirecrackerDefaultVirtualCPUCount) || firecracker["memoryMiB"] != float64(BlueclawFirecrackerDefaultMemoryMiB) {
		t.Fatalf("expected bounded Firecracker resources, got %+v", firecracker)
	}
	outboundNetwork := firecracker["outboundNetwork"].(map[string]any)
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
	forbiddenFragments := []string{"apiKeyPath", "botTokenPath", "signingSecretPath", "wrapperPath", "modelPath", "backend"}
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
	t.Setenv(BlueclawTestGenerationSeedEnvironment, "41")
	t.Setenv(BlueclawTestGenerationTemperatureEnvironment, "0")

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
	if options.GenerationSeed == nil || *options.GenerationSeed != 41 {
		t.Fatalf("expected seed from environment, got %+v", options)
	}
	if options.GenerationTemperature == nil || *options.GenerationTemperature != 0 {
		t.Fatalf("expected temperature from environment, got %+v", options)
	}
}

func TestBlueclawRuntimeConfigOptionsRejectInvalidGenerationEnvironment(t *testing.T) {
	t.Setenv(BlueclawTestGenerationSeedEnvironment, "not-a-seed")

	_, errorValue := BlueclawRuntimeConfigOptionsFromEnvironment()
	if errorValue == nil || !strings.Contains(errorValue.Error(), BlueclawTestGenerationSeedEnvironment) {
		t.Fatalf("expected seed environment error, got %v", errorValue)
	}
}

func TestBlueclawRuntimeKnowsBuiltinSkillToolsWithoutExposingAllByDefault(t *testing.T) {
	allowedToolNames := stringSet(BlueclawDefaultAllowedToolNames())
	skillScopedToolNames := stringSet([]string{
		"site.build",
		"site.repair",
		"site.preview",
	})
	disabledSkillToolNames := stringSet([]string{
		"google.docs.create",
		"google.sheets.create",
		"google.gmail.send",
	})
	skillPaths, errorValue := filepath.Glob(filepath.Join("..", "..", "..", "assets", "blueclaw-workspace", "skills", "*", "SKILL.md"))
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if len(skillPaths) == 0 {
		t.Fatal("expected built-in skills to be present")
	}

	for _, skillPath := range skillPaths {
		for _, toolName := range parseSkillAllowedToolNames(t, skillPath) {
			if disabledSkillToolNames[toolName] {
				continue
			}
			if allowedToolNames[toolName] || skillScopedToolNames[toolName] {
				continue
			}
			t.Fatalf("expected built-in skill tool %q from %s to be default-allowed or explicitly skill-scoped", toolName, skillPath)
		}
	}

	for toolName := range skillScopedToolNames {
		if allowedToolNames[toolName] {
			t.Fatalf("expected skill-scoped tool %q not to be exposed by the default runtime profile", toolName)
		}
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
	for _, tierModelField := range []string{"highModel", "mediumModel", "lowModel", "xlowModel", "codingModel"} {
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
	for _, tierModelField := range []string{"model", "highModel", "lowModel", "xlowModel", "codingModel"} {
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
		RootFilesystemImagePath:  "/srv/internkim/tenants/pilot-01/blueclaw/firecracker/rootfs.ext4",
		WorkspaceImagePath:       "/srv/internkim/tenants/pilot-01/blueclaw/firecracker/workspace.ext4",
		HostHTTPListenAddress:    "127.0.0.1:18100",
		HealthPortOrService:      "18102",
		GuestHTTPPortOrService:   "18101",
		LogDirectoryPath:         "/srv/internkim/tenants/pilot-01/blueclaw/logs/supervisor",
		RuntimeDirectoryPath:     "/srv/internkim/tenants/pilot-01/blueclaw/firecracker/runtime",
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
	guestListenerProxies := runtimeConfiguration["firecracker"].(map[string]any)["guestListenerProxies"].([]any)
	firstGuestListenerProxy := guestListenerProxies[0].(map[string]any)
	if firstGuestListenerProxy["targetUnixSocketPath"] != "/srv/internkim/tenants/pilot-01/internkim/run/capability.sock" {
		t.Fatalf("unexpected tenant capability socket proxy: %+v", firstGuestListenerProxy)
	}
	assertNestedValue(t, runtimeConfiguration, []string{"languageModel", "capability", "model"}, "x-ai/grok-4.3")
	assertNestedValue(t, runtimeConfiguration, []string{"memory", "graphitiEndpoint"}, "http://127.0.0.1:18791")
	assertNestedValue(t, runtimeConfiguration, []string{"connectors", "mattermost", "baseURL"}, "http://127.0.0.1:18065")
	assertNestedValue(t, runtimeConfiguration, []string{"firecracker", "hostWorkspacePath"}, "/srv/internkim/tenants/pilot-01/blueclaw/workspace")
	assertNestedValue(t, runtimeConfiguration, []string{"firecracker", "workspaceImagePath"}, "/srv/internkim/tenants/pilot-01/blueclaw/firecracker/workspace.ext4")
	assertNestedValue(t, runtimeConfiguration, []string{"firecracker", "hostHTTPListenAddress"}, "127.0.0.1:18100")
	assertNestedValue(t, runtimeConfiguration, []string{"firecracker", "outboundNetwork", "hostDeviceName"}, "bctap101")
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
	if !containsStringValue(adminCircles, "staff") || !containsStringValue(adminCircles, "admin") {
		t.Fatalf("expected admin person staff/admin circles, got %+v", adminCircles)
	}
	circles := policyDocument["circles"].([]any)
	for _, expectedCircle := range []string{"staff", "c-level", "representative", "admin", "hr-compensation"} {
		if !containsPolicyCircle(circles, expectedCircle) {
			t.Fatalf("expected circle %q, got %+v", expectedCircle, circles)
		}
	}
	circleSync := policyDocument["circleSync"].(map[string]any)
	mattermostPrivateChannels := circleSync["mattermostPrivateChannels"].([]any)
	for _, expectedChannel := range []string{"circle-c-level", "circle-representative", "circle-admin", "circle-hr-compensation"} {
		if !containsPolicyMattermostChannel(mattermostPrivateChannels, expectedChannel) {
			t.Fatalf("expected Mattermost circle channel %q, got %+v", expectedChannel, mattermostPrivateChannels)
		}
	}
	resourceAccess := policyDocument["resourceAccess"].([]any)
	if !containsPolicyResource(resourceAccess, "file:circle:c-level", "c-level") {
		t.Fatalf("expected c-level file resource rule, got %+v", resourceAccess)
	}
	if !containsPolicyResource(resourceAccess, "file:circle:hr-compensation", "hr-compensation") {
		t.Fatalf("expected HR compensation file resource rule, got %+v", resourceAccess)
	}
	if !containsPolicyResource(resourceAccess, "api:flow.summary", "staff") {
		t.Fatalf("expected staff Flow summary API rule, got %+v", resourceAccess)
	}
	if !containsPolicyResource(resourceAccess, "api:flow.task", "staff") {
		t.Fatalf("expected staff Flow task API rule, got %+v", resourceAccess)
	}
	if !containsPolicyResource(resourceAccess, "api:flow.definition", "admin") {
		t.Fatalf("expected admin Flow definition API rule, got %+v", resourceAccess)
	}
	if !containsPolicyResource(resourceAccess, "tool:task.add", "staff") {
		t.Fatalf("expected staff Flow tool rule, got %+v", resourceAccess)
	}
	if !containsPolicyResource(resourceAccess, "tool:task.list", "staff") {
		t.Fatalf("expected staff Flow task list tool rule, got %+v", resourceAccess)
	}
	if !containsPolicyResource(resourceAccess, "tool:task.update", "staff") {
		t.Fatalf("expected staff Flow update tool rule, got %+v", resourceAccess)
	}
	for _, toolName := range []string{"message.context", "message.search", "message.send", "message.update", "message.delete"} {
		if !containsPolicyResource(resourceAccess, "tool:"+toolName, "staff") {
			t.Fatalf("expected staff %s tool rule, got %+v", toolName, resourceAccess)
		}
	}
	if !containsPolicyResource(resourceAccess, "tool:channel.update", "admin") {
		t.Fatalf("expected admin Mattermost channel update tool rule, got %+v", resourceAccess)
	}
	if !containsPolicyResource(resourceAccess, "tool:mail.message.search", "staff") {
		t.Fatalf("expected staff mail search tool rule, got %+v", resourceAccess)
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

func stringSet(values []string) map[string]bool {
	set := map[string]bool{}
	for _, value := range values {
		set[value] = true
	}
	return set
}

func parseSkillAllowedToolNames(t *testing.T, skillPath string) []string {
	t.Helper()
	document, errorValue := os.ReadFile(skillPath)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	allowedToolNames := []string{}
	isReadingAllowedTools := false
	for _, line := range strings.Split(string(document), "\n") {
		trimmedLine := strings.TrimSpace(line)
		if trimmedLine == "allowed-tools:" {
			isReadingAllowedTools = true
			continue
		}
		if !isReadingAllowedTools {
			continue
		}
		if trimmedLine == "" {
			continue
		}
		if !strings.HasPrefix(trimmedLine, "- ") {
			break
		}
		allowedToolNames = append(allowedToolNames, strings.TrimSpace(strings.TrimPrefix(trimmedLine, "- ")))
	}
	return allowedToolNames
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
		t.Fatal("expected Blueclaw service to run the Firecracker supervisor")
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
	if !strings.Contains(serviceDocument, "--mattermost-url "+BlueclawMattermostLocalURL) {
		t.Fatalf("expected capabilityd service to use local Mattermost URL, got %s", serviceDocument)
	}
	if !strings.Contains(serviceDocument, "--mattermost-token "+BlueclawMattermostTokenPath) {
		t.Fatalf("expected capabilityd service to include Mattermost bot token path, got %s", serviceDocument)
	}
	for _, forbiddenValue := range []string{"internkim-llm-gateway", "workers", "llm-gateway-shared-secret", "--openrouter-gateway-secret"} {
		if strings.Contains(serviceDocument, forbiddenValue) {
			t.Fatalf("expected physical Jetson capabilityd service to avoid Worker gateway value %q, got %s", forbiddenValue, serviceDocument)
		}
	}
}

func TestCapabilitydServiceCanUseTestModelFromEnvironment(t *testing.T) {
	t.Setenv(BlueclawTestModelEnvironment, BlueclawTestModelName)

	serviceDocument := CapabilitydServiceUnit()
	if !strings.Contains(serviceDocument, "--openrouter-model "+BlueclawTestModelName) {
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
		"--pooling mean",
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
