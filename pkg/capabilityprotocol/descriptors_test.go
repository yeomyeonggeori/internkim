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
		OutputSchema:       strictSchema(ToolInvokeOutputSchema()),
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

func TestCanonicalizeDescriptorsRejectsUnresolvableSchemas(t *testing.T) {
	testCases := []struct {
		name         string
		mutateSchema func(*Descriptor)
	}{
		{
			name: "input",
			mutateSchema: func(descriptor *Descriptor) {
				descriptor.InputSchema = json.RawMessage(`{"type":"object","$ref":"#/$defs/missing","additionalProperties":false}`)
			},
		},
		{
			name: "result",
			mutateSchema: func(descriptor *Descriptor) {
				descriptor.ResultContract = &ToolResultContract{
					Schema: json.RawMessage(`{"type":"object","$ref":"#/$defs/missing","additionalProperties":false}`),
				}
			},
		},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			descriptor := validTestDescriptor("task.add")
			testCase.mutateSchema(&descriptor)
			if descriptors := CanonicalizeDescriptors([]Descriptor{descriptor}); descriptors != nil {
				t.Fatalf("expected unresolvable %s schema to fail closed: %#v", testCase.name, descriptors)
			}
		})
	}
}

func TestValidateDescriptorAcceptsCanonicalUnavailableStates(t *testing.T) {
	for _, state := range []string{CapabilityAvailable, CapabilityNotAllowed, CapabilityNotConnected, CapabilityNotReady} {
		descriptor := validTestDescriptor("task.add")
		descriptor.Availability.State = state
		if errorValue := ValidateDescriptor(descriptor); errorValue != nil {
			t.Fatalf("state %q: %v", state, errorValue)
		}
	}
}

func TestValidateDescriptorRejectsUnknownLatency(t *testing.T) {
	descriptor := validTestDescriptor("task.add")
	descriptor.EstimatedLatency = "instant"

	if errorValue := ValidateDescriptor(descriptor); errorValue == nil {
		t.Fatal("expected unknown latency rejection")
	}
}

func TestCanonicalizeDescriptorsValidatesResultContracts(t *testing.T) {
	validDescriptor := validTestDescriptor("task.add")
	validDescriptor.ResultContract = &ToolResultContract{
		Schema: json.RawMessage(`{"type":"object","properties":{"taskID":{"type":"string"}},"required":["taskID"]}`),
		Effects: []ResourceEffectContract{{
			ObjectType:     "task",
			Effect:         "created",
			ResultField:    "taskID",
			EffectIdentity: ResourceEffectIdentityID,
		}},
		EvidenceCondition: &EvidenceCondition{
			ResultField: "taskID",
			Equals:      json.RawMessage(`"task-1"`),
		},
	}
	descriptors := CanonicalizeDescriptors([]Descriptor{validDescriptor})
	if len(descriptors) != 1 || !isStrictSchema(descriptors[0].ResultContract.Schema) {
		t.Fatalf("expected strict result contract, got %+v", descriptors)
	}
	if descriptors[0].ResultContract.EvidenceCondition == validDescriptor.ResultContract.EvidenceCondition ||
		string(descriptors[0].ResultContract.EvidenceCondition.Equals) != `"task-1"` {
		t.Fatalf("expected canonical evidence condition copy, got %+v", descriptors[0].ResultContract.EvidenceCondition)
	}

	for _, contract := range []*ToolResultContract{
		{Schema: json.RawMessage(`{"type":"string"}`)},
		{Schema: json.RawMessage(`{"type":"object"}`), Effects: []ResourceEffectContract{{Effect: "created", ResultField: "taskID", EffectIdentity: ResourceEffectIdentityID}}},
		{Schema: json.RawMessage(`{"type":"object"}`), Effects: []ResourceEffectContract{{ObjectType: "task", Effect: "created", ResultField: "taskID", EffectIdentity: "unknown"}}},
		{Schema: json.RawMessage(`{"type":"object"}`), Effects: []ResourceEffectContract{{ObjectType: "task", Effect: "created", ResultField: "taskID", EffectIdentity: ResourceEffectIdentityID}}},
		{Schema: json.RawMessage(`{"type":"object"}`), Effects: []ResourceEffectContract{{ObjectType: "task", Effect: "created", ResultField: "taskID", EffectIdentity: ResourceEffectIdentityID}, {ObjectType: "task", Effect: "created", ResultField: "taskID", EffectIdentity: ResourceEffectIdentityID}}},
		{Schema: json.RawMessage(`{"type":"object","properties":{"passed":{"type":"boolean"}},"required":["passed"],"additionalProperties":false}`), EvidenceCondition: &EvidenceCondition{ResultField: "missing", Equals: json.RawMessage(`true`)}},
		{Schema: json.RawMessage(`{"type":"object","properties":{"passed":{"type":"boolean"}},"additionalProperties":false}`), EvidenceCondition: &EvidenceCondition{ResultField: "passed", Equals: json.RawMessage(`true`)}},
		{Schema: json.RawMessage(`{"type":"object","properties":{"passed":{"type":"boolean"}},"required":["passed"],"additionalProperties":false}`), EvidenceCondition: &EvidenceCondition{ResultField: "passed"}},
		{Schema: json.RawMessage(`{"type":"object","properties":{"passed":{"type":"boolean"}},"required":["passed"],"additionalProperties":false}`), EvidenceCondition: &EvidenceCondition{ResultField: "passed", Equals: json.RawMessage(`"true"`)}},
	} {
		descriptor := validTestDescriptor("task.add")
		descriptor.ResultContract = contract
		if canonicalDescriptors := CanonicalizeDescriptors([]Descriptor{descriptor}); canonicalDescriptors != nil {
			t.Fatalf("expected invalid result contract to fail closed: %+v", canonicalDescriptors)
		}
	}
}

func TestValidateResultContractAcceptsOnlyCanonicalArrayEffectIdentities(t *testing.T) {
	effect := ResourceEffectContract{
		ObjectType:     "file",
		Effect:         "updated",
		ResultField:    "paths",
		EffectIdentity: ResourceEffectIdentityPath,
	}
	validContract := &ToolResultContract{
		Schema:  json.RawMessage(`{"type":"object","properties":{"paths":{"type":"array","items":{"type":"string"},"minItems":1,"uniqueItems":true}},"required":["paths"],"additionalProperties":false}`),
		Effects: []ResourceEffectContract{effect},
	}
	if errorValue := validateResultContract(validContract); errorValue != nil {
		t.Fatalf("expected canonical string array identity, got %v", errorValue)
	}
	for _, schema := range []json.RawMessage{
		json.RawMessage(`{"type":"object","properties":{"paths":{"type":"array","items":{"type":"string"},"uniqueItems":true}},"required":["paths"],"additionalProperties":false}`),
		json.RawMessage(`{"type":"object","properties":{"paths":{"type":"array","items":{"type":"string"},"minItems":1}},"required":["paths"],"additionalProperties":false}`),
		json.RawMessage(`{"type":"object","properties":{"paths":{"type":"array","items":{"type":"number"},"minItems":1,"uniqueItems":true}},"required":["paths"],"additionalProperties":false}`),
	} {
		contract := &ToolResultContract{Schema: schema, Effects: []ResourceEffectContract{effect}}
		if errorValue := validateResultContract(contract); errorValue == nil {
			t.Fatalf("expected noncanonical array identity rejection for %s", schema)
		}
	}
}

func TestValidateModelVisibleCapabilityDescriptorSetRequiresResultContracts(t *testing.T) {
	modelVisibleDescriptor := validTestDescriptor("task.add")
	if errorValue := ValidateDescriptorSet([]Descriptor{modelVisibleDescriptor}); errorValue != nil {
		t.Fatalf("expected the shared descriptor validator to preserve legacy provider compatibility: %v", errorValue)
	}
	if errorValue := ValidateModelVisibleCapabilityDescriptorSet([]Descriptor{modelVisibleDescriptor}); errorValue == nil ||
		!strings.Contains(errorValue.Error(), "model-visible capability resultContract is required") {
		t.Fatalf("expected generated capability result contract rejection, got %v", errorValue)
	}

	hiddenDescriptor := validTestDescriptor("llm.text")
	hiddenDescriptor.ModelVisibility = ModelVisibilityHidden
	hiddenDescriptor.ModelVisible = false
	if errorValue := ValidateModelVisibleCapabilityDescriptorSet([]Descriptor{hiddenDescriptor}); errorValue != nil {
		t.Fatalf("expected hidden capability without a result contract: %v", errorValue)
	}

	modelVisibleDescriptor.ResultContract = &ToolResultContract{
		Schema: json.RawMessage(`{"type":"object","properties":{"taskID":{"type":"string"}},"required":["taskID"],"additionalProperties":false}`),
	}
	if errorValue := ValidateModelVisibleCapabilityDescriptorSet([]Descriptor{modelVisibleDescriptor}); errorValue != nil {
		t.Fatalf("expected typed model-visible capability: %v", errorValue)
	}
}

func TestMustCanonicalizeModelVisibleDescriptorsFailsClosed(t *testing.T) {
	modelVisibleDescriptor := validTestDescriptor("task.add")
	defer func() {
		if recover() == nil {
			t.Fatal("expected model-visible descriptor without a result contract to panic")
		}
	}()
	MustCanonicalizeModelVisibleDescriptors([]Descriptor{modelVisibleDescriptor})
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
		InputSchema:        json.RawMessage(`{"type":"object","properties":{},"additionalProperties":false}`),
		OutputSchema:       strictSchema(ToolInvokeOutputSchema()),
		InputSchemaStrict:  true,
		OutputSchemaStrict: true,
		PolicyResource:     "tool:" + name,
		SideEffectClass:    SideEffectWorkspaceWrite,
		SideEffect:         SideEffectWorkspaceWrite,
		Availability:       AvailabilityMetadata{State: AvailabilityOK},
		Idempotency:        IdempotencyMetadata{Scope: "operation"},
	}
}
