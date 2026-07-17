package capabilityprotocol

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestCanonicalizeDescriptorsBuildsStrictProviderMetadata(t *testing.T) {
	descriptors := CanonicalizeDescriptors([]Descriptor{{
		Name:               "task.add",
		CanonicalName:      "task.add",
		Namespace:          "task",
		ModelName:          "task.add",
		ModelVisibility:    ModelVisibilityVisible,
		ModelVisible:       true,
		Description:        "Create a task.",
		Version:            "2",
		PrivacyClass:       "workspace_task",
		EstimatedLatency:   "medium",
		InputSchema:        json.RawMessage(`{"type":"object","properties":{"nested":{"type":"object","properties":{"title":{"type":"string"}}},"labels":{"type":"object","additionalProperties":{"type":"string"}}}}`),
		OutputSchema:       ToolInvokeOutputSchema(),
		InputSchemaStrict:  true,
		OutputSchemaStrict: true,
		PolicyResource:     "tool:task.add",
		SideEffectClass:    "workspace_write",
		SideEffect:         "workspace_write",
		Availability:       AvailabilityMetadata{State: AvailabilityOK},
		Idempotency:        IdempotencyMetadata{Scope: "operation"},
		CompletionEvidence: &CompletionEvidenceDescriptor{
			Mode:       "success",
			Action:     "write_task",
			TargetKind: "task",
		},
	}})
	if len(descriptors) != 1 {
		t.Fatalf("expected one canonical descriptor, got %#v", descriptors)
	}
	descriptor := descriptors[0]
	if descriptor.Name != "task.add" || descriptor.CanonicalName != "task.add" || descriptor.Namespace != "task" || descriptor.ModelName != "task.add" {
		t.Fatalf("unexpected canonical identity: %+v", descriptor)
	}
	if descriptor.ModelVisibility != ModelVisibilityVisible || !descriptor.ModelVisible {
		t.Fatalf("unexpected model visibility: %+v", descriptor)
	}
	if !descriptor.InputSchemaStrict || !descriptor.OutputSchemaStrict || descriptor.SideEffect != "workspace_write" || descriptor.PolicyResource != "tool:task.add" {
		t.Fatalf("unexpected provider metadata: %+v", descriptor)
	}
	if descriptor.Availability.State != AvailabilityOK || descriptor.Idempotency.Supported || descriptor.Idempotency.Scope != "operation" {
		t.Fatalf("unexpected availability or idempotency metadata: %+v", descriptor)
	}
	var inputSchema map[string]any
	if errorValue := json.Unmarshal(descriptor.InputSchema, &inputSchema); errorValue != nil {
		t.Fatal(errorValue)
	}
	nestedSchema := inputSchema["properties"].(map[string]any)["nested"].(map[string]any)
	if nestedSchema["additionalProperties"] != false {
		t.Fatalf("expected nested object to be closed: %#v", nestedSchema)
	}
	labelsSchema := inputSchema["properties"].(map[string]any)["labels"].(map[string]any)
	if _, isMap := labelsSchema["additionalProperties"].(map[string]any); !isMap {
		t.Fatalf("expected explicit schema map to remain intact: %#v", labelsSchema)
	}
	if errorValue := ValidateDescriptorSet(descriptors); errorValue != nil {
		t.Fatal(errorValue)
	}
}

func TestDescriptorValidationRejectsMissingMetadataAndDuplicates(t *testing.T) {
	if errorValue := ValidateDescriptor(Descriptor{Name: "task.add", Description: "Create a task.", Version: "1", PrivacyClass: "task", EstimatedLatency: "low"}); errorValue == nil || !strings.Contains(errorValue.Error(), "canonicalName") {
		t.Fatalf("expected missing canonical name error, got %v", errorValue)
	}
	firstDescriptor := validTestDescriptor("task.add")
	secondDescriptor := validTestDescriptor("task.update")
	secondDescriptor.CanonicalName = "task.add"
	if descriptors := CanonicalizeDescriptors([]Descriptor{firstDescriptor, secondDescriptor}); descriptors != nil {
		t.Fatalf("expected duplicate canonical names to fail closed: %#v", descriptors)
	}
	firstDescriptor = validTestDescriptor("task.add")
	secondDescriptor = validTestDescriptor("task.update")
	firstDescriptor.ModelName = "task"
	secondDescriptor.ModelName = "task"
	if descriptors := CanonicalizeDescriptors([]Descriptor{firstDescriptor, secondDescriptor}); descriptors != nil {
		t.Fatalf("expected duplicate model names to fail closed: %#v", descriptors)
	}
	firstDescriptor = validTestDescriptor("task.add")
	secondDescriptor = validTestDescriptor("task.add")
	secondDescriptor.CanonicalName = "task.update"
	if descriptors := CanonicalizeDescriptors([]Descriptor{firstDescriptor, secondDescriptor}); descriptors != nil {
		t.Fatalf("expected duplicate names to fail closed: %#v", descriptors)
	}
}

func TestCanonicalizeDescriptorsDoesNotInferSemanticMetadata(t *testing.T) {
	missingSideEffect := validTestDescriptor("task.add")
	missingSideEffect.SideEffectClass = ""
	missingSideEffect.SideEffect = ""
	if descriptors := CanonicalizeDescriptors([]Descriptor{missingSideEffect}); descriptors != nil {
		t.Fatalf("expected missing side effect to fail closed: %#v", descriptors)
	}

	llmDescriptor := validTestDescriptor("llm.text")
	llmDescriptor.SideEffectClass = SideEffectComputation
	llmDescriptor.SideEffect = SideEffectComputation
	descriptors := CanonicalizeDescriptors([]Descriptor{llmDescriptor})
	if len(descriptors) != 1 {
		t.Fatalf("expected valid descriptor, got %#v", descriptors)
	}
	if descriptors[0].ModelVisibility != ModelVisibilityVisible {
		t.Fatalf("expected model visibility to remain explicit, got %+v", descriptors[0])
	}
	if descriptors[0].Idempotency.Supported {
		t.Fatalf("expected idempotency support to remain explicit, got %+v", descriptors[0])
	}
}

func TestCanonicalizeDescriptorsRejectsMissingSchemasAndDescription(t *testing.T) {
	testCases := []struct {
		name       string
		descriptor Descriptor
	}{
		{name: "description", descriptor: func() Descriptor {
			descriptor := validTestDescriptor("task.add")
			descriptor.Description = ""
			return descriptor
		}()},
		{name: "input schema", descriptor: func() Descriptor {
			descriptor := validTestDescriptor("task.add")
			descriptor.InputSchema = nil
			return descriptor
		}()},
		{name: "output schema", descriptor: func() Descriptor {
			descriptor := validTestDescriptor("task.add")
			descriptor.OutputSchema = nil
			return descriptor
		}()},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			if descriptors := CanonicalizeDescriptors([]Descriptor{testCase.descriptor}); descriptors != nil {
				t.Fatalf("expected missing %s to fail closed: %#v", testCase.name, descriptors)
			}
		})
	}
}

func TestCanonicalizeDescriptorsRejectsOpenObjectSchema(t *testing.T) {
	descriptor := validTestDescriptor("task.add")
	descriptor.InputSchema = json.RawMessage(`{"type":"object","additionalProperties":true}`)

	if descriptors := CanonicalizeDescriptors([]Descriptor{descriptor}); descriptors != nil {
		t.Fatalf("expected open object schema to fail closed: %#v", descriptors)
	}
}

func TestCanonicalDescriptorGroupsValidate(t *testing.T) {
	for _, descriptors := range [][]Descriptor{CompanionToolDescriptors(), CompanionLLMDescriptors(), DeviceBrowserDescriptors()} {
		if errorValue := ValidateDescriptorSet(descriptors); errorValue != nil {
			t.Fatal(errorValue)
		}
	}
}

func TestBuiltInDescriptorsExposeCanonicalProviderMetadata(t *testing.T) {
	for _, descriptors := range [][]Descriptor{CompanionToolDescriptors(), CompanionLLMDescriptors(), DeviceBrowserDescriptors()} {
		for _, descriptor := range descriptors {
			if descriptor.Name == "" || descriptor.CanonicalName == "" || descriptor.Namespace == "" || descriptor.ModelName == "" {
				t.Fatalf("descriptor identity is incomplete: %+v", descriptor)
			}
			if descriptor.ModelVisible != (descriptor.ModelVisibility == ModelVisibilityVisible) {
				t.Fatalf("descriptor model visibility is inconsistent: %+v", descriptor)
			}
			if !descriptor.InputSchemaStrict || !descriptor.OutputSchemaStrict || len(descriptor.InputSchema) == 0 || len(descriptor.OutputSchema) == 0 {
				t.Fatalf("descriptor schemas are not strict: %+v", descriptor)
			}
			if descriptor.SideEffectClass == "" || descriptor.SideEffect == "" || descriptor.PolicyResource == "" {
				t.Fatalf("descriptor provider metadata is incomplete: %+v", descriptor)
			}
			if descriptor.Availability.State != AvailabilityOK || descriptor.Idempotency.Scope == "" {
				t.Fatalf("descriptor runtime metadata is incomplete: %+v", descriptor)
			}
		}
	}
}

func validTestDescriptor(name string) Descriptor {
	return Descriptor{
		Name:               name,
		CanonicalName:      name,
		Namespace:          "task",
		ModelName:          name,
		ModelVisibility:    ModelVisibilityVisible,
		ModelVisible:       true,
		Description:        "Execute " + name + ".",
		Version:            "1",
		PrivacyClass:       "task",
		EstimatedLatency:   "low",
		InputSchema:        json.RawMessage(`{"type":"object","properties":{}}`),
		OutputSchema:       ToolInvokeOutputSchema(),
		InputSchemaStrict:  true,
		OutputSchemaStrict: true,
		PolicyResource:     "tool:" + name,
		SideEffectClass:    SideEffectWorkspaceWrite,
		SideEffect:         SideEffectWorkspaceWrite,
		Availability:       AvailabilityMetadata{State: AvailabilityOK},
		Idempotency:        IdempotencyMetadata{Scope: "operation"},
	}
}
