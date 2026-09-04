package jsonschema

import (
	"bytes"
	"encoding/json"
	"errors"
	"sort"
	"strings"

	googlejsonschema "github.com/google/jsonschema-go/jsonschema"
)

// A result contract is read on the device and written on the plane, and the two
// are deployed on their own schedules, so a field the plane learned to answer
// with reaches a device whose contract has never heard of it. Rejecting that
// costs a fleet-wide outage to buy nothing: the reader takes fields by name.
// Unknown fields are carried and named; everything the contract actually
// promises is still enforced.
type ResultCheck struct {
	UnknownFields []string
}

func ValidateInput(schemaDocument json.RawMessage, inputDocument json.RawMessage) error {
	if isBlankDocument(schemaDocument) {
		return nil
	}
	document, errorValue := decodeDocument(inputDocument, "input")
	if errorValue != nil {
		return errorValue
	}
	schema, errorValue := resolveSchema(schemaDocument, "input")
	if errorValue != nil {
		return errorValue
	}
	failure := schema.Validate(document)
	if failure == nil {
		return nil
	}
	return errors.New("tool input does not match its descriptor schema: " +
		explained(schemaDocument, document, "input", failure) + acceptedParameterSentence(schemaDocument))
}

func ValidateResult(schemaDocument json.RawMessage, resultDocument json.RawMessage) (ResultCheck, error) {
	if isBlankDocument(schemaDocument) {
		return ResultCheck{}, nil
	}
	document, errorValue := decodeDocument(resultDocument, "result")
	if errorValue != nil {
		return ResultCheck{}, errorValue
	}
	contract, errorValue := resolveSchema(schemaDocument, "result")
	if errorValue != nil {
		return ResultCheck{}, errorValue
	}
	if contract.Validate(document) == nil {
		return ResultCheck{}, nil
	}
	promised := withoutClosedObjects(schemaDocument)
	promises, errorValue := resolveSchema(promised, "result")
	if errorValue != nil {
		return ResultCheck{}, errorValue
	}
	if failure := promises.Validate(document); failure != nil {
		return ResultCheck{}, errors.New("tool result does not match its descriptor schema: " +
			explained(promised, document, "result", failure) +
			". This is the tool's own answer rather than the call, so the same request with different arguments will not change it")
	}
	return ResultCheck{UnknownFields: unknownFieldPaths(schemaDocument, document)}, nil
}

func decodeDocument(document json.RawMessage, subject string) (any, error) {
	normalized := document
	if isBlankDocument(normalized) {
		normalized = json.RawMessage(`{}`)
	}
	var value any
	if json.Unmarshal(normalized, &value) != nil {
		return nil, errors.New("tool " + subject + " is not valid JSON")
	}
	return value, nil
}

func resolveSchema(schemaDocument json.RawMessage, subject string) (*googlejsonschema.Resolved, error) {
	var schema googlejsonschema.Schema
	if json.Unmarshal(schemaDocument, &schema) != nil {
		return nil, errors.New("tool " + subject + " schema is invalid")
	}
	resolved, errorValue := schema.Resolve(nil)
	if errorValue != nil {
		return nil, errors.New("tool " + subject + " schema cannot be resolved")
	}
	return resolved, nil
}

func explained(schemaDocument json.RawMessage, document any, subject string, failure error) string {
	sentences := mismatchSentences(schemaDocument, document, subject)
	if len(sentences) == 0 {
		return compactWhitespace(failure.Error())
	}
	return strings.Join(sentences, "; ")
}

// A model that invented a parameter cannot see the descriptor, so a rejection
// that names neither what was wrong nor what exists leaves it guessing, and a
// guess costs a whole round trip.
func acceptedParameterSentence(schemaDocument json.RawMessage) string {
	names := propertyNames(objectDocument(schemaDocument))
	if len(names) == 0 {
		return ""
	}
	return ". This tool takes: " + strings.Join(names, ", ")
}

func propertyNames(schema map[string]any) []string {
	properties, hasProperties := schema["properties"].(map[string]any)
	if !hasProperties {
		return nil
	}
	names := make([]string, 0, len(properties))
	for name := range properties {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func withoutClosedObjects(schemaDocument json.RawMessage) json.RawMessage {
	schema := objectDocument(schemaDocument)
	if schema == nil {
		return schemaDocument
	}
	opened, errorValue := json.Marshal(openedNode(schema))
	if errorValue != nil {
		return schemaDocument
	}
	return opened
}

func openedNode(node map[string]any) map[string]any {
	opened := map[string]any{}
	for key, value := range node {
		if key == "additionalProperties" && value == false {
			continue
		}
		opened[key] = openedValue(value)
	}
	return opened
}

func openedValue(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		return openedNode(typed)
	case []any:
		opened := make([]any, 0, len(typed))
		for _, entry := range typed {
			opened = append(opened, openedValue(entry))
		}
		return opened
	default:
		return value
	}
}

func unknownFieldPaths(schemaDocument json.RawMessage, document any) []string {
	paths := map[string]bool{}
	collectUnknownFields(objectDocument(schemaDocument), document, "", paths)
	names := make([]string, 0, len(paths))
	for path := range paths {
		names = append(names, path)
	}
	sort.Strings(names)
	return names
}

func collectUnknownFields(schema map[string]any, document any, path string, found map[string]bool) {
	if schema == nil {
		return
	}
	switch value := document.(type) {
	case map[string]any:
		properties, _ := schema["properties"].(map[string]any)
		isClosed := schema["additionalProperties"] == false
		for name, field := range value {
			fieldSchema, isDeclared := properties[name].(map[string]any)
			if !isDeclared {
				if isClosed {
					found[joinPath(path, name)] = true
				}
				continue
			}
			collectUnknownFields(fieldSchema, field, joinPath(path, name), found)
		}
	case []any:
		items, _ := schema["items"].(map[string]any)
		for _, entry := range value {
			collectUnknownFields(items, entry, path+"[]", found)
		}
	}
}

func joinPath(path string, name string) string {
	if path == "" {
		return name
	}
	return path + "." + name
}

func objectDocument(document json.RawMessage) map[string]any {
	var value map[string]any
	if json.Unmarshal(document, &value) != nil {
		return nil
	}
	return value
}

func isBlankDocument(document json.RawMessage) bool {
	return len(bytes.TrimSpace(document)) == 0
}

func compactWhitespace(value string) string {
	return strings.Join(strings.Fields(value), " ")
}
