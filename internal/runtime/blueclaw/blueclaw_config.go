package blueclaw

import (
	"encoding/json"
	"path"
	"strings"

	"gitlab.com/eastriver/internkim/internal/capabilities"
)

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
			"transport":      "vsock",
			"unixSocketPath": "",
			"endpoint":       "http://internkim-capability",
			"timeoutSecond":  120,
			"vsockCID":       CapabilityVSockHostCID,
			"vsockPort":      CapabilityVSockPort,
			"toolNames":      capabilities.DefaultToolNames(),
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
				"allowedToolNames": append([]string{"conversation.history", "memory.search", "terminal.run", "terminal.session", "browser_handoff.openURL", "approval.request", "file.write", "file.attach"}, capabilities.DefaultToolNames()...),
			},
		},
		"mcpServers": []map[string]any{},
		"terminal": map[string]any{
			"mode":                   "firecrackerGuest",
			"sandboxProvider":        "",
			"workspaceRootPath":      BlueclawGuestWorkspacePath,
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
			"retentionCheckIntervalMinute": 60,
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
				"securityLevelName": "admin",
				"securityLevelRank": 100,
				"grantedClasses":    []string{"internal", "executive"},
				"isAdmin":           true,
			},
		},
		"channels":  []map[string]any{},
		"retention": map[string]any{"rawEventDays": 60},
	}

	documentBytes, error := json.MarshalIndent(document, "", "  ")
	if error != nil {
		return "", error
	}

	return string(documentBytes) + "\n", nil
}
