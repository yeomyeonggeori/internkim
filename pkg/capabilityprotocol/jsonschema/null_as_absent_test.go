package jsonschema

import (
	"encoding/json"
	"os"
	"reflect"
	"testing"
)

const nullAsAbsentCasesPath = "../../../.dependency/blueclaw/.dependency/bluecollar/toolcontract/testdata/null-as-absent.json"

func TestNullIsReadAsAbsentTheWayBluecollarReadsIt(t *testing.T) {
	content, errorValue := os.ReadFile(nullAsAbsentCasesPath)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	var cases struct {
		Schemas []struct {
			Schema         any `json:"schema"`
			AbsentWhenNull any `json:"absentWhenNull"`
		} `json:"schemas"`
		Documents []struct {
			Document       any `json:"document"`
			AbsentWhenNull any `json:"absentWhenNull"`
		} `json:"documents"`
	}
	if errorValue := json.Unmarshal(content, &cases); errorValue != nil {
		t.Fatal(errorValue)
	}
	for _, schemaCase := range cases.Schemas {
		if normalized := schemaWithNullAsAbsent(schemaCase.Schema); !reflect.DeepEqual(normalized, schemaCase.AbsentWhenNull) {
			t.Fatalf("schema %v\nread as %v\nwant %v", schemaCase.Schema, normalized, schemaCase.AbsentWhenNull)
		}
	}
	for _, documentCase := range cases.Documents {
		if normalized := DocumentWithNullAsAbsent(documentCase.Document); !reflect.DeepEqual(normalized, documentCase.AbsentWhenNull) {
			t.Fatalf("document %v\nread as %v\nwant %v", documentCase.Document, normalized, documentCase.AbsentWhenNull)
		}
	}
}
