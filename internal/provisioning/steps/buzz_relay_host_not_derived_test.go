package setup

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A Buzz community is keyed to the relay's public host and communities is unique
// on lower(host), so a guessed name is a second community that nothing serves.
func TestNothingDerivesTheRelayHostFromTheDeviceHost(t *testing.T) {
	roots := []string{filepath.Join("..", "..", "..", "internal"), filepath.Join("..", "..", "..", "host")}
	for _, root := range roots {
		errorValue := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkError error) error {
			if walkError != nil || entry.IsDir() || !strings.HasSuffix(path, ".go") {
				return walkError
			}
			if strings.HasSuffix(path, "_test.go") {
				return nil
			}
			document, readError := os.ReadFile(path)
			if readError != nil {
				return readError
			}
			if strings.Contains(string(document), `-relay./`) || strings.Contains(string(document), `}-relay"`) {
				t.Errorf("%s builds a relay host out of the device host; read it from the relay's own RELAY_URL", path)
			}
			return nil
		})
		if errorValue != nil {
			t.Fatal(errorValue)
		}
	}
}
