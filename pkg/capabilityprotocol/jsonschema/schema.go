package jsonschema

import (
	"bytes"
	"encoding/json"
	"errors"

	googlejsonschema "github.com/google/jsonschema-go/jsonschema"
)

type Schema struct {
	document map[string]any
}

type FieldDefinition struct {
	Name       string
	Schema     Schema
	IsRequired bool
}

func Object(fields ...FieldDefinition) Schema {
	properties := map[string]any{}
	required := []string{}
	for _, field := range fields {
		if field.Name == "" {
			continue
		}
		properties[field.Name] = field.Schema.document
		if field.IsRequired {
			required = append(required, field.Name)
		}
	}
	document := map[string]any{
		"type":       "object",
		"properties": properties,
	}
	if len(required) > 0 {
		document["required"] = required
	}
	return Schema{document: document}
}

func Field(name string, schema Schema) FieldDefinition {
	return FieldDefinition{Name: name, Schema: schema}
}

func Required(name string, schema Schema) FieldDefinition {
	return FieldDefinition{Name: name, Schema: schema, IsRequired: true}
}

func String() Schema {
	return typedSchema("string")
}

func Boolean() Schema {
	return typedSchema("boolean")
}

func Integer() Schema {
	return typedSchema("integer")
}

func Number() Schema {
	return typedSchema("number")
}

func Array(items Schema) Schema {
	return Schema{document: map[string]any{
		"type":  "array",
		"items": items.document,
	}}
}

func StringEnum(values ...string) Schema {
	enumValues := make([]string, 0, len(values))
	for _, value := range values {
		if value != "" {
			enumValues = append(enumValues, value)
		}
	}
	return Schema{document: map[string]any{
		"type": "string",
		"enum": enumValues,
	}}
}

func Raw(document json.RawMessage) Schema {
	var value map[string]any
	if json.Unmarshal(document, &value) != nil {
		return Object()
	}
	return Schema{document: value}
}

func (schema Schema) WithDescription(description string) Schema {
	if description == "" {
		return schema
	}
	document := cloneDocument(schema.document)
	document["description"] = description
	return Schema{document: document}
}

func (schema Schema) RawMessage() json.RawMessage {
	content, errorValue := json.Marshal(schema.document)
	if errorValue != nil {
		return json.RawMessage(`{"type":"object","properties":{}}`)
	}
	return json.RawMessage(content)
}

func Validate(schemaDocument json.RawMessage, inputDocument json.RawMessage) error {
	normalizedInput := inputDocument
	if len(bytes.TrimSpace(normalizedInput)) == 0 {
		normalizedInput = json.RawMessage(`{}`)
	}
	var input any
	if errorValue := json.Unmarshal(normalizedInput, &input); errorValue != nil {
		return errors.New("tool input is not valid JSON")
	}
	var schema googlejsonschema.Schema
	if errorValue := json.Unmarshal(schemaDocument, &schema); errorValue != nil {
		return errors.New("tool input schema is invalid")
	}
	resolvedSchema, errorValue := schema.Resolve(nil)
	if errorValue != nil {
		return errors.New("tool input schema cannot be resolved")
	}
	if errorValue := resolvedSchema.Validate(input); errorValue != nil {
		return errors.New("tool input does not match its descriptor schema")
	}
	return nil
}

func typedSchema(schemaType string) Schema {
	return Schema{document: map[string]any{"type": schemaType}}
}

func cloneDocument(document map[string]any) map[string]any {
	clone := map[string]any{}
	for key, value := range document {
		clone[key] = value
	}
	return clone
}
