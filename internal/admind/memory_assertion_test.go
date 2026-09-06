package admind

import (
	"encoding/json"
	"os"
	"testing"
)

func TestMemorySignerMatchesCanonicalVerifierFixture(t *testing.T) {
	document, errorValue := os.ReadFile("../../.dependency/blueclaw/pkg/memoryassertion/testdata/assertion.json")
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	var fixture struct {
		Secret         string `json:"secret"`
		Path           string `json:"path"`
		ReaderPersonID string `json:"readerPersonID"`
		ExpiresAt      int64  `json:"expiresAt"`
		Body           string `json:"body"`
		Header         string `json:"header"`
	}
	if errorValue := json.Unmarshal(document, &fixture); errorValue != nil {
		t.Fatal(errorValue)
	}
	header, errorValue := signMemoryAssertion([]byte(fixture.Body), fixture.ReaderPersonID, fixture.ExpiresAt, fixture.Path, fixture.Secret)
	if errorValue != nil || header != fixture.Header {
		t.Fatalf("signer differs from canonical verifier fixture: %v", errorValue)
	}
}
