package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBlueclawNamesTheScenarioSkillRootsVariable(t *testing.T) {
	scenariosPath := filepath.Join("..", "..", ".dependency", "blueclaw", "internal", "e2e", "scenarios.go")
	document, errorValue := os.ReadFile(scenariosPath)
	if errorValue != nil {
		t.Skip("blueclaw is not checked out: git submodule update --init --recursive")
	}
	declaration := "ScenarioSkillRootsVariable = \"" + blueclawScenarioSkillRootsVariable + "\""
	if !strings.Contains(string(document), declaration) {
		t.Fatalf("blueclaw no longer declares %s, so dev simulate hands its scenarios nothing and they skip", declaration)
	}
}

func TestBlueclawNamesTheScenarioCapabilityCatalogVariable(t *testing.T) {
	sessionPath := filepath.Join("..", "..", ".dependency", "blueclaw", "internal", "e2e", "virtual_session.go")
	document, errorValue := os.ReadFile(sessionPath)
	if errorValue != nil {
		t.Skip("blueclaw is not checked out: git submodule update --init --recursive")
	}
	declaration := "ScenarioCapabilityCatalogVariable = \"" + blueclawScenarioCapabilityCatalogVariable + "\""
	if !strings.Contains(string(document), declaration) {
		t.Fatalf("blueclaw no longer declares %s, so dev simulate hands its scenarios no catalog and they skip", declaration)
	}
}
