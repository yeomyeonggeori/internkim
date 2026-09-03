package admind

import (
	"os"
	"sync"
	"testing"
)

func forgetCentralPlaneForTest(service *Service) {
	service.centralPlaneOnce = sync.Once{}
	service.centralPlaneClient = nil
}

func writeAgentKeyForTest(t *testing.T, key string) string {
	t.Helper()
	path := t.TempDir() + "/agent-key"
	if errorValue := os.WriteFile(path, []byte(key), 0o600); errorValue != nil {
		t.Fatal(errorValue)
	}
	return path
}
