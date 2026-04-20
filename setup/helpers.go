package setup

import (
	"os"
	"path/filepath"
	"strings"
)

func trimmedRun(context *Context, command string) string {
	if context.SSH == nil {
		return ""
	}
	return strings.TrimSpace(context.SSH.Run(command))
}

func sshFileExists(context *Context, path string) bool {
	if context.SSH == nil {
		return false
	}
	return trimmedRun(context, "test -e "+shellQuote(path)+" && echo y || echo n") == "y"
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", `'"'"'`) + "'"
}

func readStagedFile(context *Context, stagePath string) ([]byte, error) {
	if context.SD == nil {
		return nil, ErrUnsupportedBackend
	}
	return os.ReadFile(filepath.Join(context.SD.RootPath(), stagePath))
}

func stagedFileExists(context *Context, stagePath string) bool {
	if context.SD == nil {
		return false
	}
	_, statError := os.Stat(filepath.Join(context.SD.RootPath(), stagePath))
	return statError == nil
}
