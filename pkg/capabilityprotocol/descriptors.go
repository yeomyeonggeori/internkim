package capabilityprotocol

import (
	"bytes"
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	"github.com/google/jsonschema-go/jsonschema"
)

const (
	ModelVisibilityVisible = "visible"
	ModelVisibilityHidden  = "hidden"
	AvailabilityOK         = CapabilityAvailable
)

const (
	SideEffectApproval        = "approval"
	SideEffectComputation     = "computation"
	SideEffectConnect         = "connect"
	SideEffectDestructive     = "destructive"
	SideEffectExternalPublish = "external_publish"
	SideEffectExternalSend    = "external_send"
	SideEffectExternalWrite   = "external_write"
	SideEffectLocalFile       = "local_file"
	SideEffectPlatformReply   = "platform_reply"
	SideEffectRead            = "read"
	SideEffectSitePublish     = "site_publish"
	SideEffectWorkspaceWrite  = "workspace_write"
)

var sideEffectClasses = map[string]struct{}{
	SideEffectApproval:        {},
	SideEffectComputation:     {},
	SideEffectConnect:         {},
	SideEffectDestructive:     {},
	SideEffectExternalPublish: {},
	SideEffectExternalSend:    {},
	SideEffectExternalWrite:   {},
	SideEffectLocalFile:       {},
	SideEffectPlatformReply:   {},
	SideEffectRead:            {},
	SideEffectSitePublish:     {},
	SideEffectWorkspaceWrite:  {},
}

var estimatedLatencies = map[string]struct{}{
	"low":         {},
	"medium":      {},
	"high":        {},
	"interactive": {},
}

var availabilityStates = map[string]struct{}{
	CapabilityAvailable:    {},
	CapabilityNotAllowed:   {},
	CapabilityNotConnected: {},
	CapabilityNotReady:     {},
}

type DescriptorIdentity struct {
	Name            string
	CanonicalName   string
	Namespace       string
	ModelName       string
	ModelVisibility string
}

type DescriptorMetadata struct {
	Description          string
	Version              string
	PrivacyClass         string
	EstimatedLatency     string
	RequiresUserPresence bool
	WorksOffline         bool
	InputSchema          json.RawMessage
	OutputSchema         json.RawMessage
	ResultContract       *ToolResultContract
	PolicyResource       string
	SideEffect           string
	RequiresApproval     bool
	CompletionEvidence   *CompletionEvidenceDescriptor
	Availability         AvailabilityMetadata
	Idempotency          IdempotencyMetadata
}

type DescriptorDefinition struct {
	Identity DescriptorIdentity
	Metadata DescriptorMetadata
}

func NewDescriptor(definition DescriptorDefinition) Descriptor {
	descriptor := Descriptor{
		Name:                 definition.Identity.Name,
		CanonicalName:        definition.Identity.CanonicalName,
		Namespace:            definition.Identity.Namespace,
		ModelName:            definition.Identity.ModelName,
		ModelVisibility:      definition.Identity.ModelVisibility,
		ModelVisible:         definition.Identity.ModelVisibility == ModelVisibilityVisible,
		Description:          definition.Metadata.Description,
		Version:              definition.Metadata.Version,
		PrivacyClass:         definition.Metadata.PrivacyClass,
		EstimatedLatency:     definition.Metadata.EstimatedLatency,
		RequiresUserPresence: definition.Metadata.RequiresUserPresence,
		WorksOffline:         definition.Metadata.WorksOffline,
		InputSchema:          strictSchema(definition.Metadata.InputSchema),
		OutputSchema:         strictSchema(definition.Metadata.OutputSchema),
		InputSchemaStrict:    true,
		OutputSchemaStrict:   true,
		ResultContract:       canonicalResultContract(definition.Metadata.ResultContract),
		PolicyResource:       definition.Metadata.PolicyResource,
		SideEffectClass:      definition.Metadata.SideEffect,
		SideEffect:           definition.Metadata.SideEffect,
		RequiresApproval:     definition.Metadata.RequiresApproval,
		CompletionEvidence:   definition.Metadata.CompletionEvidence,
		Availability:         definition.Metadata.Availability,
		Idempotency:          definition.Metadata.Idempotency,
	}
	if errorValue := ValidateDescriptor(descriptor); errorValue != nil {
		panic(errorValue)
	}
	return descriptor
}

func CanonicalizeDescriptors(descriptors []Descriptor) []Descriptor {
	canonicalDescriptors := make([]Descriptor, len(descriptors))
	copy(canonicalDescriptors, descriptors)
	canonicalizeSchemas(canonicalDescriptors)
	if ValidateDescriptorSet(canonicalDescriptors) != nil {
		return nil
	}
	return canonicalDescriptors
}

func MustCanonicalizeDescriptors(descriptors []Descriptor) []Descriptor {
	canonicalDescriptors := make([]Descriptor, len(descriptors))
	copy(canonicalDescriptors, descriptors)
	canonicalizeSchemas(canonicalDescriptors)
	if errorValue := ValidateDescriptorSet(canonicalDescriptors); errorValue != nil {
		panic(errorValue)
	}
	return canonicalDescriptors
}

func MustCanonicalizeBuiltInDescriptors(descriptors []Descriptor) []Descriptor {
	return MustCanonicalizeDescriptors(descriptors)
}

func MustCanonicalizeModelVisibleDescriptors(descriptors []Descriptor) []Descriptor {
	canonicalDescriptors := make([]Descriptor, len(descriptors))
	copy(canonicalDescriptors, descriptors)
	canonicalizeSchemas(canonicalDescriptors)
	if errorValue := ValidateModelVisibleCapabilityDescriptorSet(canonicalDescriptors); errorValue != nil {
		panic(errorValue)
	}
	return canonicalDescriptors
}

func canonicalizeSchemas(descriptors []Descriptor) {
	for index := range descriptors {
		descriptors[index].InputSchema = strictSchema(descriptors[index].InputSchema)
		descriptors[index].OutputSchema = strictSchema(descriptors[index].OutputSchema)
		descriptors[index].ResultContract = canonicalResultContract(descriptors[index].ResultContract)
	}
}

func canonicalResultContract(contract *ToolResultContract) *ToolResultContract {
	if contract == nil {
		return nil
	}
	return &ToolResultContract{
		Schema:            strictSchema(contract.Schema),
		Effects:           append([]ResourceEffectContract{}, contract.Effects...),
		EvidenceCondition: canonicalEvidenceCondition(contract.EvidenceCondition),
	}
}

func canonicalEvidenceCondition(condition *EvidenceCondition) *EvidenceCondition {
	if condition == nil {
		return nil
	}
	return &EvidenceCondition{
		ResultField: strings.TrimSpace(condition.ResultField),
		Equals:      append(json.RawMessage{}, condition.Equals...),
	}
}

func ValidateDescriptorSet(descriptors []Descriptor) error {
	names := map[string]string{}
	canonicalNames := map[string]string{}
	modelNames := map[string]string{}
	for index, descriptor := range descriptors {
		if errorValue := ValidateDescriptor(descriptor); errorValue != nil {
			return fmt.Errorf("descriptor %d: %w", index, errorValue)
		}
		if previous, found := names[descriptor.Name]; found {
			return fmt.Errorf("duplicate name %q for %s and %s", descriptor.Name, previous, descriptor.CanonicalName)
		}
		names[descriptor.Name] = descriptor.CanonicalName
		if previous, found := canonicalNames[descriptor.CanonicalName]; found {
			return fmt.Errorf("duplicate canonical name %q for %s and %s", descriptor.CanonicalName, previous, descriptor.Name)
		}
		canonicalNames[descriptor.CanonicalName] = descriptor.Name
		if previous, found := modelNames[descriptor.ModelName]; found {
			return fmt.Errorf("duplicate model name %q for %s and %s", descriptor.ModelName, previous, descriptor.Name)
		}
		modelNames[descriptor.ModelName] = descriptor.Name
	}
	return nil
}

func ValidateModelVisibleCapabilityDescriptorSet(descriptors []Descriptor) error {
	if errorValue := ValidateDescriptorSet(descriptors); errorValue != nil {
		return errorValue
	}
	for index, descriptor := range descriptors {
		if descriptor.ModelVisibility == ModelVisibilityVisible && descriptor.ResultContract == nil {
			return fmt.Errorf("descriptor %d: model-visible capability resultContract is required", index)
		}
	}
	return nil
}

func ValidateDescriptor(descriptor Descriptor) error {
	if strings.TrimSpace(descriptor.Name) == "" {
		return fmt.Errorf("name is required")
	}
	if descriptor.Name != strings.TrimSpace(descriptor.Name) {
		return fmt.Errorf("name must not have surrounding whitespace")
	}
	if strings.TrimSpace(descriptor.Version) == "" {
		return fmt.Errorf("version is required")
	}
	if strings.TrimSpace(descriptor.PrivacyClass) == "" {
		return fmt.Errorf("privacyClass is required")
	}
	if _, found := estimatedLatencies[descriptor.EstimatedLatency]; !found {
		return fmt.Errorf("estimatedLatency is invalid")
	}
	if strings.TrimSpace(descriptor.Description) == "" {
		return fmt.Errorf("description is required")
	}
	if strings.TrimSpace(descriptor.CanonicalName) == "" {
		return fmt.Errorf("canonicalName is required")
	}
	if descriptor.CanonicalName != strings.TrimSpace(descriptor.CanonicalName) {
		return fmt.Errorf("canonicalName must not have surrounding whitespace")
	}
	if strings.TrimSpace(descriptor.Namespace) == "" {
		return fmt.Errorf("namespace is required")
	}
	if descriptor.Namespace != strings.TrimSpace(descriptor.Namespace) {
		return fmt.Errorf("namespace must not have surrounding whitespace")
	}
	if strings.TrimSpace(descriptor.ModelName) == "" {
		return fmt.Errorf("modelName is required")
	}
	if descriptor.ModelName != strings.TrimSpace(descriptor.ModelName) {
		return fmt.Errorf("modelName must not have surrounding whitespace")
	}
	if descriptor.ModelVisibility != ModelVisibilityVisible && descriptor.ModelVisibility != ModelVisibilityHidden {
		return fmt.Errorf("modelVisibility must be visible or hidden")
	}
	if descriptor.ModelVisible != (descriptor.ModelVisibility == ModelVisibilityVisible) {
		return fmt.Errorf("modelVisible does not match modelVisibility")
	}
	if !descriptor.InputSchemaStrict || !isStrictSchema(descriptor.InputSchema) {
		return fmt.Errorf("inputSchema must be a strict object schema")
	}
	if errorValue := resolveDescriptorSchema(descriptor.InputSchema); errorValue != nil {
		return fmt.Errorf("inputSchema cannot be resolved: %w", errorValue)
	}
	if !descriptor.OutputSchemaStrict || !isStrictSchema(descriptor.OutputSchema) {
		return fmt.Errorf("outputSchema must be a strict object schema")
	}
	if errorValue := resolveDescriptorSchema(descriptor.OutputSchema); errorValue != nil {
		return fmt.Errorf("outputSchema cannot be resolved: %w", errorValue)
	}
	if errorValue := validateResultContract(descriptor.ResultContract); errorValue != nil {
		return errorValue
	}
	if _, found := sideEffectClasses[descriptor.SideEffect]; !found {
		return fmt.Errorf("sideEffect %q is invalid", descriptor.SideEffect)
	}
	if descriptor.SideEffectClass != descriptor.SideEffect {
		return fmt.Errorf("sideEffectClass must match sideEffect")
	}
	if strings.TrimSpace(descriptor.PolicyResource) == "" {
		return fmt.Errorf("policyResource is required")
	}
	if _, found := availabilityStates[descriptor.Availability.State]; !found {
		return fmt.Errorf("availability.state is invalid")
	}
	if strings.TrimSpace(descriptor.Idempotency.Scope) == "" {
		return fmt.Errorf("idempotency.scope is required")
	}
	return nil
}

func validateResultContract(contract *ToolResultContract) error {
	if contract == nil {
		return nil
	}
	if !isStrictSchema(contract.Schema) {
		return fmt.Errorf("resultContract.schema must be a strict object schema")
	}
	if errorValue := resolveDescriptorSchema(contract.Schema); errorValue != nil {
		return fmt.Errorf("resultContract.schema cannot be resolved: %w", errorValue)
	}
	if errorValue := validateEvidenceCondition(contract.Schema, contract.EvidenceCondition); errorValue != nil {
		return errorValue
	}
	seenEffects := map[string]bool{}
	for _, effectContract := range contract.Effects {
		objectType := strings.TrimSpace(effectContract.ObjectType)
		effect := strings.TrimSpace(effectContract.Effect)
		resultField := strings.TrimSpace(effectContract.ResultField)
		if objectType == "" || effect == "" || resultField == "" {
			return fmt.Errorf("resultContract effect must include objectType, effect, and resultField")
		}
		if effectContract.EffectIdentity != ResourceEffectIdentityID &&
			effectContract.EffectIdentity != ResourceEffectIdentityPath &&
			effectContract.EffectIdentity != ResourceEffectIdentityURL {
			return fmt.Errorf("resultContract effectIdentity is invalid")
		}
		if !schemaRequiresEffectIdentityField(contract.Schema, resultField) {
			return fmt.Errorf("resultContract resultField must name a required string or nonempty unique string array property")
		}
		effectKey := objectType + "\x00" + effect
		if seenEffects[effectKey] {
			return fmt.Errorf("resultContract effect is duplicated")
		}
		seenEffects[effectKey] = true
	}
	return nil
}

func validateEvidenceCondition(schema json.RawMessage, condition *EvidenceCondition) error {
	if condition == nil {
		return nil
	}
	resultField := strings.TrimSpace(condition.ResultField)
	if len(bytes.TrimSpace(condition.Equals)) == 0 || !json.Valid(condition.Equals) {
		return fmt.Errorf("resultContract evidenceCondition.equals must be valid JSON")
	}
	if !schemaAcceptsEvidenceValue(schema, resultField, condition.Equals) {
		return fmt.Errorf("resultContract evidenceCondition must match a required result property")
	}
	return nil
}

func schemaAcceptsEvidenceValue(document json.RawMessage, fieldName string, value json.RawMessage) bool {
	var schema jsonschema.Schema
	if json.Unmarshal(document, &schema) != nil || !slices.Contains(schema.Required, fieldName) {
		return false
	}
	property, isDefined := schema.Properties[fieldName]
	if !isDefined {
		return false
	}
	var instance any
	if json.Unmarshal(value, &instance) != nil {
		return false
	}
	resolvedProperty, errorValue := property.Resolve(nil)
	return errorValue == nil && resolvedProperty.Validate(instance) == nil
}

func resolveDescriptorSchema(document json.RawMessage) error {
	var schema jsonschema.Schema
	if errorValue := json.Unmarshal(document, &schema); errorValue != nil {
		return errorValue
	}
	_, errorValue := schema.Resolve(nil)
	return errorValue
}

func schemaRequiresEffectIdentityField(document json.RawMessage, fieldName string) bool {
	var schema struct {
		Properties map[string]json.RawMessage `json:"properties"`
		Required   []string                   `json:"required"`
	}
	if json.Unmarshal(document, &schema) != nil {
		return false
	}
	var property struct {
		Type        string `json:"type"`
		MinItems    int    `json:"minItems"`
		UniqueItems bool   `json:"uniqueItems"`
		Items       struct {
			Type string `json:"type"`
		} `json:"items"`
	}
	if json.Unmarshal(schema.Properties[fieldName], &property) != nil {
		return false
	}
	if !slices.Contains(schema.Required, fieldName) {
		return false
	}
	return property.Type == "string" ||
		property.Type == "array" && property.Items.Type == "string" && property.MinItems >= 1 && property.UniqueItems
}

func strictSchema(document json.RawMessage) json.RawMessage {
	if len(document) == 0 {
		return nil
	}
	var value any
	if json.Unmarshal(document, &value) != nil {
		return nil
	}
	closeSchemaObjects(value)
	closedDocument, errorValue := json.Marshal(value)
	if errorValue != nil {
		return nil
	}
	return closedDocument
}

func closeSchemaObjects(value any) {
	schema, isSchema := value.(map[string]any)
	if !isSchema {
		if values, isArray := value.([]any); isArray {
			for _, item := range values {
				closeSchemaObjects(item)
			}
		}
		return
	}
	if schema["type"] == "object" {
		if _, exists := schema["additionalProperties"]; !exists {
			schema["additionalProperties"] = false
		}
	}
	for _, child := range schema {
		closeSchemaObjects(child)
	}
}

func isStrictSchema(document json.RawMessage) bool {
	var value any
	if len(document) == 0 || json.Unmarshal(document, &value) != nil {
		return false
	}
	schema, isSchema := value.(map[string]any)
	return isSchema && schema["type"] == "object" && schemaObjectsAreClosed(value)
}

func schemaObjectsAreClosed(value any) bool {
	schema, isSchema := value.(map[string]any)
	if !isSchema {
		if values, isArray := value.([]any); isArray {
			for _, item := range values {
				if !schemaObjectsAreClosed(item) {
					return false
				}
			}
		}
		return true
	}
	if schema["type"] == "object" {
		additionalProperties, exists := schema["additionalProperties"]
		if !exists || additionalProperties == true {
			return false
		}
		if additionalSchema, isSchema := additionalProperties.(map[string]any); isSchema && !schemaObjectsAreClosed(additionalSchema) {
			return false
		}
	}
	for _, child := range schema {
		if !schemaObjectsAreClosed(child) {
			return false
		}
	}
	return true
}
