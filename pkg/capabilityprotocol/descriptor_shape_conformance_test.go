package capabilityprotocol

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"testing"
)

const bluecollarDescriptorSchemaPath = "../../.dependency/blueclaw/.dependency/bluecollar/toolcontract/generated/tool-descriptor.schema.json"

var namesMeaningDifferentThings = map[string]string{
	"idempotency": "bluecollar writes a word beside idempotencyScope; the catalog writes an object, and blueclaw translates between them",
}

type declaredProperty struct {
	Type  string   `json:"type"`
	Types []string `json:"types"`
}

func bluecollarDescriptorProperties(t *testing.T) map[string]declaredProperty {
	t.Helper()
	document, errorValue := os.ReadFile(filepath.FromSlash(bluecollarDescriptorSchemaPath))
	if errorValue != nil {
		t.Skipf("bluecollar's generated descriptor schema is unavailable: %v", errorValue)
	}
	var schema struct {
		Properties map[string]json.RawMessage `json:"properties"`
	}
	if errorValue := json.Unmarshal(document, &schema); errorValue != nil {
		t.Fatal(errorValue)
	}
	properties := map[string]declaredProperty{}
	for name, propertyDocument := range schema.Properties {
		var property declaredProperty
		if json.Unmarshal(propertyDocument, &property) != nil {
			continue
		}
		properties[name] = property
	}
	if len(properties) == 0 {
		t.Fatalf("%s names no property", bluecollarDescriptorSchemaPath)
	}
	return properties
}

func jsonTypeOf(value any) string {
	switch value.(type) {
	case nil:
		return "null"
	case bool:
		return "boolean"
	case float64:
		return "number"
	case string:
		return "string"
	case []any:
		return "array"
	default:
		return "object"
	}
}

func (property declaredProperty) accepts(jsonType string) bool {
	if property.Type == "" && len(property.Types) == 0 {
		return true
	}
	if property.Type == jsonType {
		return true
	}
	for _, declared := range property.Types {
		if declared == jsonType {
			return true
		}
	}
	return false
}

func catalogFieldTypes(t *testing.T) map[string]map[string]bool {
	t.Helper()
	types := map[string]map[string]bool{}
	for _, descriptor := range GeneratedToolDescriptorSet() {
		document, errorValue := json.Marshal(descriptor)
		if errorValue != nil {
			t.Fatal(errorValue)
		}
		var fields map[string]any
		if errorValue := json.Unmarshal(document, &fields); errorValue != nil {
			t.Fatal(errorValue)
		}
		for name, value := range fields {
			if _, isSeen := types[name]; !isSeen {
				types[name] = map[string]bool{}
			}
			types[name][jsonTypeOf(value)] = true
		}
	}
	return types
}

func TestSharedDescriptorFieldsMeanTheSameKindOfThing(t *testing.T) {
	declared := bluecollarDescriptorProperties(t)
	emitted := catalogFieldTypes(t)

	shared := []string{}
	for name := range emitted {
		if _, isShared := declared[name]; isShared {
			shared = append(shared, name)
		}
	}
	sort.Strings(shared)

	if len(shared) == 0 {
		t.Fatal("the catalog and bluecollar share no field name, so this test reads the wrong place")
	}

	for _, name := range shared {
		if reason, isKnown := namesMeaningDifferentThings[name]; isKnown {
			t.Logf("%s is not compared: %s", name, reason)
			continue
		}
		for jsonType := range emitted[name] {
			if !declared[name].accepts(jsonType) {
				t.Errorf("the catalog writes %s as %s, which bluecollar declares as %v",
					name, jsonType, append(declared[name].Types, declared[name].Type))
			}
		}
	}
}

func TestTheNamesMeaningDifferentThingsAreStillShared(t *testing.T) {
	declared := bluecollarDescriptorProperties(t)
	catalogNames := catalogFieldTypes(t)
	for name, reason := range namesMeaningDifferentThings {
		if _, isDeclared := declared[name]; !isDeclared {
			t.Errorf("%s is excused as %q and bluecollar no longer names it", name, reason)
		}
		if _, isWritten := catalogNames[name]; !isWritten {
			t.Errorf("%s is excused as %q and the catalog no longer writes it", name, reason)
		}
	}
}
