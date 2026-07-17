package capabilityprotocol

import (
	"encoding/json"
	"fmt"
	"strings"
)

const (
	ModelVisibilityVisible = "visible"
	ModelVisibilityHidden  = "hidden"
	AvailabilityOK         = "ok"
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

func canonicalizeSchemas(descriptors []Descriptor) {
	for index := range descriptors {
		descriptors[index].InputSchema = strictSchema(descriptors[index].InputSchema)
		descriptors[index].OutputSchema = strictSchema(descriptors[index].OutputSchema)
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
	if strings.TrimSpace(descriptor.EstimatedLatency) == "" {
		return fmt.Errorf("estimatedLatency is required")
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
	if !descriptor.OutputSchemaStrict || !isStrictSchema(descriptor.OutputSchema) {
		return fmt.Errorf("outputSchema must be a strict object schema")
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
	if descriptor.Availability.State != AvailabilityOK {
		return fmt.Errorf("availability.state must be %q", AvailabilityOK)
	}
	if descriptor.Idempotency.Scope == "" {
		return fmt.Errorf("idempotency.scope is required")
	}
	return nil
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
