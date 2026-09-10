package fleet

import (
	"path/filepath"
	"strings"
)

const (
	WorkspaceSyncModeContent        = "content"
	WorkspaceSyncModeAppendLog      = "append-log"
	WorkspaceSyncModeSealedSnapshot = "sealed-snapshot"
	WorkspaceSyncModeEphemeral      = "ephemeral"
	WorkspaceSyncModeRuntimeCache   = "runtime-cache"
)

func ClassifyWorkspacePath(path string) string {
	normalizedPath := normalizeWorkspacePath(path)
	if normalizedPath == "" {
		return WorkspaceSyncModeContent
	}
	if hasWorkspacePathPrefix(normalizedPath, ".blueclaw/postgres") {
		return WorkspaceSyncModeSealedSnapshot
	}
	if hasWorkspacePathPrefix(normalizedPath, ".blueclaw/runtime") {
		return WorkspaceSyncModeRuntimeCache
	}
	if hasWorkspacePathPrefix(normalizedPath, ".blueclaw/tmp") || isPersonTemporaryWorkspacePath(normalizedPath) || hasTemporaryWorkspaceName(normalizedPath) {
		return WorkspaceSyncModeEphemeral
	}
	if hasWorkspacePathPrefix(normalizedPath, "sessions") {
		if strings.HasSuffix(normalizedPath, ".jsonl") || strings.HasSuffix(normalizedPath, ".log") {
			return WorkspaceSyncModeAppendLog
		}
		return WorkspaceSyncModeEphemeral
	}
	return WorkspaceSyncModeContent
}

func normalizeWorkspacePath(path string) string {
	normalizedPath := filepath.ToSlash(filepath.Clean(strings.TrimSpace(path)))
	normalizedPath = strings.TrimPrefix(normalizedPath, "/root/.blueclaw/workspace/")
	normalizedPath = strings.TrimPrefix(normalizedPath, "/workspace/")
	normalizedPath = strings.TrimPrefix(normalizedPath, "./")
	if normalizedPath == "." || strings.HasPrefix(normalizedPath, "../") {
		return ""
	}
	return normalizedPath
}

func hasWorkspacePathPrefix(path string, prefix string) bool {
	return path == prefix || strings.HasPrefix(path, prefix+"/")
}

func isPersonTemporaryWorkspacePath(path string) bool {
	parts := strings.Split(path, "/")
	return len(parts) >= 5 && parts[0] == "private" && parts[1] == "people" && parts[3] == "tmp"
}

func hasTemporaryWorkspaceName(path string) bool {
	name := strings.ToLower(filepath.Base(path))
	return strings.HasSuffix(name, ".sock") ||
		strings.HasSuffix(name, ".lock") ||
		strings.HasSuffix(name, ".tmp") ||
		strings.HasPrefix(name, ".nfs")
}
