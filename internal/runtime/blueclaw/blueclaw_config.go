package blueclaw

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path"
	"strconv"
	"strings"

	"gitlab.com/eastriver/internkim/internal/capabilities"
)

const (
	BlueclawCapabilityTimeoutSecond                     = 0
	BlueclawPinnedMemoryHardLimitCharacterCount         = 6000
	BlueclawPinnedMemoryCompressionTargetCharacterCount = 3500
	BlueclawFirecrackerDefaultVirtualCPUCount           = 2
	BlueclawFirecrackerDefaultMemoryMiB                 = 4096
	BlueclawTestEscalationModelName                     = "google/gemini-3.1-flash-lite"
	BlueclawTestModelEnvironment                        = "INTERNKIM_TEST_MODEL"
	BlueclawTestModelTierEnvironment                    = "INTERNKIM_TEST_MODEL_TIER"
	BlueclawTestMaximumModelTierEnvironment             = "INTERNKIM_TEST_MAXIMUM_MODEL_TIER"
	BlueclawTestGenerationSeedEnvironment               = "INTERNKIM_TEST_GENERATION_SEED"
	BlueclawTestGenerationTemperatureEnvironment        = "INTERNKIM_TEST_GENERATION_TEMPERATURE"
	BlueclawSDKDModeEnvironment                         = "INTERNKIM_BLUECLAW_SDKD_MODE"
	BlueclawAdminTaskDiagnosticEnvironment              = "INTERNKIM_BLUECLAW_ADMIN_TASK_DIAGNOSTIC"
	BlueclawSDKDTopologyDiagnosticProfileName           = "sdkd-diagnostic"
	BlueclawSDKDTopologyDiagnosticToolSentinel          = "sdkd.diagnostic.no_tools"
	LocalOnlyEnvironment                                = "INTERNKIM_LOCAL_ONLY"
	BlueclawVirtualCPUCountEnvironment                  = "INTERNKIM_BLUECLAW_VCPU_COUNT"
)

type defaultCircleDefinition struct {
	CircleID              string
	DisplayName           string
	MattermostChannelName string
}

type RuntimeConfigOptions struct {
	ModelName                 string
	AdminTaskLinkBaseURL      string
	BaseURL                   string
	DirectExecution           bool
	WorkspaceRootPath         string
	POSIXHelperPath           string
	DatabaseConnectionString  string
	MigrationDirectoryPath    string
	CapabilitySocketPath      string
	CapabilityVSockPort       int
	GraphitiEndpoint          string
	MattermostBaseURL         string
	HostWorkspacePath         string
	RootFilesystemImagePath   string
	WorkspaceImagePath        string
	HostHTTPListenAddress     string
	HealthPortOrService       string
	GuestHTTPPortOrService    string
	LogDirectoryPath          string
	RuntimeDirectoryPath      string
	OutboundHostDeviceName    string
	OutboundGuestMACAddress   string
	OutboundNetworkCIDR       string
	OutboundHostAddressCIDR   string
	OutboundGuestAddressCIDR  string
	OutboundGuestGateway      string
	BridgeListenAddress       string
	GenerationSeed            *int64
	GenerationTemperature     *float64
	MaximumModelTier          string
	ShouldUseModelForAllTiers bool
	DefaultTaskLevel          string
	VirtualCPUCount           int
	SDKDMode                  string
	AllowAdminTaskDiagnostic  bool
	LocalOnly                 bool
}

var defaultCircleDefinitions = []defaultCircleDefinition{
	{CircleID: "staff", DisplayName: "Staff"},
	{CircleID: "c-level", DisplayName: "C-level", MattermostChannelName: "circle-c-level"},
	{CircleID: "representative", DisplayName: "Representative", MattermostChannelName: "circle-representative"},
	{CircleID: "admin", DisplayName: "Admin", MattermostChannelName: "circle-admin"},
	{CircleID: "hr-compensation", DisplayName: "HR Compensation", MattermostChannelName: "circle-hr-compensation"},
}

func blueclawAgentProfiles(allowAdminTaskDiagnostic bool) []map[string]any {
	if !allowAdminTaskDiagnostic {
		return nil
	}
	return []map[string]any{{
		"name":             BlueclawSDKDTopologyDiagnosticProfileName,
		"allowedToolNames": []string{BlueclawSDKDTopologyDiagnosticToolSentinel},
	}}
}

func removeDefaultSkillScopedToolNames(toolNames []string) []string {
	hiddenToolNames := map[string]bool{
		"site.preview": true,
	}
	result := []string{}
	for _, toolName := range toolNames {
		if !hiddenToolNames[strings.TrimSpace(toolName)] {
			result = append(result, toolName)
		}
	}
	return result
}

func BlueclawRuntimeConfigDocument(modelName string) (string, error) {
	return BlueclawRuntimeConfigDocumentWithOptions(RuntimeConfigOptions{ModelName: modelName})
}

func BlueclawRuntimeConfigOptionsFromEnvironment() (RuntimeConfigOptions, error) {
	seed, errorValue := optionalInt64Environment(BlueclawTestGenerationSeedEnvironment)
	if errorValue != nil {
		return RuntimeConfigOptions{}, errorValue
	}
	temperature, errorValue := optionalFloat64Environment(BlueclawTestGenerationTemperatureEnvironment)
	if errorValue != nil {
		return RuntimeConfigOptions{}, errorValue
	}
	modelName := optionalStringEnvironment(BlueclawTestModelEnvironment)
	modelTier := optionalStringEnvironment(BlueclawTestModelTierEnvironment)
	maximumModelTier, errorValue := NormalizeMaximumModelTier(optionalStringEnvironment(BlueclawTestMaximumModelTierEnvironment))
	if errorValue != nil {
		return RuntimeConfigOptions{}, errorValue
	}
	virtualCPUCount, errorValue := optionalInt64Environment(BlueclawVirtualCPUCountEnvironment)
	if errorValue != nil {
		return RuntimeConfigOptions{}, errorValue
	}
	allowAdminTaskDiagnostic, errorValue := optionalBooleanEnvironment(BlueclawAdminTaskDiagnosticEnvironment)
	if errorValue != nil {
		return RuntimeConfigOptions{}, errorValue
	}
	options := RuntimeConfigOptions{
		ModelName:                 modelName,
		ShouldUseModelForAllTiers: modelName != "",
		DefaultTaskLevel:          modelTier,
		GenerationSeed:            seed,
		GenerationTemperature:     temperature,
		SDKDMode:                  optionalStringEnvironment(BlueclawSDKDModeEnvironment),
		AllowAdminTaskDiagnostic:  allowAdminTaskDiagnostic,
		LocalOnly:                 LocalOnlyEnabled(),
		MaximumModelTier:          maximumModelTier,
	}
	if virtualCPUCount != nil {
		options.VirtualCPUCount = int(*virtualCPUCount)
	}
	return options, nil
}

func BlueclawRuntimeConfigDocumentWithOptions(options RuntimeConfigOptions) (string, error) {
	languageModelExecutionMode := "auto"
	terminalMode := "firecrackerGuest"
	capabilityTransport := "vsock"
	if options.DirectExecution {
		languageModelExecutionMode = "remote"
		terminalMode = "native"
		capabilityTransport = ""
	}
	virtualCPUCount := BlueclawFirecrackerDefaultVirtualCPUCount
	if options.VirtualCPUCount > 0 {
		virtualCPUCount = options.VirtualCPUCount
	}

	capabilityLanguageModel := map[string]any{
		"executionMode":         languageModelExecutionMode,
		"model":                 BlueclawDefaultModelName,
		"contextWindowTokens":   BlueclawDefaultModelContextTokens,
		"requireParameters":     true,
		"enableResponseHealing": true,
	}
	if strings.TrimSpace(options.ModelName) != "" {
		modelName := strings.TrimSpace(options.ModelName)
		capabilityLanguageModel["model"] = modelName
		if options.ShouldUseModelForAllTiers {
			for _, tierModelField := range []string{"maxModel", "xhighModel", "highModel", "lowModel", "xlowModel", "codingModel"} {
				capabilityLanguageModel[tierModelField] = modelName
			}
			capabilityLanguageModel["mediumModel"] = BlueclawTestEscalationModelName
		}
	}
	if maximumModelTier := strings.TrimSpace(options.MaximumModelTier); maximumModelTier != "" {
		capabilityLanguageModel["maximumModelTier"] = maximumModelTier
	}

	capabilityVSockPort := firstPositiveInt(options.CapabilityVSockPort, CapabilityVSockPort)
	capabilitySocketPath := firstNonEmptyString(options.CapabilitySocketPath, CapabilitySocketPath)
	capabilityUnixSocketPath := ""
	if options.DirectExecution {
		capabilityUnixSocketPath = capabilitySocketPath
	}
	terminalWorkspaceRootPath := firstNonEmptyString(options.WorkspaceRootPath, BlueclawGuestWorkspacePath)
	terminalPOSIXHelperPath := firstNonEmptyString(options.POSIXHelperPath, BlueclawPOSIXHelperPath)
	if options.DirectExecution {
		terminalPOSIXHelperPath = strings.TrimSpace(options.POSIXHelperPath)
	}
	databaseConnectionString := firstNonEmptyString(options.DatabaseConnectionString, BlueclawGuestDatabaseConnectionString)
	migrationDirectoryPath := firstNonEmptyString(options.MigrationDirectoryPath, BlueclawGuestMigrationPath)
	graphitiEndpoint := firstNonEmptyString(options.GraphitiEndpoint, GraphitiEndpoint)
	if options.DirectExecution && strings.TrimSpace(options.GraphitiEndpoint) == "" {
		graphitiEndpoint = ""
	}
	mattermostBaseURL := firstNonEmptyString(options.MattermostBaseURL, "http://localhost:8065")
	hostWorkspacePath := firstNonEmptyString(options.HostWorkspacePath, BlueclawWorkspacePath)
	rootFilesystemImagePath := firstNonEmptyString(options.RootFilesystemImagePath, BlueclawRootFilesystemImagePath)
	workspaceImagePath := firstNonEmptyString(options.WorkspaceImagePath, BlueclawWorkspaceImagePath)
	hostHTTPListenAddress := firstNonEmptyString(options.HostHTTPListenAddress, "127.0.0.1:8080")
	healthPortOrService := firstNonEmptyString(options.HealthPortOrService, "8082")
	guestHTTPPortOrService := firstNonEmptyString(options.GuestHTTPPortOrService, "8081")
	logDirectoryPath := firstNonEmptyString(options.LogDirectoryPath, BlueclawSupervisorLogDirectoryPath)
	runtimeDirectoryPath := firstNonEmptyString(options.RuntimeDirectoryPath, "/var/lib/bc")
	outboundHostDeviceName := firstNonEmptyString(options.OutboundHostDeviceName, "bctap0")
	outboundGuestMACAddress := firstNonEmptyString(options.OutboundGuestMACAddress, "AA:FC:00:00:00:01")
	outboundNetworkCIDR := firstNonEmptyString(options.OutboundNetworkCIDR, "172.31.0.0/30")
	outboundHostAddressCIDR := firstNonEmptyString(options.OutboundHostAddressCIDR, "172.31.0.1/30")
	outboundGuestAddressCIDR := firstNonEmptyString(options.OutboundGuestAddressCIDR, "172.31.0.2/30")
	outboundGuestGateway := firstNonEmptyString(options.OutboundGuestGateway, "172.31.0.1")
	bridgeListenAddress := firstNonEmptyString(options.BridgeListenAddress, BlueclawBridgeListenAddress)
	agentConfiguration := map[string]any{
		"adminTaskLinkBaseURL":     strings.TrimRight(strings.TrimSpace(options.AdminTaskLinkBaseURL), "/"),
		"allowAdminTaskDiagnostic": options.AllowAdminTaskDiagnostic,
		"intake": map[string]any{
			"enabled":       true,
			"executionMode": "auto",
		},
		"defaultTaskLevel":    firstNonEmptyString(options.DefaultTaskLevel, "low"),
		"skillTaskLevelFloor": "high",
		"toolResultMaxBytes":  32768,
		"failureRecovery": map[string]any{
			"failureDebtFinalizationGate": true,
			"attemptFingerprint":          "tool_input_error_code",
			"recoveryBudget": map[string]any{
				"correctedRetry": 1,
				"alternateRoute": 1,
				"adjacentTool":   2,
				"noToolFallback": 1,
			},
		},
	}
	generationOptions := map[string]any{}
	if options.GenerationSeed != nil {
		generationOptions["seed"] = *options.GenerationSeed
	}
	if options.GenerationTemperature != nil {
		generationOptions["temperature"] = *options.GenerationTemperature
	}
	if len(generationOptions) > 0 {
		agentConfiguration["generationOptions"] = generationOptions
	}
	languageModelConfiguration := map[string]any{
		"defaultProvider":  "capabilityLLM",
		"fallbackProvider": "",
		"capability":       capabilityLanguageModel,
	}
	if !options.DirectExecution {
		languageModelConfiguration["sdkd"] = map[string]any{
			"endpoint":              "http://127.0.0.1:18081/_internkim/sdkd",
			"unixSocketPath":        "",
			"authKeyPath":           "",
			"executionMode":         languageModelExecutionMode,
			"localOnly":             options.LocalOnly,
			"shadowEnabled":         strings.EqualFold(options.SDKDMode, "shadow"),
			"structuredSchemaNames": []string{"blueclaw_agent_turn_action", "blueclaw_agent_turn_finalizer", "blueclaw_turn_router", "blueclaw_recovery_decision"},
		}
		if strings.EqualFold(options.SDKDMode, "authoritative") {
			languageModelConfiguration["defaultProvider"] = "sdkd"
		}
	}

	document := map[string]any{
		"baseURL": firstNonEmptyString(options.BaseURL, BlueclawBaseURL),
		"capabilities": map[string]any{
			"transport":       capabilityTransport,
			"unixSocketPath":  capabilityUnixSocketPath,
			"endpoint":        "http://internkim-capability",
			"timeoutSecond":   BlueclawCapabilityTimeoutSecond,
			"vsockCID":        CapabilityVSockHostCID,
			"vsockPort":       capabilityVSockPort,
			"toolDescriptors": capabilities.DefaultToolDescriptors(),
			"routing": map[string]any{
				"candidates": capabilities.RoutingCandidates(),
				"localOnly":  options.LocalOnly,
			},
		},
		"languageModel": languageModelConfiguration,
		"firecracker": map[string]any{
			"firecrackerPath":        BlueclawFirecrackerPath,
			"jailerPath":             BlueclawJailerPath,
			"kernelImagePath":        BlueclawKernelImagePath,
			"rootfsImagePath":        rootFilesystemImagePath,
			"workspaceImagePath":     workspaceImagePath,
			"hostWorkspacePath":      hostWorkspacePath,
			"vcpuCount":              virtualCPUCount,
			"memoryMiB":              BlueclawFirecrackerDefaultMemoryMiB,
			"vsockCID":               52,
			"healthPortOrService":    healthPortOrService,
			"guestHTTPPortOrService": guestHTTPPortOrService,
			"hostHTTPListenAddress":  hostHTTPListenAddress,
			"logDirectoryPath":       logDirectoryPath,
			"runtimeDirectoryPath":   runtimeDirectoryPath,
			"outboundNetwork": map[string]any{
				"enabled":          true,
				"hostDeviceName":   outboundHostDeviceName,
				"guestMACAddress":  outboundGuestMACAddress,
				"networkCIDR":      outboundNetworkCIDR,
				"hostAddressCIDR":  outboundHostAddressCIDR,
				"guestAddressCIDR": outboundGuestAddressCIDR,
				"guestGateway":     outboundGuestGateway,
			},
			"guestListenerProxies": []map[string]any{
				{
					"guestPort":            capabilityVSockPort,
					"targetUnixSocketPath": capabilitySocketPath,
				},
			},
		},
		"bridge": map[string]any{
			"mode":                     "localAgent",
			"authMode":                 "sshKeyReuse",
			"authorizedPublicKeysPath": BlueclawBridgeAuthorizedKeysPath,
			"listenAddress":            bridgeListenAddress,
		},
		"database": map[string]any{
			"driver":                 "postgres",
			"connectionString":       databaseConnectionString,
			"migrationDirectoryPath": migrationDirectoryPath,
		},
		"memory": map[string]any{
			"workspaceID":                                 "default",
			"graphitiEndpoint":                            graphitiEndpoint,
			"graphitiKuzuPath":                            path.Join(BlueclawGuestWorkspacePath, ".blueclaw", "graphiti", "kuzu"),
			"pinnedMemoryRootPath":                        path.Join(BlueclawGuestWorkspacePath, ".blueclaw", "memory"),
			"pinnedMemoryHardLimitCharacterCount":         BlueclawPinnedMemoryHardLimitCharacterCount,
			"pinnedMemoryCompressionTargetCharacterCount": BlueclawPinnedMemoryCompressionTargetCharacterCount,
			"timeoutSecond":                               60,
		},
		"agent": agentConfiguration,
		"connectors": map[string]any{
			"mattermost": map[string]any{
				"baseURL": mattermostBaseURL,
			},
			"slack": map[string]any{
				"baseURL": BlueclawSlackAPIBaseURL,
			},
		},
		"agentProfiles": blueclawAgentProfiles(options.AllowAdminTaskDiagnostic),
		"mcpServers":    []map[string]any{},
		"terminal": map[string]any{
			"mode":              terminalMode,
			"sandboxProvider":   "",
			"workspaceRootPath": terminalWorkspaceRootPath,
			"posixHelperPath":   terminalPOSIXHelperPath,
			"requesterWorkspace": map[string]any{
				"requesterTemporaryEnvironmentVariable": "BLUECLAW_REQUESTER_TMP",
				"taskTemporaryEnvironmentVariable":      "BLUECLAW_TASK_TMP",
				"requesterArtifactsEnvironmentVariable": "BLUECLAW_REQUESTER_ARTIFACTS",
				"requesterTemporaryDirectoryTemplate":   "/workspace/private/people/{personID}/tmp",
				"taskTemporaryDirectoryTemplate":        "/workspace/private/people/{personID}/tmp/{taskID}",
				"requesterArtifactsDirectoryTemplate":   "/workspace/private/people/{personID}/artifacts",
			},
			"timeoutSecond":         600,
			"outputMaxBytes":        32768,
			"sessionMaxCount":       4,
			"allowNetwork":          true,
			"allowInteractiveShell": true,
		},
		"scheduler": map[string]any{
			"retentionCheckIntervalMinute":   60,
			"taskSchedulePollIntervalSecond": 30,
		},
	}

	documentBytes, error := json.MarshalIndent(document, "", "  ")
	if error != nil {
		return "", error
	}

	return string(documentBytes) + "\n", nil
}

func NormalizeMaximumModelTier(modelTier string) (string, error) {
	normalizedModelTier := strings.ToLower(strings.TrimSpace(modelTier))
	if normalizedModelTier == "" {
		return "", nil
	}
	for _, supportedModelTier := range []string{"xlow", "low", "medium", "high", "xhigh", "max"} {
		if normalizedModelTier == supportedModelTier {
			return normalizedModelTier, nil
		}
	}
	return "", fmt.Errorf("maximum model tier must be xlow, low, medium, high, xhigh, or max: %s", modelTier)
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		trimmedValue := strings.TrimSpace(value)
		if trimmedValue != "" {
			return trimmedValue
		}
	}
	return ""
}

func firstPositiveInt(values ...int) int {
	for _, value := range values {
		if value > 0 {
			return value
		}
	}
	return 0
}

func optionalStringEnvironment(name string) string {
	return strings.TrimSpace(os.Getenv(name))
}

func optionalBooleanEnvironment(name string) (bool, error) {
	value := optionalStringEnvironment(name)
	if value == "" {
		return false, nil
	}
	parsedValue, errorValue := strconv.ParseBool(value)
	if errorValue != nil {
		return false, fmt.Errorf("%s must be a boolean: %w", name, errorValue)
	}
	return parsedValue, nil
}

func LocalOnlyEnabled() bool {
	value := optionalStringEnvironment(LocalOnlyEnvironment)
	if value == "" {
		return false
	}
	localOnly, errorValue := strconv.ParseBool(value)
	return errorValue != nil || localOnly
}

func optionalInt64Environment(name string) (*int64, error) {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return nil, nil
	}
	parsedValue, errorValue := strconv.ParseInt(value, 10, 64)
	if errorValue != nil {
		return nil, fmt.Errorf("%s must be an int64: %w", name, errorValue)
	}
	return &parsedValue, nil
}

func optionalFloat64Environment(name string) (*float64, error) {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return nil, nil
	}
	parsedValue, errorValue := strconv.ParseFloat(value, 64)
	if errorValue != nil {
		return nil, fmt.Errorf("%s must be a float64: %w", name, errorValue)
	}
	if math.IsNaN(parsedValue) || math.IsInf(parsedValue, 0) || parsedValue < 0 {
		return nil, fmt.Errorf("%s must be 0 or greater", name)
	}
	return &parsedValue, nil
}

func uniqueStringList(values []string) []string {
	seenValues := map[string]bool{}
	uniqueValues := []string{}
	for _, value := range values {
		trimmedValue := strings.TrimSpace(value)
		if trimmedValue == "" || seenValues[trimmedValue] {
			continue
		}
		seenValues[trimmedValue] = true
		uniqueValues = append(uniqueValues, trimmedValue)
	}
	return uniqueValues
}

func BlueclawPolicyDocument(adminEmail string) (string, error) {
	if adminEmail == "" {
		adminEmail = "admin@intern.kim"
	}

	document := map[string]any{
		"people": []map[string]any{
			{
				"personID":          BlueclawPolicyAdminID,
				"displayName":       "Intern Kim Admin",
				"emails":            []string{adminEmail},
				"circles":           []string{"staff", "admin"},
				"securityLevelName": "admin",
				"securityLevelRank": 100,
				"grantedClasses":    []string{"internal", "executive"},
				"isAdmin":           true,
			},
		},
		"circles": defaultCirclePolicies(),
		"circleSync": map[string]any{
			"mattermostPrivateChannels": defaultMattermostCircleChannelPolicies(),
		},
		"channels":       []map[string]any{},
		"resourceAccess": defaultResourceAccessPolicies(),
		"retention":      map[string]any{"rawEventDays": 60},
	}

	documentBytes, error := json.MarshalIndent(document, "", "  ")
	if error != nil {
		return "", error
	}

	return string(documentBytes) + "\n", nil
}

func defaultCirclePolicy(circleID string, displayName string) map[string]any {
	return map[string]any{
		"circleID":               circleID,
		"displayName":            displayName,
		"isMattermostManaged":    circleID != "staff",
		"workspaceDirectoryPath": "/workspace/circles/" + circleID,
	}
}

func defaultCirclePolicies() []map[string]any {
	circles := make([]map[string]any, 0, len(defaultCircleDefinitions))
	for _, circleDefinition := range defaultCircleDefinitions {
		circles = append(circles, defaultCirclePolicy(circleDefinition.CircleID, circleDefinition.DisplayName))
	}
	return circles
}

func defaultMattermostCircleChannelPolicies() []map[string]any {
	channels := []map[string]any{}
	for _, circleDefinition := range defaultCircleDefinitions {
		if strings.TrimSpace(circleDefinition.MattermostChannelName) == "" {
			continue
		}
		channels = append(channels, map[string]any{
			"circleID":    circleDefinition.CircleID,
			"channelName": circleDefinition.MattermostChannelName,
		})
	}
	return channels
}

func defaultResourceAccessPolicies() []map[string]any {
	policies := []map[string]any{}
	for _, circleDefinition := range defaultCircleDefinitions {
		actions := []string{"read", "write"}
		if circleDefinition.CircleID == "admin" {
			actions = append(actions, "manage")
		}
		policies = append(policies, map[string]any{
			"resource": "file:circle:" + circleDefinition.CircleID,
			"actions":  actions,
			"circles":  []string{circleDefinition.CircleID},
		})
	}
	return append(policies, []map[string]any{
		{"resource": "api:flow.summary", "actions": []string{"read"}, "circles": []string{"staff"}},
		{"resource": "api:flow.task", "actions": []string{"create", "update"}, "circles": []string{"staff"}},
		{"resource": "api:flow.definition", "actions": []string{"manage"}, "circles": []string{"admin"}},
		{"resource": "api:credentials.providers", "actions": []string{"manage"}, "circles": []string{"admin"}},
		{"resource": "tool:web.search", "actions": []string{"execute"}, "circles": []string{"staff"}},
		{"resource": "tool:web.fetch", "actions": []string{"execute"}, "circles": []string{"staff"}},
		{"resource": "tool:artifact.review", "actions": []string{"execute"}, "circles": []string{"staff"}},
		{"resource": "tool:task.add", "actions": []string{"execute"}, "circles": []string{"staff"}},
		{"resource": "tool:task.list", "actions": []string{"execute"}, "circles": []string{"staff"}},
		{"resource": "tool:task.update", "actions": []string{"execute"}, "circles": []string{"staff"}},
		{"resource": "tool:message.context", "actions": []string{"execute"}, "circles": []string{"staff"}},
		{"resource": "tool:message.search", "actions": []string{"execute"}, "circles": []string{"staff"}},
		{"resource": "tool:message.send", "actions": []string{"execute"}, "circles": []string{"staff"}},
		{"resource": "tool:message.update", "actions": []string{"execute"}, "circles": []string{"staff"}},
		{"resource": "tool:message.delete", "actions": []string{"execute"}, "circles": []string{"staff"}},
		{"resource": "tool:channel.update", "actions": []string{"execute"}, "circles": []string{"admin"}},
		{"resource": "tool:mail.message.list", "actions": []string{"execute"}, "circles": []string{"staff"}},
		{"resource": "tool:mail.message.search", "actions": []string{"execute"}, "circles": []string{"staff"}},
		{"resource": "tool:mail.message.read", "actions": []string{"execute"}, "circles": []string{"staff"}},
		{"resource": "tool:mail.message.send", "actions": []string{"execute"}, "circles": []string{"staff"}},
		{"resource": "tool:mail.message.move", "actions": []string{"execute"}, "circles": []string{"staff"}},
		{"resource": "tool:mail.message.mark", "actions": []string{"execute"}, "circles": []string{"staff"}},
		{"resource": "tool:site.create", "actions": []string{"execute"}, "circles": []string{"staff"}},
		{"resource": "tool:site.preview", "actions": []string{"execute"}, "circles": []string{"staff"}},
		{"resource": "tool:site.publish", "actions": []string{"execute"}, "circles": []string{"staff"}},
		{"resource": "tool:site.status", "actions": []string{"execute"}, "circles": []string{"staff"}},
		{"resource": "tool:site.history", "actions": []string{"execute"}, "circles": []string{"staff"}},
		{"resource": "tool:site.diff", "actions": []string{"execute"}, "circles": []string{"staff"}},
		{"resource": "tool:site.logs", "actions": []string{"execute"}, "circles": []string{"staff"}},
		{"resource": "tool:site.restore", "actions": []string{"execute"}, "circles": []string{"staff"}},
		{"resource": "tool:site.repair", "actions": []string{"execute"}, "circles": []string{"staff"}},
		{"resource": "tool:site.rollback", "actions": []string{"execute"}, "circles": []string{"staff"}},
		{"resource": "tool:site.unpublish", "actions": []string{"execute"}, "circles": []string{"staff"}},
		{"resource": "tool:site.delete", "actions": []string{"execute"}, "circles": []string{"staff"}},
		{"resource": "tool:company.broadcast.send", "actions": []string{"execute"}, "circles": []string{"representative"}},
	}...)
}
