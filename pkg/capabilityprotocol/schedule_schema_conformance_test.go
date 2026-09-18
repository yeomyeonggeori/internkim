package capabilityprotocol

import (
	"encoding/json"
	"os"
	"regexp"
	"sort"
	"testing"
)

const signedScheduleContractPath = "../../.dependency/blueclaw/internal/adminapi/schedule_contracts.go"

type signedScheduleContract struct {
	inputSchemaName  string
	outputSchemaName string
}

var signedScheduleContracts = map[string]signedScheduleContract{
	"schedule_create": {"scheduleToolCreateInputSchema", "scheduleToolMutationOutputSchema"},
	"schedule_update": {"scheduleToolUpdateInputSchema", "scheduleToolMutationOutputSchema"},
	"schedule_cancel": {"scheduleToolCancelInputSchema", "scheduleToolCancelOutputSchema"},
}

// The conversation a schedule answers in is a fact the runtime holds, so
// capabilityd fills these on the way through and the model never sees them.
var scheduleDeliveryBindingFields = []string{"platform", "conversationID", "replyTargetID"}

// The catalog descriptor and the signed schedule contract are written in two
// repositories, and a field only one of them knows is the failure this catches:
// a field the model may send that the contract refuses, or an answer the
// contract may give that the strict result check refuses.
func TestScheduleWriteDescriptorsAgreeWithTheSignedScheduleContract(t *testing.T) {
	source, errorValue := os.ReadFile(signedScheduleContractPath)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	for toolName, contract := range signedScheduleContracts {
		descriptor := MustGeneratedToolDescriptors(toolName)[0]
		contracted := scheduleContractSchema(t, source, contract.inputSchemaName)
		assertScheduleSchemasAgree(t, toolName+" input", decodeSchemaObject(t, descriptor.InputSchema), withoutScheduleFields(contracted, scheduleDeliveryBindingFields))
		assertScheduleSchemasAgree(t, toolName+" output", decodeSchemaObject(t, descriptor.OutputSchema), scheduleContractSchema(t, source, contract.outputSchemaName))
	}
}

func scheduleContractSchema(t *testing.T, source []byte, variableName string) map[string]any {
	t.Helper()
	match := regexp.MustCompile("(?s)var " + variableName + " = json.RawMessage\\(`(.*?)`\\)").FindSubmatch(source)
	if len(match) != 2 {
		t.Fatalf("the signed schedule contract declares no %s", variableName)
	}
	return decodeSchemaObject(t, match[1])
}

func assertScheduleSchemasAgree(t *testing.T, what string, catalog map[string]any, contracted map[string]any) {
	t.Helper()
	catalogProperties := schemaProperties(t, catalog)
	contractedProperties := schemaProperties(t, contracted)
	if !sameNames(schemaPropertyNames(catalogProperties), schemaPropertyNames(contractedProperties)) {
		t.Fatalf("%s names %v where the signed contract names %v",
			what, schemaPropertyNames(catalogProperties), schemaPropertyNames(contractedProperties))
	}
	if !sameNames(schemaRequiredNames(t, catalog), schemaRequiredNames(t, contracted)) {
		t.Fatalf("%s requires %v where the signed contract requires %v",
			what, schemaRequiredNames(t, catalog), schemaRequiredNames(t, contracted))
	}
	if catalog["additionalProperties"] != false {
		t.Fatalf("%s accepts properties the signed contract refuses", what)
	}
	for _, propertyName := range schemaPropertyNames(catalogProperties) {
		catalogShape := schemaValueShape(t, catalogProperties[propertyName])
		contractedShape := schemaValueShape(t, contractedProperties[propertyName])
		if catalogShape != contractedShape {
			t.Fatalf("%s declares %s as %s where the signed contract declares it as %s",
				what, propertyName, catalogShape, contractedShape)
		}
	}
}

func withoutScheduleFields(schema map[string]any, fieldNames []string) map[string]any {
	trimmed := map[string]any{}
	for key, value := range schema {
		trimmed[key] = value
	}
	properties := map[string]any{}
	for name, property := range schema["properties"].(map[string]any) {
		properties[name] = property
	}
	for _, fieldName := range fieldNames {
		delete(properties, fieldName)
	}
	trimmed["properties"] = properties
	trimmed["required"] = keepingNamesOutside(schema["required"], fieldNames)
	return trimmed
}

func keepingNamesOutside(required any, fieldNames []string) []any {
	kept := []any{}
	names, isList := required.([]any)
	if !isList {
		return kept
	}
	for _, name := range names {
		if slicesContain(fieldNames, name) {
			continue
		}
		kept = append(kept, name)
	}
	return kept
}

func slicesContain(names []string, value any) bool {
	for _, name := range names {
		if name == value {
			return true
		}
	}
	return false
}

func schemaProperties(t *testing.T, schema map[string]any) map[string]any {
	t.Helper()
	properties, isObject := schema["properties"].(map[string]any)
	if !isObject {
		t.Fatal("a schedule schema declares no properties")
	}
	return properties
}

func schemaPropertyNames(properties map[string]any) []string {
	names := make([]string, 0, len(properties))
	for name := range properties {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func schemaRequiredNames(t *testing.T, schema map[string]any) []string {
	t.Helper()
	names := []string{}
	required, isPresent := schema["required"]
	if !isPresent {
		return names
	}
	values, isList := required.([]any)
	if !isList {
		t.Fatal("a schedule schema declares a required list that is not a list")
	}
	for _, value := range values {
		name, isString := value.(string)
		if !isString {
			t.Fatal("a schedule schema requires something that is not a property name")
		}
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// The type and the closed set of values a property accepts. Everything else a
// zod schema stamps on, such as the JavaScript safe-integer ceiling, narrows
// what the contract already accepts and cannot make a call fail upstream.
func schemaValueShape(t *testing.T, property any) string {
	t.Helper()
	declaration, isObject := property.(map[string]any)
	if !isObject {
		t.Fatal("a schedule schema declares a property that is not an object")
	}
	shape := map[string]any{"type": declaration["type"]}
	if enumeration, hasEnumeration := declaration["enum"]; hasEnumeration {
		shape["enum"] = enumeration
	}
	if items, hasItems := declaration["items"].(map[string]any); hasItems {
		shape["items"] = map[string]any{"type": items["type"]}
	}
	encoded, errorValue := json.Marshal(shape)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	return string(encoded)
}

func sameNames(left []string, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}

func decodeSchemaObject(t *testing.T, document []byte) map[string]any {
	t.Helper()
	var schema map[string]any
	if errorValue := json.Unmarshal(document, &schema); errorValue != nil {
		t.Fatal(errorValue)
	}
	return schema
}
