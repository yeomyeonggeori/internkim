package buzzidentity

import "testing"

// These vectors are shared with the TypeScript mirror (chatd
// src/mirror/identity.ts tests). If either side changes a derivation formula,
// one of these must fail — that is the cross-language drift guard.
// chatd derives the same key from the same seed so it can sign an edit or a
// deletion as the person who asked for it. Its conformance test,
// blueclaw chatd/tests/buzz-identity.test.ts, holds these same two vectors.
func TestSecretMatchesCrossLanguageVector(t *testing.T) {
	for _, vector := range []struct {
		seed  string
		email string
		want  string
	}{
		{"mirror-test-seed", " Alice@Example.com ", "aa04132487d89014a53b0c1d6378dd99fcea6d503842036ee13602211423164b"},
		{"test-seed-1", "Sample@Example.com", "133163a288bebb795ba689ff5c32beeba804905ebb8160e245ff28f89263ebdf"},
	} {
		if got := Secret(vector.seed, vector.email); got != vector.want {
			t.Fatalf("Secret(%q, %q) vector drift: got %s want %s", vector.seed, vector.email, got, vector.want)
		}
	}
}

func TestChannelIDMatchesCrossLanguageVector(t *testing.T) {
	got := ChannelID("mirror-test-seed", "mmchannel123")
	want := "9344f32a-da9b-31fd-828b-efa4cde88523"
	if got != want {
		t.Fatalf("ChannelID vector drift: got %s want %s", got, want)
	}
}
