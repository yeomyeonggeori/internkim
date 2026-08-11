package capabilityprotocol

import (
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"testing"
)

const bluecollarToolContractPath = "../../.dependency/blueclaw/.dependency/bluecollar/toolcontract/registry.go"

func TestRecoveryHintMatchesBluecollarToolContract(t *testing.T) {
	canonicalTags := bluecollarStructJSONTags(t, "RecoveryHint")
	if len(canonicalTags) == 0 {
		t.Fatalf("no RecoveryHint fields found in %s", bluecollarToolContractPath)
	}

	mirroredTags := structJSONTags(reflect.TypeOf(RecoveryHint{}))
	if !reflect.DeepEqual(canonicalTags, mirroredTags) {
		t.Fatalf("RecoveryHint drifted from bluecollar: canonical=%v mirrored=%v", canonicalTags, mirroredTags)
	}
}

func bluecollarStructJSONTags(t *testing.T, structName string) []string {
	t.Helper()
	source, errorValue := os.ReadFile(filepath.FromSlash(bluecollarToolContractPath))
	if errorValue != nil {
		t.Skipf("bluecollar source unavailable: %v", errorValue)
	}
	declaration := regexp.MustCompile(`(?s)type ` + structName + ` struct \{(.*?)\n\}`).FindSubmatch(source)
	if declaration == nil {
		t.Fatalf("type %s not found in %s", structName, bluecollarToolContractPath)
	}
	tags := []string{}
	for _, match := range regexp.MustCompile("`json:\"([^\"]+)\"`").FindAllSubmatch(declaration[1], -1) {
		tags = append(tags, string(match[1]))
	}
	return tags
}

func structJSONTags(structType reflect.Type) []string {
	tags := []string{}
	for index := 0; index < structType.NumField(); index++ {
		tags = append(tags, structType.Field(index).Tag.Get("json"))
	}
	return tags
}
