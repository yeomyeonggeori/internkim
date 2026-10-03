package admind

import (
	"os"
	"strings"
	"testing"
)

func TestTheMemberSyncOnlyEverFillsRoomsTheWholeCompanyCanRead(t *testing.T) {
	query := memberRoomQuery

	for _, condition := range []string{"visibility = 'open'", "deleted_at IS NULL", "created_by = ANY(ARRAY(SELECT decode(unnest($1::text[]), 'hex')))"} {
		if !strings.Contains(query, condition) {
			t.Fatalf("a room the whole company is added to must be %s: %s", condition, query)
		}
	}
}

func TestTheMemberSyncSkipsWhoIsAlreadyIn(t *testing.T) {
	source, errorValue := os.ReadFile("buzz_channel_membership.go")
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	body := string(source)

	if !strings.Contains(body, `if isHeld && (member.Role == "" || heldRole == member.Role) {`) {
		t.Fatal("the relay announces a joining for every add it is asked to make, so an add that changes nothing still costs a row of every timeline; only a role the room does not already hold earns one")
	}
	if !strings.Contains(body, "buzzChannelMemberRoles(ctx, relay, channelID)") {
		t.Fatal("skipping who is already in means reading who is in first, and promoting means reading the role they hold")
	}
}

func TestTheUserSyncSkipsWhoIsAlreadyIn(t *testing.T) {
	source, errorValue := os.ReadFile("buzz_channel_membership.go")
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	body := string(source)

	if !strings.Contains(body, `if heldRole, isHeld := heldRoles[pubkey]; isHeld && (role == "" || heldRole == role) {`) {
		t.Fatal("a rerun for someone already in the room must not ask the relay to add them again, or every rerun announces a joining that already happened")
	}
}
