package blueclawworkspace

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEveryVendoredPluginDeclaresTheSchemaThatExists(t *testing.T) {
	repositoryRootPath := filepath.Join("..", "..")
	manifests, errorValue := PluginManifests(repositoryRootPath)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if len(manifests) == 0 {
		t.Fatal("no plugin is checked out: git submodule update --init --recursive")
	}
	for _, manifest := range manifests {
		if manifest.Name == "" || manifest.Version == "" {
			t.Fatalf("%s names no plugin or no version", manifest.Path)
		}
	}
}

func TestAManifestNamingAnotherSchemaIsRefused(t *testing.T) {
	pluginPath := t.TempDir()
	manifest := `{"$schema":"https://agent-plugins.org/schemas/1.1.0/plugin.schema.json","name":"x","version":"1"}`
	if errorValue := os.WriteFile(filepath.Join(pluginPath, "plugin.json"), []byte(manifest), 0o644); errorValue != nil {
		t.Fatal(errorValue)
	}

	_, errorValue := ReadPluginManifest(pluginPath)

	if errorValue == nil || !strings.Contains(errorValue.Error(), PluginSchemaURL) {
		t.Fatalf("a schema no client can resolve makes it reject the plugin outright, got %v", errorValue)
	}
}
