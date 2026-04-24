package blueclaw

import "encoding/json"

func BlueclawRuntimeConfigDocument(modelName string) (string, error) {
	document := map[string]any{
		"baseURL": BlueclawBaseURL,
		"languageModel": map[string]any{
			"defaultProvider":  "openRouter",
			"fallbackProvider": "",
			"openRouter": map[string]any{
				"baseURL":               BlueclawOpenRouterCompletionsURL,
				"modelName":             modelName,
				"requireParameters":     true,
				"enableResponseHealing": true,
			},
			"liteRTLM": map[string]any{
				"wrapperPath":        BlueclawLiteRTLMWrapperPath,
				"wrapperArguments":   []string{"--stdio"},
				"modelPath":          BlueclawLiteRTLMModelPath(),
				"backend":            "cpu",
				"constraintProvider": "llguidance",
			},
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
		"connectors": map[string]any{
			"mattermost": map[string]any{
				"baseURL":      "http://localhost:8065",
				"botTokenPath": BlueclawMattermostTokenPath,
			},
			"slack": map[string]any{
				"baseURL":      BlueclawSlackAPIBaseURL,
				"botTokenPath": BlueclawSlackTokenPath,
			},
		},
		"agentProfiles": []map[string]any{},
		"mcpServers":    []map[string]any{},
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
		adminEmail = "admin@intern.kim"
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
