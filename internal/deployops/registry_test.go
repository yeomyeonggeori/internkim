package deployops

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDiscoverDefaultRegistryFindsPilotTarget(t *testing.T) {
	homePath := t.TempDir()
	statePath := filepath.Join(homePath, "profiles", "pilot", "devices", "jetson-orin-nano", "boards", "1")
	if errorValue := os.MkdirAll(statePath, 0o755); errorValue != nil {
		t.Fatal(errorValue)
	}
	writeTestState(t, statePath, "device_url", "https://pilot-01.example.test")
	writeTestState(t, statePath, "node_id", "pilot-01")

	registry := discoverDefaultRegistry(homePath)
	if len(registry.Targets) != 1 {
		t.Fatalf("expected one target, got %d", len(registry.Targets))
	}
	target := registry.Targets[0]
	if target.Name != "pilot-01" {
		t.Fatalf("expected pilot-01 name, got %q", target.Name)
	}
	if target.Profile != "pilot" || target.NodeArgument != "1" {
		t.Fatalf("unexpected target routing: %#v", target)
	}
	if target.SecretSource != statePath {
		t.Fatalf("expected local state secret source, got %q", target.SecretSource)
	}
}

func TestSaveRegistryDoesNotRequireSecretValue(t *testing.T) {
	repositoryRootPath := t.TempDir()
	registry := TargetRegistry{Targets: []Target{{
		Name:         "pilot-01",
		AdminURL:     "pilot-01.example.test",
		Profile:      "pilot",
		NodeArgument: "1",
		SecretSource: "/local/state",
	}}}

	if errorValue := SaveRegistry(repositoryRootPath, registry); errorValue != nil {
		t.Fatal(errorValue)
	}
	loadedRegistry, errorValue := LoadRegistry(repositoryRootPath, t.TempDir())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	target := loadedRegistry.Targets[0]
	if target.AdminURL != "https://pilot-01.example.test" {
		t.Fatalf("expected normalized admin URL, got %q", target.AdminURL)
	}
	if target.ID == "" {
		t.Fatal("expected generated target id")
	}
}

func writeTestState(t *testing.T, statePath string, name string, value string) {
	t.Helper()
	if errorValue := os.WriteFile(filepath.Join(statePath, name), []byte(value), 0o600); errorValue != nil {
		t.Fatal(errorValue)
	}
}
