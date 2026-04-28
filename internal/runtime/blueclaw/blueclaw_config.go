package blueclaw

import (
	"encoding/json"
	"strings"

	"github.com/anthropic-lab/internkim/internal/capabilities"
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
			"transport":      "unix",
			"unixSocketPath": CapabilitySocketPath,
			"endpoint":       "http://internkim",
			"timeoutSecond":  120,
			"vsockCID":       52,
			"vsockPort":      7000,
			"toolNames":      capabilities.CompanionToolNames(),
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
			"firecrackerPath":     BlueclawFirecrackerPath,
			"jailerPath":          BlueclawJailerPath,
			"kernelImagePath":     BlueclawKernelImagePath,
			"rootfsImagePath":     BlueclawRootFilesystemImagePath,
			"workspaceImagePath":  BlueclawWorkspaceImagePath,
			"vcpuCount":           4,
			"memoryMiB":           8192,
			"vsockCID":            52,
			"healthPortOrService": "8080",
			"logDirectoryPath":    BlueclawSupervisorLogDirectoryPath,
		},
		"bridge": map[string]any{
			"mode":                     "localAgent",
			"authMode":                 "sshKeyReuse",
			"authorizedPublicKeysPath": BlueclawBridgeAuthorizedKeysPath,
			"listenAddress":            BlueclawBridgeListenAddress,
		},
		"database": map[string]any{
			"driver":                 "postgres",
			"connectionString":       BlueclawDatabaseConnectionString,
			"migrationDirectoryPath": BlueclawMigrationPath,
		},
		"memory": map[string]any{
			"workspaceID":      "default",
			"graphitiEndpoint": GraphitiEndpoint,
			"graphitiKuzuPath": GraphitiKuzuPath,
			"timeoutSecond":    60,
		},
		"agent": map[string]any{
			"intake": map[string]any{
				"enabled":       true,
				"executionMode": "auto",
			},
			"defaultBudgetClass": "thirty_minutes",
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
				"allowedToolNames": append([]string{"conversation.history", "memory.search"}, capabilities.CompanionToolNames()...),
			},
		},
		"mcpServers": []map[string]any{},
		"terminal": map[string]any{
			"mode":                   "native",
			"sandboxProvider":        "",
			"workspaceRootPath":      BlueclawWorkspacePath,
			"allowedExecutableNames": BlueclawAllowedExecutables,
			"deniedExecutableNames":  BlueclawDeniedExecutables,
			"deniedPathPrefixes":     BlueclawDeniedPathPrefixes,
			"timeoutSecond":          120,
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
