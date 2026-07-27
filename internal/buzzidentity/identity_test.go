package buzzidentity

import "testing"

// These vectors are shared with the TypeScript mirror (chatd
// src/mirror/identity.ts tests). If either side changes a derivation formula,
// one of these must fail — that is the cross-language drift guard.
func TestSecretMatchesCrossLanguageVector(t *testing.T) {
	got := Secret("mirror-test-seed", " Alice@Example.com ")
	want := "aa04132487d89014a53b0c1d6378dd99fcea6d503842036ee13602211423164b"
	if got != want {
		t.Fatalf("Secret vector drift: got %s want %s", got, want)
	}
}

func TestChannelIDMatchesCrossLanguageVector(t *testing.T) {
	got := ChannelID("mirror-test-seed", "mmchannel123")
	want := "9344f32a-da9b-31fd-828b-efa4cde88523"
	if got != want {
		t.Fatalf("ChannelID vector drift: got %s want %s", got, want)
	}
}
