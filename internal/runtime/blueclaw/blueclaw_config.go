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
)

type defaultCircleDefinition struct {
	CircleID              string
	DisplayName           string
	MattermostChannelName string
}

var defaultCircleDefinitions = []defaultCircleDefinition{
	{CircleID: "staff", DisplayName: "Staff"},
	{CircleID: "c-level", DisplayName: "C-level", MattermostChannelName: "circle-c-level"},
	{CircleID: "representative", DisplayName: "Representative", MattermostChannelName: "circle-representative"},
	{CircleID: "admin", DisplayName: "Admin", MattermostChannelName: "circle-admin"},
	{CircleID: "hr-compensation", DisplayName: "HR Compensation", MattermostChannelName: "circle-hr-compensation"},
}

func BlueclawRuntimeConfigDocument(modelName string) (string, error) {
	capabilityLanguageModel := map[string]any{
		"executionMode":         "auto",
		"model":                 BlueclawDefaultModelName,
		"contextWindowTokens":   BlueclawDefaultModelContextTokens,
		"requireParameters":     true,
		"enableResponseHealing": true,
	}
	if strings.TrimSpace(modelName) != "" {
		capabilityLanguageModel["model"] = strings.TrimSpace(modelName)
	}

	document := map[string]any{
		"baseURL": BlueclawBaseURL,
		"capabilities": map[string]any{
			"transport":       "vsock",
			"unixSocketPath":  "",
			"endpoint":        "http://internkim-capability",
			"timeoutSecond":   BlueclawCapabilityTimeoutSecond,
			"vsockCID":        CapabilityVSockHostCID,
			"vsockPort":       CapabilityVSockPort,
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
			"rootfsImagePath":        BlueclawRootFilesystemImagePath,
			"workspaceImagePath":     BlueclawWorkspaceImagePath,
			"hostWorkspacePath":      BlueclawWorkspacePath,
			"vcpuCount":              4,
			"memoryMiB":              8192,
			"vsockCID":               52,
			"healthPortOrService":    "8082",
			"guestHTTPPortOrService": "8081",
			"hostHTTPListenAddress":  "127.0.0.1:8080",
			"logDirectoryPath":       BlueclawSupervisorLogDirectoryPath,
			"runtimeDirectoryPath":   "/var/lib/bc",
			"guestListenerProxies": []map[string]any{
				{
					"guestPort":            CapabilityVSockPort,
					"targetUnixSocketPath": CapabilitySocketPath,
				},
			},
		},
		"bridge": map[string]any{
			"mode":                     "localAgent",
			"authMode":                 "sshKeyReuse",
			"authorizedPublicKeysPath": BlueclawBridgeAuthorizedKeysPath,
			"listenAddress":            BlueclawBridgeListenAddress,
		},
		"database": map[string]any{
			"driver":                 "postgres",
			"connectionString":       BlueclawGuestDatabaseConnectionString,
			"migrationDirectoryPath": BlueclawGuestMigrationPath,
		},
		"memory": map[string]any{
			"workspaceID":                                 "default",
			"graphitiEndpoint":                            GraphitiEndpoint,
			"graphitiKuzuPath":                            path.Join(BlueclawGuestWorkspacePath, ".blueclaw", "graphiti", "kuzu"),
			"pinnedMemoryRootPath":                        path.Join(BlueclawGuestWorkspacePath, ".blueclaw", "memory"),
			"pinnedMemoryHardLimitCharacterCount":         BlueclawPinnedMemoryHardLimitCharacterCount,
			"pinnedMemoryCompressionTargetCharacterCount": BlueclawPinnedMemoryCompressionTargetCharacterCount,
			"timeoutSecond":                               60,
		},
		"agent": map[string]any{
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
				"baseURL": "http://localhost:8065",
			},
			"slack": map[string]any{
				"baseURL": BlueclawSlackAPIBaseURL,
			},
		},
		"agentProfiles": []map[string]any{
			{
				"name":             "default",
				"allowedToolNames": append([]string{"conversation.history", "memory.search", "memory.remember", "terminal.run", "terminal.session", "browser_handoff.openURL", "ask.confirm", "ask.choice", "ask.input", "file.write", "file.promote", "file.attach", "skill.add", "skill.remove", "skill.search", "schedule.create", "schedule.cancel"}, capabilities.DefaultToolNames()...),
			},
		},
		"mcpServers": []map[string]any{},
		"terminal": map[string]any{
			"mode":                   "firecrackerGuest",
			"sandboxProvider":        "",
			"workspaceRootPath":      BlueclawGuestWorkspacePath,
			"posixHelperPath":        BlueclawPOSIXHelperPath,
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
		{"resource": "tool:flow.task.add", "actions": []string{"execute"}, "circles": []string{"staff"}},
		{"resource": "tool:mail.message.list", "actions": []string{"execute"}, "circles": []string{"staff"}},
		{"resource": "tool:mail.message.search", "actions": []string{"execute"}, "circles": []string{"staff"}},
		{"resource": "tool:mail.message.read", "actions": []string{"execute"}, "circles": []string{"staff"}},
		{"resource": "tool:mail.message.send", "actions": []string{"execute"}, "circles": []string{"staff"}},
		{"resource": "tool:mail.message.move", "actions": []string{"execute"}, "circles": []string{"staff"}},
		{"resource": "tool:mail.message.mark", "actions": []string{"execute"}, "circles": []string{"staff"}},
		{"resource": "tool:site.app.create", "actions": []string{"execute"}, "circles": []string{"staff"}},
		{"resource": "tool:site.app.publish", "actions": []string{"execute"}, "circles": []string{"staff"}},
		{"resource": "tool:site.app.status", "actions": []string{"execute"}, "circles": []string{"staff"}},
		{"resource": "tool:site.app.logs", "actions": []string{"execute"}, "circles": []string{"staff"}},
		{"resource": "tool:site.app.restore", "actions": []string{"execute"}, "circles": []string{"staff"}},
		{"resource": "tool:site.app.rollback", "actions": []string{"execute"}, "circles": []string{"staff"}},
		{"resource": "tool:site.app.unpublish", "actions": []string{"execute"}, "circles": []string{"staff"}},
		{"resource": "tool:site.app.delete", "actions": []string{"execute"}, "circles": []string{"staff"}},
		{"resource": "tool:company.broadcast.send", "actions": []string{"execute"}, "circles": []string{"representative"}},
	}...)
}
