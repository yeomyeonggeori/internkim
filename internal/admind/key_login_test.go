package admind

import (
	"encoding/base64"
	"strconv"
	"testing"
	"time"

	nostr "github.com/nbd-wtf/go-nostr"
)

func newKeyLoginTestService(t *testing.T) *Service {
	t.Helper()
	return NewService(Configuration{StateDirectory: t.TempDir()})
}

func TestKeyLoginChallengeRoundTrip(t *testing.T) {
	service := newKeyLoginTestService(t)
	challenge, expiresAt, errorValue := service.issueKeyLoginChallenge()
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if expiresAt <= time.Now().Unix() {
		t.Fatalf("challenge already expired: %d", expiresAt)
	}
	if errorValue := service.verifyKeyLoginChallenge(challenge); errorValue != nil {
		t.Fatalf("fresh challenge must verify: %v", errorValue)
	}
}

func TestKeyLoginChallengeRejectsTampering(t *testing.T) {
	service := newKeyLoginTestService(t)
	challenge, _, errorValue := service.issueKeyLoginChallenge()
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	tampered := challenge[:len(challenge)-2] + pickDifferentSuffix(challenge)
	if service.verifyKeyLoginChallenge(tampered) == nil {
		t.Fatal("a tampered challenge must be rejected")
	}
}

func TestKeyLoginChallengeRejectsExpired(t *testing.T) {
	service := newKeyLoginTestService(t)
	key, errorValue := service.webSessionSigningKey()
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	payload := "deadbeef:" + strconv.FormatInt(time.Now().Add(-time.Hour).Unix(), 10)
	expired := base64.RawURLEncoding.EncodeToString([]byte(payload)) + "." + signChallengePayload(key, payload)
	if service.verifyKeyLoginChallenge(expired) == nil {
		t.Fatal("an expired challenge must be rejected")
	}
}

func TestKeyLoginChallengeRejectsForeignKey(t *testing.T) {
	service := newKeyLoginTestService(t)
	challenge, _, _ := service.issueKeyLoginChallenge()
	other := newKeyLoginTestService(t)
	if other.verifyKeyLoginChallenge(challenge) == nil {
		t.Fatal("a challenge signed by another server key must be rejected")
	}
}

func TestChallengeTagValueReadsTheChallengeTag(t *testing.T) {
	event := nostr.Event{Tags: nostr.Tags{nostr.Tag{"challenge", "abc123"}, nostr.Tag{"p", "someone"}}}
	if challengeTagValue(event) != "abc123" {
		t.Fatalf("expected the challenge tag, got %q", challengeTagValue(event))
	}
	if challengeTagValue(nostr.Event{Tags: nostr.Tags{nostr.Tag{"p", "x"}}}) != "" {
		t.Fatal("missing challenge tag must yield empty string")
	}
}

func pickDifferentSuffix(challenge string) string {
	last := challenge[len(challenge)-2:]
	if last == "AA" {
		return "BB"
	}
	return "AA"
}
