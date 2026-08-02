package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"gitlab.com/eastriver/internkim/internal/capabilities"
	"gitlab.com/eastriver/internkim/internal/runtime/blueclaw"
)

func TestGeneratedCapabilityContractIsAcceptedByPocRefresh(t *testing.T) {
	temporaryDirectoryPath := t.TempDir()
	contractPath := filepath.Join(temporaryDirectoryPath, "capability-contract.json")
	configurationRootPath := filepath.Join(temporaryDirectoryPath, "config")
	tenantPath := filepath.Join(configurationRootPath, "tenant_15")
	if errorValue := os.MkdirAll(tenantPath, 0o700); errorValue != nil {
		t.Fatal(errorValue)
	}
	contractDocument, errorValue := blueclaw.CapabilityContractDocument()
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	writeTestDocument(t, contractPath, contractDocument)
	writeTestDocument(t, filepath.Join(tenantPath, "runtime.json"), `{
  "capabilities": {
    "toolNames": ["flow.task.list"],
    "toolDescriptors": [{"name": "flow.task.list"}],
    "routing": {"candidates": ["legacy"], "localOnly": false}
  }
}`)
	writeTestDocument(t, filepath.Join(tenantPath, "policy.json"), `{
  "resourceAccess": [
    {"resource": "tool:flow.task.list", "actions": ["execute"], "circles": ["staff"]}
  ]
}`)

	command := exec.Command(
		"python3",
		filepath.Join("..", "refresh_capability_contract.py"),
		"--contract",
		contractPath,
		"--config-root",
		configurationRootPath,
	)
	if output, errorValue := command.CombinedOutput(); errorValue != nil {
		t.Fatalf("refresh failed: %v\n%s", errorValue, output)
	}

	runtimeDocument := readTestObject(t, filepath.Join(tenantPath, "runtime.json"))
	capabilityConfiguration := runtimeDocument["capabilities"].(map[string]any)
	toolDescriptors := capabilityConfiguration["toolDescriptors"].([]any)
	if len(toolDescriptors) != len(capabilities.DefaultToolDescriptors()) {
		t.Fatalf("tool descriptor count = %d, want %d", len(toolDescriptors), len(capabilities.DefaultToolDescriptors()))
	}
	if _, hasLegacyToolNames := capabilityConfiguration["toolNames"]; hasLegacyToolNames {
		t.Fatal("legacy toolNames remained in refreshed runtime")
	}
	if capabilityConfiguration["protocolVersion"] != "0.4.0" {
		t.Fatalf("protocol version = %v, want 0.4.0", capabilityConfiguration["protocolVersion"])
	}
	if capabilityConfiguration["aggregateProtocolHash"] != "cbe08983acd56a9098505c5f2e99f77a73d7257fac01258ef00846e7f83cf998" {
		t.Fatalf("aggregate protocol hash = %v, want generated hash", capabilityConfiguration["aggregateProtocolHash"])
	}
	for _, value := range toolDescriptors {
		toolDescriptor := value.(map[string]any)
		if toolDescriptor["name"] == "flow.task.list" {
			t.Fatal("legacy task tool remained in refreshed runtime")
		}
	}

	policyDocument := readTestObject(t, filepath.Join(tenantPath, "policy.json"))
	for _, value := range policyDocument["resourceAccess"].([]any) {
		entry := value.(map[string]any)
		if entry["resource"] == "tool:flow.task.list" {
			t.Fatal("legacy task policy remained after refresh")
		}
	}
}

func writeTestDocument(t *testing.T, path string, document string) {
	t.Helper()
	if errorValue := os.WriteFile(path, []byte(document), 0o600); errorValue != nil {
		t.Fatal(errorValue)
	}
}

func readTestObject(t *testing.T, path string) map[string]any {
	t.Helper()
	document, errorValue := os.ReadFile(path)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	var object map[string]any
	if errorValue := json.Unmarshal(document, &object); errorValue != nil {
		t.Fatal(errorValue)
	}
	return object
}
