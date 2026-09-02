package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"gitlab.com/eastriver/internkim/internal/blueclawworkspace"
)

// A scenario names the skills the agent has to pick. The names used to be
// written as paths, and since only the last segment was ever read, a skill that
// moved left the rest of the path saying something untrue and nothing failed.
func TestEveryScenarioNamesASkillThatExists(t *testing.T) {
	repositoryRoot := filepath.Join("..", "..")
	present := map[string]bool{}
	for _, root := range blueclawworkspace.SkillRootPaths(repositoryRoot) {
		entries, errorValue := os.ReadDir(root)
		if errorValue != nil {
			continue
		}
		for _, entry := range entries {
			if entry.IsDir() {
				present[entry.Name()] = true
			}
		}
	}
	if len(present) == 0 {
		t.Fatal("no skills were found at all, so this test is reading the wrong place")
	}
	// Half the skills live in a vendored plugin. Without it the failure reads as a
	// scenario naming a skill nobody wrote, and the next person goes looking for a
	// skill that is right there in another checkout.
	if len(blueclawworkspace.PluginSkillPaths(repositoryRoot)) == 0 {
		t.Skip("no plugin is checked out: git submodule update --init --recursive")
	}

	scenarios, errorValue := filepath.Glob(filepath.Join(repositoryRoot, "tests", "expensive", "*.json"))
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	for _, scenarioPath := range scenarios {
		document, errorValue := os.ReadFile(scenarioPath)
		if errorValue != nil {
			t.Fatal(errorValue)
		}
		var scenario struct {
			SkillNames []string `json:"skillNames"`
		}
		if errorValue := json.Unmarshal(document, &scenario); errorValue != nil {
			t.Fatalf("%s: %v", filepath.Base(scenarioPath), errorValue)
		}
		for _, skillName := range scenario.SkillNames {
			if !present[skillName] {
				t.Fatalf("%s expects the agent to pick %q, and no skill goes by that name", filepath.Base(scenarioPath), skillName)
			}
		}
	}
}
