package llmbackend

import (
	"encoding/json"
	"testing"

	"gitlab.com/eastriver/internkim/internal/capabilities"
)

func multiToolActionDescriptors(t *testing.T) []capabilities.Descriptor {
	t.Helper()
	descriptors := []capabilities.Descriptor{}
	descriptors = append(descriptors, capabilities.FileDescriptors()...)
	descriptors = append(descriptors, capabilities.FlowDescriptors()...)
	descriptors = append(descriptors, capabilities.CalendarDescriptors()...)
	descriptors = append(descriptors, capabilities.WebDescriptors()...)
	if len(descriptors) <= formerDispatcherThreshold {
		t.Fatalf("need more tools than the former dispatcher threshold of %d to exercise the regression, got %d", formerDispatcherThreshold, len(descriptors))
	}
	return descriptors
}

const formerDispatcherThreshold = 12

func TestNativeActionToolsKeepPerToolSchemasInsteadOfDispatcher(t *testing.T) {
	descriptors := multiToolActionDescriptors(t)
	schema := StructuredOutputSchema{
		Name:     "blueclaw_agent_turn_action",
		Document: testActionSchemaForDescriptors(t, descriptors),
	}

	toolSet, isActionSchema, errorValue := nativeActionToolsForSchema(schema)
	if errorValue != nil || !isActionSchema {
		t.Fatalf("expected action schema, got isAction=%v error=%v", isActionSchema, errorValue)
	}

	if _, isDispatcher := toolSet.ToolByName["continue"]; isDispatcher {
		t.Fatalf("expected per-tool strict functions for %d tools, got the argument-less dispatcher", len(descriptors))
	}
	if len(toolSet.Tools) != len(descriptors) {
		t.Fatalf("expected one function per tool, got %d functions for %d tools", len(toolSet.Tools), len(descriptors))
	}
}

func TestNativeActionToolExposesOptionalToolInputProperties(t *testing.T) {
	descriptors := multiToolActionDescriptors(t)
	schema := StructuredOutputSchema{
		Name:     "blueclaw_agent_turn_action",
		Document: testActionSchemaForDescriptors(t, descriptors),
	}

	toolSet, _, errorValue := nativeActionToolsForSchema(schema)
	if errorValue != nil {
		t.Fatalf("native tools failed: %v", errorValue)
	}

	readTool, isFound := findNativeActionToolByName(toolSet, "document_read")
	if !isFound {
		t.Skip("document_read not present in descriptor set")
	}
	var parameters struct {
		Properties map[string]json.RawMessage `json:"properties"`
	}
	if errorValue := json.Unmarshal(readTool.Parameters, &parameters); errorValue != nil {
		t.Fatalf("document_read parameters parse failed: %v", errorValue)
	}
	for _, propertyName := range []string{"path", "maxPages", "maxOutputBytes"} {
		if _, isPresent := parameters.Properties[propertyName]; !isPresent {
			t.Fatalf("document_read native schema is missing %q so the model has no field to fill; got %v", propertyName, parameters.Properties)
		}
	}
}

func findNativeActionToolByName(toolSet nativeActionToolSet, toolName string) (nativeActionTool, bool) {
	for _, tool := range toolSet.Tools {
		if tool.ToolName == toolName {
			return tool, true
		}
	}
	return nativeActionTool{}, false
}
