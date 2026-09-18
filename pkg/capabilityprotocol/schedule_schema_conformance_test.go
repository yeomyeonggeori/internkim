package capabilityprotocol

import (
	"encoding/json"
	"os"
	"regexp"
	"testing"
)

func TestScheduleListStandaloneInputSchemaMatchesCanonicalCatalog(t *testing.T) {
	descriptor := MustGeneratedToolDescriptors("schedule_list")[0]
	source, errorValue := os.ReadFile("../../.dependency/blueclaw/internal/agentruntime/schedule_tool_contracts.go")
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	assertScheduleSchemaMatches(t, "input", descriptor.InputSchema, source, "scheduleListInputSchema")
	assertScheduleSchemaMatches(t, "output", descriptor.OutputSchema, source, "scheduleListOutputSchema")
}

func assertScheduleSchemaMatches(t *testing.T, name string, canonicalDocument []byte, source []byte, variableName string) {
	t.Helper()
	canonical := decodeSchemaObject(t, canonicalDocument)
	delete(canonical, "$schema")
	match := regexp.MustCompile("(?s)var " + variableName + " = json.RawMessage\\(`(.*?)`\\)").FindSubmatch(source)
	if len(match) != 2 {
		t.Fatalf("schedule_list standalone %s schema was not found", name)
	}
	standalone := decodeSchemaObject(t, match[1])
	encodedCanonical, _ := json.Marshal(canonical)
	encodedStandalone, _ := json.Marshal(standalone)
	if string(encodedCanonical) != string(encodedStandalone) {
		t.Fatalf("standalone schedule_list %s schema drifted\ncanonical: %s\nstandalone: %s", name, encodedCanonical, encodedStandalone)
	}
}

func decodeSchemaObject(t *testing.T, document []byte) map[string]any {
	t.Helper()
	var schema map[string]any
	if errorValue := json.Unmarshal(document, &schema); errorValue != nil {
		t.Fatal(errorValue)
	}
	return schema
}
