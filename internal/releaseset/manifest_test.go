package releaseset

import "testing"

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
