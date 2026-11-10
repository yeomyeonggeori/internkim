package admind

import (
	"context"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/yeomyeonggeori/internkim/internal/buzzidentity"
)

func TestAnAdministratorWhoLeftIsNotSeatedAgainAndAnInvitedOneIsRaised(t *testing.T) {
	administrators := []seatableAdministrator{
		{Pubkey: "bbb", MemberID: "member-b"},
		{Pubkey: "aaa", MemberID: "member-a"},
		{Pubkey: "left", MemberID: "member-left"},
		{Pubkey: "invited", MemberID: "member-invited"},
		{Pubkey: "owner", MemberID: "member-owner"},
	}
	heldRoles := map[string]string{"invited": "member", "owner": "owner"}
	everSeated := map[string]bool{"left": true, "invited": true, "owner": true}

	plan := planAdministratorSeats(heldRoles, everSeated, administrators)

	wantNewcomers := []seatableAdministrator{{Pubkey: "aaa", MemberID: "member-a"}, {Pubkey: "bbb", MemberID: "member-b"}}
	wantRaised := []seatableAdministrator{{Pubkey: "invited", MemberID: "member-invited"}}
	if !reflect.DeepEqual(plan.Newcomers, wantNewcomers) || !reflect.DeepEqual(plan.Raised, wantRaised) {
		t.Fatalf("administrators never in the channel are seated and an invited one is raised, got %+v", plan)
	}
}

func TestAdministratorsAreSeatedOnlyInChannelsTheyMayBeOpenedInto(t *testing.T) {
	relay := disposableRelayDatabase(t)
	if _, errorValue := relay.Exec(roomShapeCreateStatement); errorValue != nil {
		t.Fatalf("create the room tables: %v", errorValue)
	}
	const (
		liveRoom     = "00000000-0000-0000-0000-0000000000a1"
		archivedRoom = "00000000-0000-0000-0000-0000000000a2"
		deletedRoom  = "00000000-0000-0000-0000-0000000000a5"
		directRoom   = "00000000-0000-0000-0000-0000000000a6"
		circleRoom   = "00000000-0000-0000-0000-0000000000a7"
		unknownRoom  = "00000000-0000-0000-0000-0000000000a8"
	)
	agentKey := derivedKey(t, buzzidentity.AgentSubject)
	holdRoom(t, relay, liveRoom, "stream", "private", "", "")
	holdRoom(t, relay, archivedRoom, "stream", "private", "now()", "")
	holdRoom(t, relay, deletedRoom, "stream", "private", "", "now()")
	holdRoom(t, relay, directRoom, "dm", "private", "", "")
	holdRoomOpenedBy(t, relay, circleRoom, "private", agentKey)

	answers := map[string]bool{}
	for _, channelID := range []string{liveRoom, archivedRoom, deletedRoom, directRoom, circleRoom, unknownRoom} {
		isSeatable, errorValue := channelTakesAdministrators(context.Background(), relay, channelID, agentKey)
		if errorValue != nil {
			t.Fatalf("read %s: %v", channelID, errorValue)
		}
		answers[channelID] = isSeatable
	}

	want := map[string]bool{liveRoom: true, archivedRoom: false, deletedRoom: false, directRoom: false, circleRoom: false, unknownRoom: false}
	if !reflect.DeepEqual(answers, want) {
		t.Fatalf("only a live stream channel the company does not seat by name takes administrators, got %v", answers)
	}
}

func TestSomebodyWhoLeftAChannelStillCountsAsSeatedInIt(t *testing.T) {
	relay := disposableRelayDatabase(t)
	if _, errorValue := relay.Exec(roomShapeCreateStatement); errorValue != nil {
		t.Fatalf("create the room tables: %v", errorValue)
	}
	const (
		room       = "00000000-0000-0000-0000-0000000000b1"
		stayed     = "aa00000000000000000000000000000000000000000000000000000000000001"
		leftIt     = "aa00000000000000000000000000000000000000000000000000000000000002"
		neverThere = "aa00000000000000000000000000000000000000000000000000000000000003"
	)
	holdRoom(t, relay, room, "stream", "open", "", "")
	seatInRoom(t, relay, room, stayed, false)
	seatInRoom(t, relay, room, leftIt, true)

	everSeated, errorValue := everSeatedPubkeys(context.Background(), relay, room)
	if errorValue != nil {
		t.Fatalf("read who was seated: %v", errorValue)
	}

	if !everSeated[stayed] || !everSeated[leftIt] || everSeated[neverThere] {
		t.Fatalf("whoever stayed or left counts as seated and nobody else, got %v", everSeated)
	}
}

func TestTheRoomsOwnOwnersAreNamedInKeyOrder(t *testing.T) {
	held := map[string]string{"ccc": "owner", "aaa": "admin", "bbb": "member"}
	if named := seatedOwnerPubkeys(held); !reflect.DeepEqual(named, []string{"aaa", "ccc"}) {
		t.Fatalf("only the elevated should be named, got %v", named)
	}
}

func TestARoomNobodyAdministersNamesNoOwnerToSignAs(t *testing.T) {
	if named := seatedOwnerPubkeys(map[string]string{"aaa": "member"}); len(named) != 0 {
		t.Fatalf("a plain member cannot seat anybody, got %v", named)
	}
}

func TestSeatingAdministratorsIsRefusedToACallerWhoNamesNobody(t *testing.T) {
	service := NewService(Configuration{})
	request := httptest.NewRequest(http.MethodPost, "/agent/api/buzz-seat-admins", strings.NewReader(`{"channelID":"00000000-0000-0000-0000-0000000000a1"}`))
	recorder := httptest.NewRecorder()

	service.handleBuzzSeatAdmins(recorder, request)

	if recorder.Code != http.StatusForbidden {
		t.Fatalf("a caller who names no requester must be refused, got %d", recorder.Code)
	}
}
