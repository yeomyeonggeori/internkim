package admind

import (
	"testing"

	nostr "github.com/nbd-wtf/go-nostr"
)

func TestRemapEventReferencesRepointsOwnThreadReplies(t *testing.T) {
	newByOld := map[string]string{"old-root": "new-root"}
	tags := nostr.Tags{
		nostr.Tag{"h", "channel-1"},
		nostr.Tag{"e", "old-root", "", "reply"},
	}
	remapped := remapEventReferences(tags, newByOld)

	if remapped[0][1] != "channel-1" {
		t.Fatalf("h tag must be untouched, got %v", remapped[0])
	}
	if remapped[1][1] != "new-root" {
		t.Fatalf("expected reply e-tag remapped to new id, got %v", remapped[1])
	}
	if remapped[1][3] != "reply" {
		t.Fatalf("reply marker must be preserved, got %v", remapped[1])
	}
}

func TestRemapEventReferencesLeavesForeignReferences(t *testing.T) {
	remapped := remapEventReferences(nostr.Tags{nostr.Tag{"e", "someone-elses-event"}}, map[string]string{"old": "new"})
	if remapped[0][1] != "someone-elses-event" {
		t.Fatalf("references not in the map must be left untouched, got %v", remapped[0])
	}
}

func TestRemapEventReferencesDoesNotMutateInput(t *testing.T) {
	original := nostr.Tags{nostr.Tag{"e", "old"}}
	remapEventReferences(original, map[string]string{"old": "new"})
	if original[0][1] != "old" {
		t.Fatal("input tags must not be mutated")
	}
}

func TestBuzzKeyForVersionMatchesVersionedDerivation(t *testing.T) {
	first := buzzKeyForVersion("seed", "a@example.com", 1)
	second := buzzKeyForVersion("seed", "a@example.com", 2)
	if first == second {
		t.Fatal("different versions must produce different keys")
	}
	if first != buzzKeyForVersion("seed", "A@Dawn.KIM", 1) {
		t.Fatal("version 1 must be stable across email casing")
	}
}
