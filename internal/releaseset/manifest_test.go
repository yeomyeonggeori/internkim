package releaseset

import (
	"encoding/json"
	"testing"

	"gitlab.com/eastriver/internkim/pkg/capabilityprotocol"
)

func TestManifestPublishedWithASignatureStillReads(t *testing.T) {
	published := NewManifest("release-1", "stable", map[string]Component{
		"blueclawPayload": {
			Name:         "blueclawPayload",
			Revision:     "revision-1",
			SHA256:       "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
			Size:         10,
			BlobPath:     "blobs/sha256/0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
			RestartGroup: "blueclaw",
			HealthCheck:  "blueclaw",
		},
	})
	document, errorValue := json.Marshal(published)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	var fields map[string]any
	if errorValue := json.Unmarshal(document, &fields); errorValue != nil {
		t.Fatal(errorValue)
	}
	fields["signature"] = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	document, errorValue = json.Marshal(fields)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	var read Manifest
	if errorValue := json.Unmarshal(document, &read); errorValue != nil {
		t.Fatal(errorValue)
	}
	if errorValue := read.Validate(); errorValue != nil {
		t.Fatalf("a manifest an earlier release published with a signature must still read: %v", errorValue)
	}
}

func TestManifestValidationRequiresProtocolIdentity(t *testing.T) {
	manifest := NewManifest("release-1", "stable", map[string]Component{
		"admind": {
			Name:     "admind",
			Revision: "revision-1",
			SHA256:   "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
			Size:     10,
			BlobPath: "blobs/sha256/0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
		},
	})
	manifest.ProtocolVersion = ""
	if errorValue := manifest.Validate(); errorValue == nil {
		t.Fatal("expected missing protocol version to fail")
	}
	manifest.ProtocolVersion = capabilityprotocol.GeneratedProtocolVersion()
	manifest.AggregateProtocolHash = "ABCDEF6789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	if errorValue := manifest.Validate(); errorValue == nil {
		t.Fatal("expected non-lowercase aggregate protocol hash to fail")
	}
}
