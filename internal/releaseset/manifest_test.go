package releaseset

import (
	"testing"

	"gitlab.com/eastriver/internkim/pkg/capabilityprotocol"
)

func TestManifestSignatureRoundTrip(t *testing.T) {
	manifest := NewManifest("release-1", "stable", map[string]Component{
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

	signedManifest, errorValue := manifest.Sign("secret")
	if errorValue != nil {
		t.Fatalf("expected signed manifest: %v", errorValue)
	}
	if signedManifest.Signature == "" {
		t.Fatal("expected signature")
	}
	if signedManifest.ProtocolIdentity != capabilityprotocol.GeneratedProtocolIdentity() {
		t.Fatalf("unexpected protocol identity: %+v", signedManifest.ProtocolIdentity)
	}
	if errorValue := signedManifest.VerifySignature("secret"); errorValue != nil {
		t.Fatalf("expected valid signature: %v", errorValue)
	}
}

func TestManifestSignatureRejectsMutation(t *testing.T) {
	manifest := NewManifest("release-1", "stable", map[string]Component{
		"admind": {
			Name:         "admind",
			Revision:     "revision-1",
			SHA256:       "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
			Size:         10,
			BlobPath:     "blobs/sha256/0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
			RestartGroup: "admind",
			HealthCheck:  "admind",
		},
	})
	signedManifest, errorValue := manifest.Sign("secret")
	if errorValue != nil {
		t.Fatalf("expected signed manifest: %v", errorValue)
	}

	signedManifest.Components["admind"] = Component{
		Name:         "admind",
		Revision:     "revision-2",
		SHA256:       "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
		Size:         10,
		BlobPath:     "blobs/sha256/0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
		RestartGroup: "admind",
		HealthCheck:  "admind",
	}

	if errorValue := signedManifest.VerifySignature("secret"); errorValue == nil {
		t.Fatal("expected signature mismatch")
	}
}

func TestManifestSignatureRejectsProtocolIdentityMutation(t *testing.T) {
	manifest := NewManifest("release-1", "stable", map[string]Component{
		"admind": {
			Name:     "admind",
			Revision: "revision-1",
			SHA256:   "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
			Size:     10,
			BlobPath: "blobs/sha256/0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
		},
	})
	signedManifest, errorValue := manifest.Sign("secret")
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	signedManifest.AggregateProtocolHash = "1123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

	if errorValue := signedManifest.VerifySignature("secret"); errorValue == nil {
		t.Fatal("expected protocol identity mutation to invalidate signature")
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
