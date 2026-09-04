package jsonschema

import (
	"encoding/json"
	"os"
	"sort"
	"strings"
	"testing"
)

// The walk names what is wrong; a keyword it does not know is a fault it cannot
// see, and the validator's own sentence only comes back when the walk finds
// nothing at all. So a constraint that arrives in the catalog while this does
// not handle it goes silent behind whatever else the same answer got wrong.
func TestEveryConstraintTheCatalogUsesIsExplained(t *testing.T) {
	document, errorValue := os.ReadFile("../generated/capability-tools.json")
	if errorValue != nil {
		t.Fatalf("the catalog this explains is not readable: %v", errorValue)
	}
	var catalog struct {
		Tools []struct {
			InputSchema    json.RawMessage `json:"inputSchema"`
			ResultContract struct {
				Schema json.RawMessage `json:"schema"`
			} `json:"resultContract"`
		} `json:"tools"`
	}
	if json.Unmarshal(document, &catalog) != nil {
		t.Fatal("the catalog is not readable as JSON")
	}

	unexplained := map[string]bool{}
	for _, tool := range catalog.Tools {
		collectUnexplainedKeywords(tool.InputSchema, unexplained)
		collectUnexplainedKeywords(tool.ResultContract.Schema, unexplained)
	}
	if len(unexplained) == 0 {
		return
	}
	names := make([]string, 0, len(unexplained))
	for name := range unexplained {
		names = append(names, name)
	}
	sort.Strings(names)
	t.Fatalf("the catalog constrains values with %s, which no sentence explains; add them to explainedKeywords and to boundMismatch", strings.Join(names, ", "))
}

// Annotations describe a value without constraining it, so a walk that ignores
// them names nothing the caller has to fix.
var annotationKeywords = map[string]bool{
	"$schema":     true,
	"default":     true,
	"description": true,
	"examples":    true,
	"format":      true,
	"title":       true,
}

func collectUnexplainedKeywords(schemaDocument json.RawMessage, found map[string]bool) {
	var node any
	if len(schemaDocument) == 0 || json.Unmarshal(schemaDocument, &node) != nil {
		return
	}
	walkKeywords(node, found)
}

func walkKeywords(node any, found map[string]bool) {
	object, isObject := node.(map[string]any)
	if !isObject {
		if list, isList := node.([]any); isList {
			for _, entry := range list {
				walkKeywords(entry, found)
			}
		}
		return
	}
	for name, value := range object {
		if !explainedKeywords[name] && !annotationKeywords[name] {
			found[name] = true
		}
		if name == "properties" {
			for _, field := range value.(map[string]any) {
				walkKeywords(field, found)
			}
			continue
		}
		walkKeywords(value, found)
	}
}

func TestBoundsAreNamedTheWayTheyAreFixed(t *testing.T) {
	contract := json.RawMessage(`{
		"type":"object",
		"properties":{
			"eventID":{"type":"string","minLength":1},
			"limit":{"type":"number","minimum":1,"maximum":200},
			"weeks":{"type":"array","minItems":1,"items":{"type":"string"}},
			"code":{"type":"string","pattern":"^[0-9]{4}$"}
		}
	}`)
	forEach := map[string]string{
		`{"eventID":""}`:   `input.eventID must be at least 1 character long, and it is ""`,
		`{"limit":500}`:    "input.limit must be at most 200, and it is 500",
		`{"weeks":[]}`:     "input.weeks must carry at least 1 entry, and it carries 0 entries",
		`{"code":"12-34"}`: `input.code must match ^[0-9]{4}$, and it is "12-34"`,
	}
	for given, expected := range forEach {
		errorValue := ValidateInput(contract, json.RawMessage(given))
		if errorValue == nil {
			t.Fatalf("%s: expected the bound to refuse it", given)
		}
		if !strings.Contains(errorValue.Error(), expected) {
			t.Fatalf("%s: expected %q, got %q", given, expected, errorValue.Error())
		}
	}
}
