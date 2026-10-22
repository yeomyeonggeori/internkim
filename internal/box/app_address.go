package box

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/yeomyeonggeori/internkim/internal/durablefile"
)

const appAddressFileName = "app-url"

func RememberedAppURL(stateDirectoryPath string) string {
	document, errorValue := os.ReadFile(filepath.Join(stateDirectoryPath, appAddressFileName))
	if errorValue != nil {
		return ""
	}
	return strings.TrimSpace(string(document))
}

func rememberAppURL(stateDirectoryPath string, appURL string) error {
	if appURL == "" || RememberedAppURL(stateDirectoryPath) == appURL {
		return nil
	}
	if errorValue := os.MkdirAll(stateDirectoryPath, 0o700); errorValue != nil {
		return errorValue
	}
	return durablefile.Write(filepath.Join(stateDirectoryPath, appAddressFileName), []byte(appURL+"\n"), 0o600)
}
