package cli

import "testing"

func TestMarshalEnvironmentStringArrayNormalizesNil(t *testing.T) {
	if document := marshalEnvironmentStringArray(nil); document != "[]" {
		t.Fatalf("expected empty JSON array, got %s", document)
	}
}

func TestMarshalEnvironmentStringArrayPreservesValues(t *testing.T) {
	if document := marshalEnvironmentStringArray([]string{"보고서.pdf", "확인"}); document != `["보고서.pdf","확인"]` {
		t.Fatalf("unexpected JSON array %s", document)
	}
}
