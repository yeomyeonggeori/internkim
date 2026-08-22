package admind

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A person's Buzz key belongs to the bridge: it versions the subject, keeps the
// agent apart from its bot address, and pins the result. An import left to
// derive its own writes history signed by people nothing else can act as, and
// puts members in rooms that the writers are then refused from.
func TestEveryImportPathAsksWhoOwnsAPersonsKey(t *testing.T) {
	callers := map[string]string{
		"recovery.go":          buzzReimportCommand(),
		"step_buzz_migrate.go": readImportStep(t),
	}
	for name, command := range callers {
		if !strings.Contains(command, "--bridge-url") {
			t.Errorf("%s runs buzz-migrate without --bridge-url, so it derives keys of its own and records nothing it makes", name)
		}
		if strings.Contains(command, "--agent-email") {
			t.Errorf("%s still names the agent by address; the bridge is what knows the agent's identity", name)
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
