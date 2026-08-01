package blueclaw

import (
	"encoding/json"
	"testing"
)

// A benchmark score only means something if every tier used the model the run
// claims to have used.
func TestBlueclawRuntimeConfigPinsOneModelAcrossEveryTier(t *testing.T) {
	const pinnedModelName = "openai/gpt-5.6-luna"
	t.Setenv(BlueclawTestModelEnvironment, pinnedModelName)

	options, errorValue := BlueclawRuntimeConfigOptionsFromEnvironment()
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if !options.ShouldUseModelForAllTiers {
		t.Fatal("expected a pinned test model to apply to every tier")
	}
	document, errorValue := BlueclawRuntimeConfigDocumentWithOptions(options)
	if errorValue != nil {
		t.Fatal(errorValue)
	}

	var runtimeConfiguration map[string]any
	if errorValue := json.Unmarshal([]byte(document), &runtimeConfiguration); errorValue != nil {
		t.Fatal(errorValue)
	}
	languageModel := runtimeConfiguration["languageModel"].(map[string]any)
	capability := languageModel["capability"].(map[string]any)

	for _, tierField := range []string{"model", "maxModel", "xhighModel", "highModel", "lowModel", "xlowModel", "codingModel"} {
		if capability[tierField] != pinnedModelName {
			t.Fatalf("expected %s to be %s, got %v", tierField, pinnedModelName, capability[tierField])
		}
	}
	if capability["mediumModel"] != BlueclawTestEscalationModelName {
		t.Fatalf("expected the escalation tier to stay %s, got %v", BlueclawTestEscalationModelName, capability["mediumModel"])
	}
}
