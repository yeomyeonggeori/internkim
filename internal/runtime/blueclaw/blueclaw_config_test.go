package blueclaw

import (
	"encoding/json"
	"strings"
	"testing"

	"gitlab.com/eastriver/internkim/internal/runtime/locallm"
)

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
	capabilities := runtimeConfiguration["capabilities"].(map[string]any)
	if capabilities["transport"] != "vsock" {
		t.Fatalf("expected capability vsock transport, got %q", capabilities["transport"])
	}
	if capabilities["timeoutSecond"] != float64(BlueclawCapabilityTimeoutSecond) {
		t.Fatalf("expected capability timeout %d, got %v", BlueclawCapabilityTimeoutSecond, capabilities["timeoutSecond"])
	}
	if capabilities["unixSocketPath"] != "" {
		t.Fatalf("expected capability unix socket path to be omitted for guest runtime, got %q", capabilities["unixSocketPath"])
	}
	capabilityToolNames := capabilities["toolNames"].([]any)
	if !containsStringValue(capabilityToolNames, "user.confirm") {
		t.Fatalf("expected companion capability tools, got %+v", capabilityToolNames)
	}
	capabilityToolDescriptors := capabilities["toolDescriptors"].([]any)
	if !containsDescriptor(capabilityToolDescriptors, "browser.open", "inputSchema") {
		t.Fatalf("expected browser.open descriptor with input schema, got %+v", capabilityToolDescriptors)
	}
	if !containsDescriptor(capabilityToolDescriptors, "user.confirm", "requiresApproval") {
		t.Fatalf("expected user.confirm descriptor to require approval, got %+v", capabilityToolDescriptors)
	}
	routing := capabilities["routing"].(map[string]any)
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
	for _, expectedToolName := range []string{"conversation.history", "memory.search", "memory.remember", "math.calculate", "web.search", "web.fetch", "terminal.run", "terminal.session", "browser_handoff.openURL", "ask.confirm", "ask.choice", "ask.input", "file.read", "file.write", "file.promote", "file.attach", "skill.add", "skill.remove", "skill.search", "tool.describe", "schedule.create", "schedule.cancel"} {
		if !containsStringValue(allowedToolNames, expectedToolName) {
			t.Fatalf("expected default agent profile to allow internal tool %q, got %+v", expectedToolName, allowedToolNames)
		}
	}
	if !containsStringValue(allowedToolNames, "conversation.history") || !containsStringValue(allowedToolNames, "memory.search") || !containsStringValue(allowedToolNames, "memory.remember") {
		t.Fatalf("expected default agent profile to allow internal tools, got %+v", allowedToolNames)
	}
	for _, disabledToolName := range []string{"site.app.create", "site.app.publish", "platform.dm.send", "calendar.event.add", "mail.message.search", "flow.task.add", "google.docs.create", "google.sheets.create", "google.gmail.send", "google.calendar.event", "google.calendar.list", "google.drive.import_pptx"} {
		if containsStringValue(allowedToolNames, disabledToolName) {
			t.Fatalf("expected default profile to omit domain tool %q, got %+v", disabledToolName, allowedToolNames)
		}
	}
	capabilitiesConfiguration := runtimeConfiguration["capabilities"].(map[string]any)
	capabilityToolNames = capabilitiesConfiguration["toolNames"].([]any)
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
	if capabilityLanguageModel["contextWindowTokens"] != float64(BlueclawDefaultModelContextTokens) {
		t.Fatalf("expected context window to remain tied to default runtime model, got %+v", capabilityLanguageModel)
	}
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
	if !containsPolicyResource(resourceAccess, "tool:flow.task.add", "staff") {
		t.Fatalf("expected staff Flow tool rule, got %+v", resourceAccess)
	}
	if !containsPolicyResource(resourceAccess, "tool:mail.message.search", "staff") {
		t.Fatalf("expected staff mail search tool rule, got %+v", resourceAccess)
	}
	if !containsPolicyResource(resourceAccess, "tool:company.broadcast.send", "representative") {
		t.Fatalf("expected representative broadcast tool rule, got %+v", resourceAccess)
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

func TestCapabilitydServicePrefersCompanionLLM(t *testing.T) {
	serviceDocument := CapabilitydServiceUnit()
	if !strings.Contains(serviceDocument, "--prefer-companion-llm") {
		t.Fatalf("expected capabilityd service to prefer companion LLM, got %s", serviceDocument)
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
