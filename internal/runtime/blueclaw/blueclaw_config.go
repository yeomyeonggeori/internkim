package blueclaw

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"strconv"
	"strings"

	"gitlab.com/eastriver/internkim/internal/modelladder"
)

const (
	BlueclawCapabilityTimeoutSecond              = 0
	BlueclawGuestDefaultVirtualCPUCount          = 2
	BlueclawGuestDefaultMemoryMiB                = 4096
	BlueclawTestEscalationModelName              = modelladder.PrimaryModel
	BlueclawTestModelEnvironment                 = "INTERNKIM_TEST_MODEL"
	BlueclawTestModelTierEnvironment             = "INTERNKIM_TEST_MODEL_TIER"
	BlueclawTestMaximumModelTierEnvironment      = "INTERNKIM_TEST_MAXIMUM_MODEL_TIER"
	BlueclawTestMinimumModelTierEnvironment      = "INTERNKIM_TEST_MINIMUM_MODEL_TIER"
	BlueclawTestGenerationSeedEnvironment        = "INTERNKIM_TEST_GENERATION_SEED"
	BlueclawTestGenerationTemperatureEnvironment = "INTERNKIM_TEST_GENERATION_TEMPERATURE"
	BlueclawAdminTaskDiagnosticEnvironment       = "INTERNKIM_BLUECLAW_ADMIN_TASK_DIAGNOSTIC"
	BlueclawModelPathDiagnosticProfileName       = "model-path-diagnostic"
	BlueclawModelPathDiagnosticToolSentinel      = "model_path.diagnostic.no_tools"
	LocalOnlyEnvironment                         = "INTERNKIM_LOCAL_ONLY"
	BlueclawVirtualCPUCountEnvironment           = "INTERNKIM_BLUECLAW_VCPU_COUNT"
)

type defaultCircleDefinition struct {
	CircleID    string
	DisplayName string
}

type RuntimeConfigOptions struct {
	ModelName                  string
	AdminTaskLinkBaseURL       string
	BaseURL                    string
	DirectExecution            bool
	WorkspaceRootPath          string
	POSIXHelperPath            string
	DatabaseConnectionString   string
	MigrationDirectoryPath     string
	CapabilitySocketPath       string
	CapabilityVSockPort        int
	AdminAssertionKeyPath      string
	HostWorkspacePath          string
	RootFilesystemImagePath    string
	WorkspaceImagePath         string
	HostHTTPListenAddress      string
	HealthPortOrService        string
	GuestHTTPPortOrService     string
	LogDirectoryPath           string
	RuntimeDirectoryPath       string
	OutboundHostDeviceName     string
	OutboundGuestMACAddress    string
	OutboundNetworkCIDR        string
	OutboundHostAddressCIDR    string
	OutboundGuestAddressCIDR   string
	OutboundGuestGateway       string
	BridgeListenAddress        string
	GenerationSeed             *int64
	GenerationTemperature      *float64
	MaximumModelTier           string
	MinimumModelTier           string
	ShouldUseModelForAllTiers  bool
	DefaultTaskLevel           string
	VirtualMachineMonitor      string
	KernelImagePath            string
	VfkitPath                  string
	DeliveryDirectoryPath      string
	WorkspaceMinimumBytes      int64
	HostRunsNoCapabilityDaemon bool
	VirtualCPUCount            int
	AllowAdminTaskDiagnostic   bool
	LocalOnly                  bool
}

var defaultCircleDefinitions = []defaultCircleDefinition{
	{CircleID: "member", DisplayName: "Member"},
	{CircleID: "c-level", DisplayName: "C-level"},
	{CircleID: "representative", DisplayName: "Representative"},
	{CircleID: "admin", DisplayName: "Admin"},
	{CircleID: "hr", DisplayName: "HR"},
}

func blueclawAgentProfiles(allowAdminTaskDiagnostic bool) []map[string]any {
	if !allowAdminTaskDiagnostic {
		return nil
	}
	return []map[string]any{{
		"name":             BlueclawModelPathDiagnosticProfileName,
		"allowedToolNames": []string{BlueclawModelPathDiagnosticToolSentinel},
	}}
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
	minimumModelTier, errorValue := NormalizeMaximumModelTier(optionalStringEnvironment(BlueclawTestMinimumModelTierEnvironment))
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
		AllowAdminTaskDiagnostic:  allowAdminTaskDiagnostic,
		LocalOnly:                 LocalOnlyEnabled(),
		MaximumModelTier:          maximumModelTier,
		MinimumModelTier:          minimumModelTier,
	}
	if virtualCPUCount != nil {
		options.VirtualCPUCount = int(*virtualCPUCount)
	}
	return options, nil
}

func StampLadderOwnedModels(capability map[string]any) {
	capability["decisionModel"] = modelladder.DecisionModel
}

func BlueclawRuntimeConfigDocumentWithOptions(options RuntimeConfigOptions) (string, error) {
	languageModelExecutionMode := "auto"
	terminalMode := "virtualMachineGuest"
	capabilityTransport := "vsock"
	if options.DirectExecution {
		languageModelExecutionMode = "remote"
		terminalMode = "native"
		capabilityTransport = ""
	}
	virtualCPUCount := BlueclawGuestDefaultVirtualCPUCount
	if options.VirtualCPUCount > 0 {
		virtualCPUCount = options.VirtualCPUCount
	}

	capabilityLanguageModel := map[string]any{
		"executionMode": languageModelExecutionMode,
		"model":         BlueclawDefaultModelName,
	}
	for tier, modelName := range modelladder.TierModelNames() {
		capabilityLanguageModel[tier+"Model"] = modelName
	}
	StampLadderOwnedModels(capabilityLanguageModel)
	if strings.TrimSpace(options.ModelName) != "" {
		modelName := strings.TrimSpace(options.ModelName)
		capabilityLanguageModel["model"] = modelName
		if options.ShouldUseModelForAllTiers {
			for _, tierModelField := range []string{"maxModel", "xhighModel", "highModel", "lowModel", "xlowModel"} {
				capabilityLanguageModel[tierModelField] = modelName
			}
			capabilityLanguageModel["mediumModel"] = BlueclawTestEscalationModelName
		}
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
	virtualMachineMonitor := firstNonEmptyString(options.VirtualMachineMonitor, BlueclawVirtualMachineMonitor)
	databaseConnectionString := firstNonEmptyString(options.DatabaseConnectionString, BlueclawGuestDatabaseConnectionString)
	migrationDirectoryPath := firstNonEmptyString(options.MigrationDirectoryPath, guestMigrationDirectoryPath(virtualMachineMonitor))
	adminAssertionKeyPath := strings.TrimSpace(options.AdminAssertionKeyPath)
	if adminAssertionKeyPath == "" {
		if options.DirectExecution {
			adminAssertionKeyPath = InternKimCentralPlaneAgentKeyPath
		} else {
			adminAssertionKeyPath = BlueclawGuestDeliverySecretsPath + "/" + BlueclawAdminAssertionKeyName
		}
	}
	hostWorkspacePath := firstNonEmptyString(options.HostWorkspacePath, BlueclawWorkspacePath)
	rootFilesystemImagePath := firstNonEmptyString(options.RootFilesystemImagePath, BlueclawRootFilesystemImagePath)
	workspaceImagePath := firstNonEmptyString(options.WorkspaceImagePath, BlueclawWorkspaceImagePath)
	hostHTTPListenAddress := firstNonEmptyString(options.HostHTTPListenAddress, "127.0.0.1:8080")
	healthPortOrService := firstNonEmptyString(options.HealthPortOrService, "8082")
	guestHTTPPortOrService := firstNonEmptyString(options.GuestHTTPPortOrService, "8081")
	logDirectoryPath := firstNonEmptyString(options.LogDirectoryPath, BlueclawSupervisorLogDirectoryPath)
	runtimeDirectoryPath := firstNonEmptyString(options.RuntimeDirectoryPath, "/var/lib/bc")
	deliveryDirectoryPath := firstNonEmptyString(options.DeliveryDirectoryPath, deliveryDirectoryPathForMonitor(virtualMachineMonitor))
	kernelImagePath := firstNonEmptyString(options.KernelImagePath, BlueclawKernelImagePath)
	vfkitPath := firstNonEmptyString(options.VfkitPath, BlueclawVfkitPath)
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
		"defaultTaskLevel": firstNonEmptyString(options.DefaultTaskLevel, "low"),
		"optionalFileReadPathSuffixes": []string{
			".internkim/site.json",
			".internkim/idea.md",
			".internkim/artifact-brief.md",
			".internkim/review-log.json",
		},
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
		"contextWindowTokens": BlueclawDefaultModelContextTokens,
		"embedding":           map[string]any{"model": modelladder.EmbeddingModel},
		"capability":          capabilityLanguageModel,
	}
	if maximumModelTier := strings.TrimSpace(options.MaximumModelTier); maximumModelTier != "" {
		languageModelConfiguration["maximumModelTier"] = maximumModelTier
	}
	if minimumModelTier := strings.TrimSpace(options.MinimumModelTier); minimumModelTier != "" {
		languageModelConfiguration["minimumModelTier"] = minimumModelTier
	}
	capabilityContract := CurrentCapabilityContract()

	document := map[string]any{
		"baseURL": firstNonEmptyString(options.BaseURL, BlueclawBaseURL),
		"capabilities": map[string]any{
			"transport":             capabilityTransport,
			"unixSocketPath":        capabilityUnixSocketPath,
			"endpoint":              "http://internkim-capability",
			"timeoutSecond":         BlueclawCapabilityTimeoutSecond,
			"vsockCID":              CapabilityVSockHostCID,
			"vsockPort":             capabilityVSockPort,
			"protocolVersion":       capabilityContract.ProtocolVersion,
			"aggregateProtocolHash": capabilityContract.AggregateProtocolHash,
			"toolDescriptors":       capabilityContract.ToolDescriptors,
			"routing": map[string]any{
				"candidates": capabilityContract.RoutingCandidates,
				"localOnly":  options.LocalOnly,
			},
		},
		"languageModel": languageModelConfiguration,
		"guest": map[string]any{
			"virtualMachineMonitor":       virtualMachineMonitor,
			"cloudHypervisorPath":         BlueclawCloudHypervisorPath,
			"virtiofsdPath":               BlueclawVirtiofsdPath,
			"vfkitPath":                   vfkitPath,
			"deliveryDirectoryPath":       deliveryDirectoryPath,
			"deliveryReadOnlyEnforcement": deliveryReadOnlyEnforcementForMonitor(virtualMachineMonitor),
			"kernelImagePath":             kernelImagePath,
			"rootfsImagePath":             rootFilesystemImagePath,
			"workspaceImagePath":          workspaceImagePath,
			"workspaceMinimumBytes":       options.WorkspaceMinimumBytes,
			"hostWorkspacePath":           hostWorkspacePath,
			"vcpuCount":                   virtualCPUCount,
			"memoryMiB":                   BlueclawGuestDefaultMemoryMiB,
			"vsockCID":                    52,
			"healthPortOrService":         healthPortOrService,
			"guestHTTPPortOrService":      guestHTTPPortOrService,
			"hostHTTPListenAddress":       hostHTTPListenAddress,
			"logDirectoryPath":            logDirectoryPath,
			"runtimeDirectoryPath":        runtimeDirectoryPath,
			"outboundNetwork": map[string]any{
				"enabled":          monitorReachesTheNetworkThroughAHostTap(virtualMachineMonitor),
				"hostDeviceName":   outboundHostDeviceName,
				"guestMACAddress":  outboundGuestMACAddress,
				"networkCIDR":      outboundNetworkCIDR,
				"hostAddressCIDR":  outboundHostAddressCIDR,
				"guestAddressCIDR": outboundGuestAddressCIDR,
				"guestGateway":     outboundGuestGateway,
			},
			"guestListenerProxies": guestListenerProxiesFor(options.HostRunsNoCapabilityDaemon, capabilityVSockPort, capabilitySocketPath),
		},
		"bridge": map[string]any{
			"mode":                     "localAgent",
			"authMode":                 "sshKeyReuse",
			"authorizedPublicKeysPath": BlueclawBridgeAuthorizedKeysPath,
			"listenAddress":            bridgeListenAddress,
		},
		"database": map[string]any{
			"driver":                      "postgres",
			"connectionString":            databaseConnectionString,
			"migrationDirectoryPath":      migrationDirectoryPath,
			AgentDatabaseConnectionsField: AgentDatabaseConnections,
		},
		"memory": map[string]any{
			"adminAssertionKeyPath":  adminAssertionKeyPath,
			"embeddingModel":         modelladder.EmbeddingModel,
			"embeddingExecutionMode": "auto",
			"extractionDisabled":     false,
		},
		"agent": agentConfiguration,
		"connectors": map[string]any{
			// The agent runs in the guest, where the loopback it would otherwise
			// fall back to is its own, so chatd is named at the address the guest
			// reaches this machine on.
			"chatd": map[string]any{
				"endpoint":         "http://" + outboundGuestGateway + ":" + ChatdListenPort,
				"enabledPlatforms": []string{BlueclawMessengerPlatform},
			},
		},
		"agentProfiles": blueclawAgentProfiles(options.AllowAdminTaskDiagnostic),
		"terminal": map[string]any{
			"mode":                  terminalMode,
			"sandboxProvider":       "",
			"workspaceRootPath":     terminalWorkspaceRootPath,
			"posixHelperPath":       terminalPOSIXHelperPath,
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

func BlueclawPolicyDocument(adminEmail string) (string, error) {
	if adminEmail == "" {
		adminEmail = "admin@example.test"
	}

	document := map[string]any{
		"people": []map[string]any{
			{
				"personID":          BlueclawPolicyAdminID,
				"displayName":       "Intern Kim Admin",
				"emails":            []string{adminEmail},
				"circles":           []string{"member", "admin"},
				"securityLevelName": "admin",
				"securityLevelRank": 100,
				"grantedClasses":    []string{"internal", "executive"},
				"isAdmin":           true,
			},
		},
		"circles":        defaultCirclePolicies(),
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
		{"resource": "api:flow.summary", "actions": []string{"read"}, "circles": []string{"member"}},
		{"resource": "api:flow.task", "actions": []string{"create", "update"}, "circles": []string{"member"}},
		{"resource": "api:flow.definition", "actions": []string{"manage"}, "circles": []string{"admin"}},
		{"resource": "api:credentials.providers", "actions": []string{"manage"}, "circles": []string{"admin"}},
		{"resource": "tool:web_search", "actions": []string{"execute"}, "circles": []string{"member"}},
		{"resource": "tool:web_fetch", "actions": []string{"execute"}, "circles": []string{"member"}},
		{"resource": "tool:artifact_review", "actions": []string{"execute"}, "circles": []string{"member"}},
		{"resource": "tool:task_add", "actions": []string{"execute"}, "circles": []string{"member"}},
		{"resource": "tool:task_list", "actions": []string{"execute"}, "circles": []string{"member"}},
		{"resource": "tool:task_update", "actions": []string{"execute"}, "circles": []string{"member"}},
		{"resource": "tool:attendance_list", "actions": []string{"execute"}, "circles": []string{"member"}},
		{"resource": "tool:attendance_add", "actions": []string{"execute"}, "circles": []string{"member"}},
		{"resource": "tool:attendance_update", "actions": []string{"execute"}, "circles": []string{"member"}},
		{"resource": "tool:attendance_delete", "actions": []string{"execute"}, "circles": []string{"member"}},
		{"resource": "tool:message_context", "actions": []string{"execute"}, "circles": []string{"member"}},
		{"resource": "tool:message_search", "actions": []string{"execute"}, "circles": []string{"member"}},
		{"resource": "tool:message_send", "actions": []string{"execute"}, "circles": []string{"member"}},
		{"resource": "tool:message_update", "actions": []string{"execute"}, "circles": []string{"member"}},
		{"resource": "tool:message_delete", "actions": []string{"execute"}, "circles": []string{"member"}},
		{"resource": "tool:mail_message_list", "actions": []string{"execute"}, "circles": []string{"member"}},
		{"resource": "tool:mail_message_search", "actions": []string{"execute"}, "circles": []string{"member"}},
		{"resource": "tool:mail_message_read", "actions": []string{"execute"}, "circles": []string{"member"}},
		{"resource": "tool:mail_message_send", "actions": []string{"execute"}, "circles": []string{"member"}},
		{"resource": "tool:mail_message_move", "actions": []string{"execute"}, "circles": []string{"member"}},
		{"resource": "tool:mail_message_mark", "actions": []string{"execute"}, "circles": []string{"member"}},
		{"resource": "tool:site_serve", "actions": []string{"execute"}, "circles": []string{"member"}},
		{"resource": "tool:site_list", "actions": []string{"execute"}, "circles": []string{"member"}},
		{"resource": "tool:site_unserve", "actions": []string{"execute"}, "circles": []string{"member"}},
		{"resource": "tool:company.broadcast.send", "actions": []string{"execute"}, "circles": []string{"representative"}},
	}...)
}

// The proxy exists to carry the guest's capability calls to capabilityd's socket. A host
// running no capabilityd has no socket, and a proxy to one reports a failure every boot for
// work nobody asked for.
func guestListenerProxiesFor(hostRunsNoCapabilityDaemon bool, capabilityVSockPort int, capabilitySocketPath string) []map[string]any {
	if hostRunsNoCapabilityDaemon {
		return []map[string]any{}
	}
	return []map[string]any{
		{
			"guestPort":            capabilityVSockPort,
			"targetUnixSocketPath": capabilitySocketPath,
		},
	}
}

func deliveryReadOnlyEnforcementForMonitor(virtualMachineMonitor string) string {
	if virtualMachineMonitor == VfkitMonitorName {
		return "immutableFlags"
	}
	return "hostBindMount"
}

func deliveryDirectoryPathForMonitor(virtualMachineMonitor string) string {
	if virtualMachineMonitor == VfkitMonitorName {
		return BlueclawDeliveryPath
	}
	return BlueclawDeliveryReadOnlyPath
}

// The migrations travel with the payload, so they are wherever it is.
func guestMigrationDirectoryPath(virtualMachineMonitor string) string {
	if deliveryDirectoryPathForMonitor(virtualMachineMonitor) == "" {
		return BlueclawGuestMigrationPath
	}
	return BlueclawGuestDeliveryRuntimePath + "/migrations"
}

func monitorReachesTheNetworkThroughAHostTap(virtualMachineMonitor string) bool {
	return virtualMachineMonitor != VfkitMonitorName
}
