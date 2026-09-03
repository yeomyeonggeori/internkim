package capabilityprotocol

import (
	"encoding/json"
	"reflect"
	"sort"
	"strings"
	"testing"
)

func TestApprovalTargetMatchesTheGeneratedSchema(t *testing.T) {
	schemaProperties := generatedSchemaPropertyNames(t, "approval-target")
	if len(schemaProperties) == 0 {
		t.Fatal("no properties found in the generated approval-target schema")
	}

	mirroredProperties := structJSONFieldNames(reflect.TypeOf(ApprovalTarget{}))
	if !reflect.DeepEqual(schemaProperties, mirroredProperties) {
		t.Fatalf("ApprovalTarget drifted from the generated schema: generated=%v mirrored=%v", schemaProperties, mirroredProperties)
	}
}

func TestApprovalTargetMatchesBluecollarAgentContract(t *testing.T) {
	canonicalTags := bluecollarStructJSONTags(t, bluecollarApprovalTargetPath, "ApprovalTarget")
	if len(canonicalTags) == 0 {
		t.Fatalf("no ApprovalTarget fields found in %s", bluecollarApprovalTargetPath)
	}

	mirroredTags := structJSONTags(reflect.TypeOf(ApprovalTarget{}))
	if !reflect.DeepEqual(canonicalTags, mirroredTags) {
		t.Fatalf("ApprovalTarget drifted from bluecollar: canonical=%v mirrored=%v", canonicalTags, mirroredTags)
	}
}

func generatedSchemaPropertyNames(t *testing.T, schemaName string) []string {
	t.Helper()
	document, errorValue := generatedCatalogFiles.ReadFile("generated/json-schema/" + schemaName + ".schema.json")
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	schema := struct {
		Properties map[string]struct{} `json:"properties"`
	}{}
	if errorValue := json.Unmarshal(document, &schema); errorValue != nil {
		t.Fatal(errorValue)
	}
	propertyNames := make([]string, 0, len(schema.Properties))
	for propertyName := range schema.Properties {
		propertyNames = append(propertyNames, propertyName)
	}
	sort.Strings(propertyNames)
	return propertyNames
}

func structJSONFieldNames(structType reflect.Type) []string {
	fieldNames := make([]string, 0, structType.NumField())
	for index := 0; index < structType.NumField(); index++ {
		fieldNames = append(fieldNames, strings.Split(structType.Field(index).Tag.Get("json"), ",")[0])
	}
	sort.Strings(fieldNames)
	return fieldNames
}
