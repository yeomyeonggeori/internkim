package blueclawworkspace

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// PluginSchemaURL is a const in the Agent Plugins specification, so a manifest
// naming any other value is rejected by every conforming client.
const PluginSchemaURL = "https://agent-plugins.org/schemas/1.0.0/plugin.schema.json"

type PluginManifest struct {
	Schema  string `json:"$schema"`
	Name    string `json:"name"`
	Version string `json:"version"`
	Path    string `json:"-"`
}

func ReadPluginManifest(pluginPath string) (PluginManifest, error) {
	manifestPath := filepath.Join(pluginPath, "plugin.json")
	document, errorValue := os.ReadFile(manifestPath)
	if errorValue != nil {
		return PluginManifest{}, errorValue
	}
	manifest := PluginManifest{Path: pluginPath}
	if errorValue := json.Unmarshal(document, &manifest); errorValue != nil {
		return PluginManifest{}, fmt.Errorf("%s: %w", manifestPath, errorValue)
	}
	if manifest.Schema != PluginSchemaURL {
		return PluginManifest{}, fmt.Errorf("%s names schema %q, and a conforming client accepts only %q", manifestPath, manifest.Schema, PluginSchemaURL)
	}
	if manifest.Name == "" || manifest.Version == "" {
		return PluginManifest{}, fmt.Errorf("%s must name the plugin and its version", manifestPath)
	}
	return manifest, nil
}

func PluginManifests(scriptDir string) ([]PluginManifest, error) {
	manifests := []PluginManifest{}
	for _, pluginPath := range PluginPaths(scriptDir) {
		manifest, errorValue := ReadPluginManifest(pluginPath)
		if errorValue != nil {
			return nil, errorValue
		}
		manifests = append(manifests, manifest)
	}
	return manifests, nil
}
