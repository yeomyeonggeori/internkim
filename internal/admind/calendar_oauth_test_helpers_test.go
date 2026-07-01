package admind

import (
	"os"
	"path/filepath"
	"testing"
)

func writeGoogleClientFile(t *testing.T, service *Service, content string) {
	t.Helper()
	directory := service.calendarSecretsDirectory()
	if errorValue := os.MkdirAll(directory, 0o700); errorValue != nil {
		t.Fatalf("mkdir: %v", errorValue)
	}
	path := filepath.Join(directory, googleOAuthClientFileName)
	if errorValue := os.WriteFile(path, []byte(content), 0o600); errorValue != nil {
		t.Fatalf("write client.json: %v", errorValue)
	}
}
