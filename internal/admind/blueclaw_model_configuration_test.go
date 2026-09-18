package admind

import (
	"encoding/json"
	"reflect"
	"testing"

	"gitlab.com/eastriver/internkim/internal/modelladder"
	blueclawruntime "gitlab.com/eastriver/internkim/internal/runtime/blueclaw"
)

func TestRefreshMigratesDeployedLegacyModelConfiguration(t *testing.T) {
	document := `{"capabilities":{},"languageModel":{"defaultProvider":"capabilityLLM","fallbackProvider":"","capability":{"model":"vendor/legacy","executionMode":"auto","contextWindowTokens":1048576,"lowModel":"vendor/custom"}},"terminal":{"workspaceRootPath":"/workspace"}}`
	refreshed, errorValue := refreshedBlueclawRuntimeConfiguration(document, blueclawruntime.CurrentCapabilityContract())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	var result struct {
		LanguageModel struct {
			Capability map[string]any `json:"capability"`
			Embedding  struct {
				Model string `json:"model"`
			} `json:"embedding"`
			ContextWindowTokens int `json:"contextWindowTokens"`
		} `json:"languageModel"`
	}
	if errorValue := json.Unmarshal([]byte(refreshed), &result); errorValue != nil {
		t.Fatal(errorValue)
	}
	for tier, modelName := range modelladder.TierModelNames() {
		if tier == "low" {
			modelName = "vendor/custom"
		}
		if result.LanguageModel.Capability[tier+"Model"] != modelName {
			t.Fatalf("%s tier lost its effective model: %+v", tier, result.LanguageModel.Capability)
		}
	}
	if result.LanguageModel.Embedding.Model != modelladder.EmbeddingModel || result.LanguageModel.ContextWindowTokens != 1048576 {
		t.Fatalf("model configuration was not completely migrated: %+v", result.LanguageModel)
	}
	secondRefresh, errorValue := refreshedBlueclawRuntimeConfiguration(refreshed, blueclawruntime.CurrentCapabilityContract())
	if errorValue != nil || secondRefresh != refreshed {
		t.Fatalf("migration is not idempotent: %v", errorValue)
	}
}

func TestRefreshStampsTheLadderOwnedModelsIntoADeployedConfiguration(t *testing.T) {
	document := `{"capabilities":{},"languageModel":{"capability":{"model":"vendor/current","executionMode":"auto","lowModel":"vendor/current"}}}`
	refreshed, errorValue := refreshedBlueclawRuntimeConfiguration(document, blueclawruntime.CurrentCapabilityContract())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	var result struct {
		LanguageModel struct {
			Capability map[string]any `json:"capability"`
		} `json:"languageModel"`
	}
	if errorValue := json.Unmarshal([]byte(refreshed), &result); errorValue != nil {
		t.Fatal(errorValue)
	}
	if result.LanguageModel.Capability["decisionModel"] != modelladder.DecisionModel {
		t.Fatalf("a deployed configuration without a decision model keeps starting without one: %+v", result.LanguageModel.Capability)
	}
	if result.LanguageModel.Capability["lowModel"] != "vendor/current" {
		t.Fatalf("stamping the decision model rewrote a configured tier: %+v", result.LanguageModel.Capability)
	}
}

func TestModelMigrationLeavesCurrentAndExplicitEndpointConfigurationsUntouched(t *testing.T) {
	for _, document := range []string{
		`{"languageModel":{"capability":{"lowModel":"vendor/only-low"}}}`,
		`{"languageModel":{"tiers":{"low":[{"endpoint":"https://example.com/v1","model":"vendor/low"}]},"defaultProvider":"capabilityLLM","capability":{"model":"vendor/legacy"}}}`,
	} {
		var original, migrated map[string]any
		if errorValue := json.Unmarshal([]byte(document), &original); errorValue != nil {
			t.Fatal(errorValue)
		}
		if errorValue := json.Unmarshal([]byte(document), &migrated); errorValue != nil {
			t.Fatal(errorValue)
		}
		migrateBlueclawCapabilityModelConfiguration(migrated)
		if !reflect.DeepEqual(original, migrated) {
			t.Fatalf("migration rewrote a current configuration: %+v", migrated)
		}
	}
}
