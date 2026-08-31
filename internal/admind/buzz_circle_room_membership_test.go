package admind

import (
	"os"
	"slices"
	"strings"
	"testing"
)

func TestACircleRoomIsFoundByTheNameThePolicyDeclares(t *testing.T) {
	circles := declaredCirclesOfPolicy(map[string]any{
		"circles": []any{
			map[string]any{"circleID": "c-level", "displayName": "C-level"},
			map[string]any{"circleID": "hr", "displayName": "HR"},
			map[string]any{"circleID": "", "displayName": "Nameless"},
			map[string]any{"circleID": "member"},
		},
	})

	if len(circles) != 2 {
		t.Fatalf("a circle with no id and one with no display name name no room, got %+v", circles)
	}
	if circles[1].CircleID != "hr" || circles[1].DisplayName != "HR" {
		t.Fatalf("expected the hr circle to carry both, got %+v", circles[1])
	}
}

func TestOnlyThePeopleCarryingTheCircleBelong(t *testing.T) {
	circlesByEmail := map[string][]string{
		"lee@example.test":  {"member", "admin", "c-level"},
		"rain@example.test": {"member", "admin"},
		"kwak@example.test": {"member"},
	}

	belong := emailsCarrying(circlesByEmail, "c-level")
	slices.Sort(belong)
	if !slices.Equal(belong, []string{"lee@example.test"}) {
		t.Fatalf("a room for the c-level circle holds the c-level circle, got %v", belong)
	}
}

func TestACircleNobodyCarriesHoldsNobody(t *testing.T) {
	belong := emailsCarrying(map[string][]string{"lee@example.test": {"member"}}, "representative")

	if len(belong) != 0 {
		t.Fatalf("an empty circle names nobody rather than everybody, got %v", belong)
	}
}

func TestTheMemberSyncOnlyEverFillsRoomsTheWholeCompanyCanRead(t *testing.T) {
	query := memberRoomQuery

	for _, condition := range []string{"visibility = 'open'", "deleted_at IS NULL", "created_by = ANY(ARRAY(SELECT decode(unnest($1::text[]), 'hex')))"} {
		if !strings.Contains(query, condition) {
			t.Fatalf("a room the whole company is added to must be %s: %s", condition, query)
		}
	}
}

func TestEveryRoomChangeInGoTellsTheClientsToo(t *testing.T) {
	source, errorValue := os.ReadFile("buzz_circle_room_membership.go")
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	body := string(source)

	if !strings.Contains(body, "tellClientsWhoIsInTheRoom") {
		t.Fatal("a room this device changes must have its discovery events written again, or every client keeps the members it last read")
	}
	if !strings.Contains(body, "DELETE FROM events WHERE kind IN (39000,39001,39002)") {
		t.Fatal("reconcile-channels writes an event only where none exists, so the room's own must go first")
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

func TestARoomKeepsOnlyItsLatestJoiningNotices(t *testing.T) {
	source, errorValue := os.ReadFile("buzz_circle_room_membership.go")
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	body := string(source)

	if !strings.Contains(body, "PARTITION BY channel_id ORDER BY created_at DESC") {
		t.Fatal("the newest notices a room keeps are counted per room, or one busy room decides for every other")
	}
	if !strings.Contains(body, "keepJoiningNoticesFromEatingTheWindow(syncContext)") {
		t.Fatal("pruning belongs on the same tick that writes memberships, because that is what makes them")
	}
	if joiningNoticesARoomKeeps >= 50 {
		t.Fatalf("a timeline shows fifty rows, so keeping %d leaves nothing for what people wrote", joiningNoticesARoomKeeps)
	}
}
