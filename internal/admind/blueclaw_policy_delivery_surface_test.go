package admind

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The agent reads its roster from a file it cannot write, so asking it to save
// one answers 500 forever. Every write goes through deliverBlueclawPolicy, which
// writes the file and then tells the agent to re-read.
func TestNothingAsksTheAgentToSaveItsOwnPolicy(t *testing.T) {
	entries, errorValue := os.ReadDir(".")
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	for _, entry := range entries {
		name := entry.Name()
		if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		document, errorValue := os.ReadFile(filepath.Join(".", name))
		if errorValue != nil {
			t.Fatal(errorValue)
		}
		if strings.Contains(string(document), "/admin/api/policy/save") {
			t.Errorf("%s posts the roster to the agent, which cannot write its roster file; call deliverBlueclawPolicy instead", name)
		}
	}
}
