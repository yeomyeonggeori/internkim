package blueclaw

import (
	"encoding/json"
	"path"
	"strings"

	"gitlab.com/eastriver/internkim/internal/capabilities"
)

const BlueclawCapabilityTimeoutSecond = 0

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
			"workspaceID":      "default",
			"graphitiEndpoint": GraphitiEndpoint,
			"graphitiKuzuPath": path.Join(BlueclawGuestWorkspacePath, ".blueclaw", "graphiti", "kuzu"),
			"timeoutSecond":    60,
		},
		"agent": map[string]any{
			"intake": map[string]any{
				"enabled":       true,
				"executionMode": "auto",
			},
			"defaultEffortLevel": "standard",
			"toolResultMaxBytes": 32768,
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
				"allowedToolNames": append([]string{"conversation.history", "memory.search", "terminal.run", "terminal.session", "browser_handoff.openURL", "approval.request", "file.write", "file.attach", "skill.add", "skill.remove", "schedule.create", "schedule.cancel"}, capabilities.DefaultToolNames()...),
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
			"timeoutSecond":          120,
			"outputMaxBytes":         32768,
			"sessionMaxCount":        4,
			"allowNetwork":           true,
			"allowInteractiveShell":  true,
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
		"circles": []map[string]any{
			defaultCirclePolicy("staff", "Staff"),
			defaultCirclePolicy("c-level", "C-level"),
			defaultCirclePolicy("representative", "Representative"),
			defaultCirclePolicy("admin", "Admin"),
		},
		"circleSync": map[string]any{
			"mattermostPrivateChannels": []map[string]any{
				{"circleID": "c-level", "channelName": "circle-c-level"},
				{"circleID": "representative", "channelName": "circle-representative"},
				{"circleID": "admin", "channelName": "circle-admin"},
			},
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

func defaultResourceAccessPolicies() []map[string]any {
	return []map[string]any{
		{"resource": "file:circle:staff", "actions": []string{"read", "write"}, "circles": []string{"staff"}},
		{"resource": "file:circle:c-level", "actions": []string{"read", "write"}, "circles": []string{"c-level"}},
		{"resource": "file:circle:representative", "actions": []string{"read", "write"}, "circles": []string{"representative"}},
		{"resource": "file:circle:admin", "actions": []string{"read", "write", "manage"}, "circles": []string{"admin"}},
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
	}
}
