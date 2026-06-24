package blueclaw

import (
	"encoding/json"
	"path"
	"strings"

	"gitlab.com/eastriver/internkim/internal/capabilities"
)

const (
	BlueclawCapabilityTimeoutSecond                     = 0
	BlueclawPinnedMemoryHardLimitCharacterCount         = 6000
	BlueclawPinnedMemoryCompressionTargetCharacterCount = 3500
	BlueclawFirecrackerDefaultVirtualCPUCount           = 2
	BlueclawFirecrackerDefaultMemoryMiB                 = 4096
)

type defaultCircleDefinition struct {
	CircleID              string
	DisplayName           string
	MattermostChannelName string
}

type RuntimeConfigOptions struct {
	ModelName                string
	AdminTaskLinkBaseURL     string
	BaseURL                  string
	DirectExecution          bool
	WorkspaceRootPath        string
	POSIXHelperPath          string
	DatabaseConnectionString string
	MigrationDirectoryPath   string
	CapabilitySocketPath     string
	CapabilityVSockPort      int
	GraphitiEndpoint         string
	MattermostBaseURL        string
	HostWorkspacePath        string
	RootFilesystemImagePath  string
	WorkspaceImagePath       string
	HostHTTPListenAddress    string
	HealthPortOrService      string
	GuestHTTPPortOrService   string
	LogDirectoryPath         string
	RuntimeDirectoryPath     string
	OutboundHostDeviceName   string
	OutboundGuestMACAddress  string
	OutboundNetworkCIDR      string
	OutboundHostAddressCIDR  string
	OutboundGuestAddressCIDR string
	OutboundGuestGateway     string
	BridgeListenAddress      string
}

var defaultCircleDefinitions = []defaultCircleDefinition{
	{CircleID: "staff", DisplayName: "Staff"},
	{CircleID: "c-level", DisplayName: "C-level", MattermostChannelName: "circle-c-level"},
	{CircleID: "representative", DisplayName: "Representative", MattermostChannelName: "circle-representative"},
	{CircleID: "admin", DisplayName: "Admin", MattermostChannelName: "circle-admin"},
	{CircleID: "hr-compensation", DisplayName: "HR Compensation", MattermostChannelName: "circle-hr-compensation"},
}

var blueclawNativeToolNames = []string{
	"conversation.history",
	"memory.search",
	"memory.remember",
	"math.calculate",
	"terminal.run",
	"terminal.session",
	"browser_handoff.openURL",
	"ask.confirm",
	"ask.choice",
	"ask.input",
	"file.read",
	"file.preview",
	"file.write",
	"file.edit",
	"file.patch",
	"file.promote",
	"file.attach",
	"skill.add",
	"skill.remove",
	"skill.search",
	"schedule.create",
	"schedule.cancel",
	"db.sql",
}

func BlueclawDefaultAllowedToolNames() []string {
	return removeDefaultSkillScopedToolNames(uniqueStringList(append(blueclawNativeToolNames, capabilities.DefaultToolNames()...)))
}

func removeDefaultSkillScopedToolNames(toolNames []string) []string {
	hiddenToolNames := map[string]bool{
		"site.app.preview": true,
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

func BlueclawRuntimeConfigDocumentWithOptions(options RuntimeConfigOptions) (string, error) {
	languageModelExecutionMode := "auto"
	terminalMode := "firecrackerGuest"
	capabilityTransport := "vsock"
	if options.DirectExecution {
		languageModelExecutionMode = "remote"
		terminalMode = "native"
		capabilityTransport = ""
	}

	capabilityLanguageModel := map[string]any{
		"executionMode":         languageModelExecutionMode,
		"model":                 BlueclawDefaultModelName,
		"contextWindowTokens":   BlueclawDefaultModelContextTokens,
		"requireParameters":     true,
		"enableResponseHealing": true,
	}
	if strings.TrimSpace(options.ModelName) != "" {
		capabilityLanguageModel["model"] = strings.TrimSpace(options.ModelName)
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

	document := map[string]any{
		"baseURL": firstNonEmptyString(options.BaseURL, BlueclawBaseURL),
		"capabilities": map[string]any{
			"transport":       capabilityTransport,
			"unixSocketPath":  capabilityUnixSocketPath,
			"endpoint":        "http://internkim-capability",
			"timeoutSecond":   BlueclawCapabilityTimeoutSecond,
			"vsockCID":        CapabilityVSockHostCID,
			"vsockPort":       capabilityVSockPort,
			"toolNames":       capabilities.DefaultToolNames(),
			"toolDescriptors": capabilities.DefaultToolDescriptors(),
			"routing": map[string]any{
				"candidates": capabilities.RoutingCandidates(),
				"localOnly":  false,
			},
		},
		"languageModel": map[string]any{
			"defaultProvider":  "capabilityLLM",
			"fallbackProvider": "",
			"capability":       capabilityLanguageModel,
		},
		"firecracker": map[string]any{
			"firecrackerPath":        BlueclawFirecrackerPath,
			"jailerPath":             BlueclawJailerPath,
			"kernelImagePath":        BlueclawKernelImagePath,
			"rootfsImagePath":        rootFilesystemImagePath,
			"workspaceImagePath":     workspaceImagePath,
			"hostWorkspacePath":      hostWorkspacePath,
			"vcpuCount":              BlueclawFirecrackerDefaultVirtualCPUCount,
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
		"agent": map[string]any{
			"adminTaskLinkBaseURL": strings.TrimRight(strings.TrimSpace(options.AdminTaskLinkBaseURL), "/"),
			"intake": map[string]any{
				"enabled":       true,
				"executionMode": "auto",
			},
			"defaultEffortLevel": "standard",
			"toolResultMaxBytes": 32768,
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
		},
		"connectors": map[string]any{
			"mattermost": map[string]any{
				"baseURL": mattermostBaseURL,
			},
			"slack": map[string]any{
				"baseURL": BlueclawSlackAPIBaseURL,
			},
		},
		"agentProfiles": []map[string]any{
			{
				"name":             "default",
				"allowedToolNames": BlueclawDefaultAllowedToolNames(),
			},
		},
		"mcpServers": []map[string]any{},
		"terminal": map[string]any{
			"mode":                   terminalMode,
			"sandboxProvider":        "",
			"workspaceRootPath":      terminalWorkspaceRootPath,
			"posixHelperPath":        terminalPOSIXHelperPath,
			"allowedExecutableNames": BlueclawAllowedExecutables,
			"deniedExecutableNames":  BlueclawDeniedExecutables,
			"deniedPathPrefixes":     BlueclawDeniedPathPrefixes,
			"requesterWorkspace": map[string]any{
				"requesterTemporaryEnvironmentVariable": "BLUECLAW_REQUESTER_TMP",
				"taskTemporaryEnvironmentVariable":      "BLUECLAW_TASK_TMP",
				"requesterArtifactsEnvironmentVariable": "BLUECLAW_REQUESTER_ARTIFACTS",
				"requesterTemporaryDirectoryTemplate":   "/workspace/private/people/{personID}/tmp",
				"taskTemporaryDirectoryTemplate":        "/workspace/private/people/{personID}/tmp/{taskID}",
				"requesterArtifactsDirectoryTemplate":   "/workspace/private/people/{personID}/artifacts",
			},
			"timeoutSecond":         120,
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
		adminEmail = "admin@example.test"
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
		{"resource": "tool:flow.task.add", "actions": []string{"execute"}, "circles": []string{"staff"}},
		{"resource": "tool:flow.task.list", "actions": []string{"execute"}, "circles": []string{"staff"}},
		{"resource": "tool:flow.task.update", "actions": []string{"execute"}, "circles": []string{"staff"}},
		{"resource": "tool:platform.message.context", "actions": []string{"execute"}, "circles": []string{"staff"}},
		{"resource": "tool:platform.message.search", "actions": []string{"execute"}, "circles": []string{"staff"}},
		{"resource": "tool:platform.message.send", "actions": []string{"execute"}, "circles": []string{"staff"}},
		{"resource": "tool:platform.message.update", "actions": []string{"execute"}, "circles": []string{"staff"}},
		{"resource": "tool:platform.message.delete", "actions": []string{"execute"}, "circles": []string{"staff"}},
		{"resource": "tool:mattermost.channel.update", "actions": []string{"execute"}, "circles": []string{"admin"}},
		{"resource": "tool:mail.message.list", "actions": []string{"execute"}, "circles": []string{"staff"}},
		{"resource": "tool:mail.message.search", "actions": []string{"execute"}, "circles": []string{"staff"}},
		{"resource": "tool:mail.message.read", "actions": []string{"execute"}, "circles": []string{"staff"}},
		{"resource": "tool:mail.message.send", "actions": []string{"execute"}, "circles": []string{"staff"}},
		{"resource": "tool:mail.message.move", "actions": []string{"execute"}, "circles": []string{"staff"}},
		{"resource": "tool:mail.message.mark", "actions": []string{"execute"}, "circles": []string{"staff"}},
		{"resource": "tool:site.app.create", "actions": []string{"execute"}, "circles": []string{"staff"}},
		{"resource": "tool:site.app.preview", "actions": []string{"execute"}, "circles": []string{"staff"}},
		{"resource": "tool:site.app.publish", "actions": []string{"execute"}, "circles": []string{"staff"}},
		{"resource": "tool:site.app.status", "actions": []string{"execute"}, "circles": []string{"staff"}},
		{"resource": "tool:site.app.history", "actions": []string{"execute"}, "circles": []string{"staff"}},
		{"resource": "tool:site.app.diff", "actions": []string{"execute"}, "circles": []string{"staff"}},
		{"resource": "tool:site.app.logs", "actions": []string{"execute"}, "circles": []string{"staff"}},
		{"resource": "tool:site.app.restore", "actions": []string{"execute"}, "circles": []string{"staff"}},
		{"resource": "tool:site.app.rollback", "actions": []string{"execute"}, "circles": []string{"staff"}},
		{"resource": "tool:site.app.unpublish", "actions": []string{"execute"}, "circles": []string{"staff"}},
		{"resource": "tool:site.app.delete", "actions": []string{"execute"}, "circles": []string{"staff"}},
		{"resource": "tool:company.broadcast.send", "actions": []string{"execute"}, "circles": []string{"representative"}},
	}...)
}
