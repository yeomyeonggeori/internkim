package admind

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The agent is one identity across the import and everything after it. Without
// --agent-email its history is signed with a key derived from the bot's
// messenger address rather than the one chatd answers with, so the company reads
// its own agent as a stranger.
func TestEveryImportPathNamesTheAgent(t *testing.T) {
	callers := map[string]string{
		"recovery.go":          buzzReimportCommand(),
		"step_buzz_migrate.go": readImportStep(t),
	}
	for name, command := range callers {
		if !strings.Contains(command, "--agent-email") {
			t.Errorf("%s runs buzz-migrate without --agent-email, so the agent's history gets a key nothing else uses", name)
		}
	}
}

func readImportStep(t *testing.T) string {
	t.Helper()
	document, errorValue := os.ReadFile(filepath.Join("..", "provisioning", "steps", "step_buzz_migrate.go"))
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	return string(document)
}
