package capabilityprotocol

import (
	"os"
	"path/filepath"
	"regexp"
	"testing"
)

const bluecollarProviderPath = "../../.dependency/blueclaw/.dependency/blueprotocol/toolcontract/provider.go"

// bluecollar owns the grammar, so it is read from bluecollar rather than retyped.
func TestEveryCatalogNameSatisfiesTheCanonicalGrammar(t *testing.T) {
	grammar := bluecollarCanonicalToolNamePattern(t)
	for _, descriptor := range GeneratedToolDescriptorSet() {
		for fieldName, value := range map[string]string{
			"name":          descriptor.Name,
			"canonicalName": descriptor.CanonicalName,
			"modelName":     descriptor.ModelName,
		} {
			if !grammar.MatchString(value) {
				t.Errorf("%s %s is %q, which %s refuses", descriptor.Name, fieldName, value, grammar)
			}
		}
	}
}

func bluecollarCanonicalToolNamePattern(t *testing.T) *regexp.Regexp {
	t.Helper()
	source, errorValue := os.ReadFile(filepath.FromSlash(bluecollarProviderPath))
	if errorValue != nil {
		t.Fatalf("blueprotocol source unavailable: %v", errorValue)
	}
	declaration := regexp.MustCompile("canonicalToolNamePattern = regexp.MustCompile\\(`([^`]+)`\\)").FindSubmatch(source)
	if declaration == nil {
		t.Fatalf("canonicalToolNamePattern not found in %s", bluecollarProviderPath)
	}
	return regexp.MustCompile(string(declaration[1]))
}
