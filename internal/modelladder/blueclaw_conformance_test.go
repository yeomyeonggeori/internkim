package modelladder

import (
	"encoding/json"
	"maps"
	"os"
	"slices"
	"testing"
)

// blueclaw is a separate Go module this one does not depend on, so Rung and
// Document cannot be blueclaw's own config.ModelEndpointConfiguration and have
// to be written twice. What holds the two spellings together is the wire:
// blueclaw's committed example is the shape its loader is tested against
// (internal/llm/example_configuration_test.go), and this decodes and re-encodes
// that same file. A field or a tier renamed on either side fails here.
const blueclawExamplePath = "../../.dependency/blueclaw/config/runtime.standalone.example.json"

func blueclawExampleLadder(t *testing.T) map[string]json.RawMessage {
	t.Helper()
	document, errorValue := os.ReadFile(blueclawExamplePath)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	var example struct {
		LanguageModel map[string]json.RawMessage `json:"languageModel"`
	}
	if errorValue := json.Unmarshal(document, &example); errorValue != nil {
		t.Fatal(errorValue)
	}
	if len(example.LanguageModel) == 0 {
		t.Fatalf("%s names no languageModel block", blueclawExamplePath)
	}
	return example.LanguageModel
}

func TestOurLadderDecodesTheDocumentBlueclawIsTestedAgainst(t *testing.T) {
	block, errorValue := json.Marshal(blueclawExampleLadder(t))
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	var decoded Document
	if errorValue := json.Unmarshal(block, &decoded); errorValue != nil {
		t.Fatal(errorValue)
	}

	if !slices.Equal(slices.Sorted(maps.Keys(decoded.Tiers)), slices.Sorted(slices.Values(Tiers))) {
		t.Fatalf("blueclaw's example names %v and this names %v", slices.Sorted(maps.Keys(decoded.Tiers)), Tiers)
	}
	for _, tier := range Tiers {
		for rungIndex, rung := range decoded.Tiers[tier] {
			if rung.Endpoint == "" || rung.Model == "" {
				t.Fatalf("%s rung %d decoded empty, so a field is spelled differently on the two sides: %+v", tier, rungIndex, rung)
			}
		}
	}
	if decoded.Embedding.Endpoint == "" || decoded.Embedding.Model == "" {
		t.Fatalf("the embedding block decoded empty: %+v", decoded.Embedding)
	}
}

func TestWhatWeWriteCarriesTheFieldsBlueclawReads(t *testing.T) {
	written, errorValue := json.Marshal(LanguageModelDocument("http://127.0.0.1:11434/v1", "/run/example-key"))
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	var ours map[string]json.RawMessage
	if errorValue := json.Unmarshal(written, &ours); errorValue != nil {
		t.Fatal(errorValue)
	}

	theirs := blueclawExampleLadder(t)
	for _, field := range slices.Sorted(maps.Keys(theirs)) {
		if _, isWritten := ours[field]; !isWritten {
			t.Fatalf("blueclaw's example carries languageModel.%s and nothing here writes it", field)
		}
	}

	var theirRungs map[string][]map[string]json.RawMessage
	if errorValue := json.Unmarshal(theirs["tiers"], &theirRungs); errorValue != nil {
		t.Fatal(errorValue)
	}
	var ourRungs map[string][]map[string]json.RawMessage
	if errorValue := json.Unmarshal(ours["tiers"], &ourRungs); errorValue != nil {
		t.Fatal(errorValue)
	}
	for _, field := range slices.Sorted(maps.Keys(theirRungs["low"][0])) {
		if keySourceFields[field] && writesAKeySource(ourRungs["low"][0]) {
			continue
		}
		if _, isWritten := ourRungs["low"][0][field]; !isWritten {
			t.Fatalf("blueclaw's example spells a rung's field %q and nothing here writes it", field)
		}
	}
}

var keySourceFields = map[string]bool{"apiKeyPath": true, "apiKeyEnvironment": true}

func writesAKeySource(rung map[string]json.RawMessage) bool {
	for field := range keySourceFields {
		if _, isWritten := rung[field]; isWritten {
			return true
		}
	}
	return false
}
