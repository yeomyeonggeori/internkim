package blueclawworkspace

import (
	"os"
	"path/filepath"
	"testing"
)

func writeTestPlugin(t *testing.T, repositoryRootPath string, pluginName string, skillNames ...string) {
	t.Helper()
	pluginPath := filepath.Join(dependencyPath(repositoryRootPath), pluginName)
	for _, skillName := range skillNames {
		if errorValue := os.MkdirAll(filepath.Join(pluginPath, "skills", skillName), 0o755); errorValue != nil {
			t.Fatal(errorValue)
		}
	}
	if errorValue := os.MkdirAll(filepath.Join(pluginPath, "skills"), 0o755); errorValue != nil {
		t.Fatal(errorValue)
	}
	manifest := []byte(`{"name":"` + pluginName + `"}`)
	if errorValue := os.WriteFile(filepath.Join(pluginPath, "plugin.json"), manifest, 0o644); errorValue != nil {
		t.Fatal(errorValue)
	}
}

func TestASecondPluginIsFoundByItsManifest(t *testing.T) {
	repositoryRootPath := t.TempDir()
	writeTestPlugin(t, repositoryRootPath, "internkim-plugin", "bundled")
	writeTestPlugin(t, repositoryRootPath, "another-plugin", "borrowed")
	if errorValue := os.MkdirAll(filepath.Join(dependencyPath(repositoryRootPath), "blueclaw"), 0o755); errorValue != nil {
		t.Fatal(errorValue)
	}

	skillDirectories, errorValue := SkillDirectories(repositoryRootPath)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	foundNames := []string{}
	for _, skillDirectory := range skillDirectories {
		foundNames = append(foundNames, skillDirectory.Name)
	}
	if len(foundNames) != 2 || foundNames[0] != "borrowed" || foundNames[1] != "bundled" {
		t.Fatalf("both plugins must be read the same way, got %v", foundNames)
	}
	if len(PluginPaths(repositoryRootPath)) != 2 {
		t.Fatalf("a vendored directory without a manifest is not a plugin, got %v", PluginPaths(repositoryRootPath))
	}
}
