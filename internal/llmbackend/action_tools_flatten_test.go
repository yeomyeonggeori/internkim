package llmbackend

import (
	"encoding/json"
	"testing"
)

func TestFlattenToolInputSchemaMovesToolPropertiesToTopLevel(t *testing.T) {
	toolInputSchema := json.RawMessage(`{"type":"object","properties":{"command":{"type":"string"}},"required":["command"]}`)

	properties, required := flattenToolInputSchema(toolInputSchema)

	if _, isFound := properties["command"]; !isFound {
		t.Fatalf("expected command property, got %+v", properties)
	}
	if _, isFound := properties["toolInput"]; isFound {
		t.Fatalf("expected no toolInput property, got %+v", properties)
	}
	if !requiredContains(required, "command") {
		t.Fatalf("expected command to be required, got %+v", required)
	}
}

func TestToolActionParametersFlattenTerminalRunInput(t *testing.T) {
	variant := actionSchemaVariant{
		Properties: map[string]json.RawMessage{
			"action":               json.RawMessage(`{"type":"string","enum":["continue"]}`),
			"toolName":             json.RawMessage(`{"type":"string","enum":["terminal_run"]}`),
			"toolInput":            json.RawMessage(`{"type":"object","properties":{"command":{"type":"string"}},"required":["command"]}`),
			"message":              json.RawMessage(`{"type":"string"}`),
			"reason":               json.RawMessage(`{"type":"string"}`),
			"goalStatus":           json.RawMessage(`{"type":"string","enum":["in_progress"]}`),
			"goalSatisfied":        json.RawMessage(`{"type":"boolean"}`),
			"remainingWork":        json.RawMessage(`{"type":"string"}`),
			"executionStateUpdate": json.RawMessage(`{"type":"object","properties":{}}`),
			"requestTools":         json.RawMessage(`{"type":"array","items":{"type":"string"}}`),
			"requestSkills":        json.RawMessage(`{"type":"array","items":{"type":"string"}}`),
		},
		Required: []string{"action", "toolName", "toolInput", "executionStateUpdate"},
	}

	parameters := mustToolActionParameters(t, variant)
	properties := parameters["properties"].(map[string]any)

	if _, isFound := properties["command"]; !isFound {
		t.Fatalf("expected command as top-level property, got %+v", parameters)
	}
	if _, isFound := properties["toolInput"]; isFound {
		t.Fatalf("expected no toolInput property, got %+v", parameters)
	}
	if _, isFound := properties["blueclawMessage"]; !isFound {
		t.Fatalf("expected blueclawMessage planning field, got %+v", parameters)
	}
	for _, fieldName := range []string{
		"blueclawReason",
		"blueclawGoalStatus",
		"blueclawGoalSatisfied",
		"blueclawRemainingWork",
		"blueclawExecutionStateUpdate",
	} {
		if _, isFound := properties[fieldName]; isFound {
			t.Fatalf("expected planning field %s to be omitted from per-tool continue schema, got %+v", fieldName, parameters)
		}
	}
	for _, fieldName := range []string{"blueclawRequestTools", "blueclawRequestSkills"} {
		if _, isFound := properties[fieldName]; !isFound {
			t.Fatalf("expected kept planning field %s in per-tool continue schema, got %+v", fieldName, parameters)
		}
	}
	required := parameters["required"].([]any)
	if !requiredContains(required, "command") {
		t.Fatalf("expected command to be required, got %+v", required)
	}
	if requiredContains(required, "blueclawExecutionStateUpdate") {
		t.Fatalf("expected omitted blueclawExecutionStateUpdate not to be required, got %+v", required)
	}
}

func TestReconstructActionFromFlatArgumentsSplitsToolInputAndPlanningFields(t *testing.T) {
	flatArguments := map[string]json.RawMessage{
		"command":                      json.RawMessage(`"go test"`),
		"blueclawMessage":              json.RawMessage(`"running tests"`),
		"blueclawReason":               json.RawMessage(`"verify the change"`),
		"blueclawExecutionStateUpdate": json.RawMessage(`{}`),
		"blueclawRequestTools":         json.RawMessage(`["web_search"]`),
		"blueclawGoalStatus":           json.RawMessage(`"in_progress"`),
		"blueclawGoalSatisfied":        json.RawMessage(`false`),
		"blueclawRemainingWork":        json.RawMessage(`"none"`),
	}

	toolInput, planningFields := reconstructActionFromFlatArguments(flatArguments, nativeBlueclawPlanningArgumentNames())

	if toolInput["command"] != "go test" {
		t.Fatalf("expected command in toolInput, got %+v", toolInput)
	}
	if _, isFound := toolInput["blueclawExecutionStateUpdate"]; isFound {
		t.Fatalf("expected no blueclaw fields in toolInput, got %+v", toolInput)
	}
	if planningFields["message"] != "running tests" {
		t.Fatalf("expected message planning field, got %+v", planningFields)
	}
	if _, isFound := planningFields["executionStateUpdate"]; !isFound {
		t.Fatalf("expected executionStateUpdate planning field, got %+v", planningFields)
	}
	if _, isFound := planningFields["requestTools"]; !isFound {
		t.Fatalf("expected requestTools planning field, got %+v", planningFields)
	}
}

func TestControlActionParametersDoNotGainToolInput(t *testing.T) {
	for _, actionName := range []string{"finish", "fail"} {
		variant := actionSchemaVariant{
			Properties: map[string]json.RawMessage{
				"action":               json.RawMessage(`{"type":"string","enum":["` + actionName + `"]}`),
				"message":              json.RawMessage(`{"type":"string"}`),
				"executionStateUpdate": json.RawMessage(`{"type":"object","properties":{}}`),
			},
			Required: []string{"action", "message", "executionStateUpdate"},
		}

		parameters := mustControlActionParameters(t, variant)
		properties := parameters["properties"].(map[string]any)

		if _, isFound := properties["toolInput"]; isFound {
			t.Fatalf("expected %s parameters to omit toolInput, got %+v", actionName, parameters)
		}
		if _, isFound := properties["message"]; !isFound {
			t.Fatalf("expected %s parameters to keep message, got %+v", actionName, parameters)
		}
	}
}

func mustToolActionParameters(t *testing.T, variant actionSchemaVariant) map[string]any {
	t.Helper()
	content, errorValue := toolActionParameters(variant)
	if errorValue != nil {
		t.Fatalf("expected tool parameters: %v", errorValue)
	}
	return mustJSONDocument(t, content)
}

func mustControlActionParameters(t *testing.T, variant actionSchemaVariant) map[string]any {
	t.Helper()
	content, errorValue := controlActionParameters(variant)
	if errorValue != nil {
		t.Fatalf("expected control parameters: %v", errorValue)
	}
	return mustJSONDocument(t, content)
}

func mustJSONDocument(t *testing.T, content json.RawMessage) map[string]any {
	t.Helper()
	var document map[string]any
	if errorValue := json.Unmarshal(content, &document); errorValue != nil {
		t.Fatalf("expected json document: %v", errorValue)
	}
	return document
}
