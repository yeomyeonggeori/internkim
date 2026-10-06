package admind

import (
	"context"
	"reflect"
	"testing"
)

func TestAnAdministratorWithNoSeatIsNamed(t *testing.T) {
	held := map[string]string{"aaa": "member"}
	if named := adminsMissingTheirSeat(held, []string{"aaa", "bbb"}); !reflect.DeepEqual(named, []string{"aaa", "bbb"}) {
		t.Fatalf("both administrators should be named, got %v", named)
	}
}

func TestAnAdministratorAlreadySeatedIsLeftAlone(t *testing.T) {
	held := map[string]string{"aaa": "owner"}
	if named := adminsMissingTheirSeat(held, []string{"aaa"}); len(named) != 0 {
		t.Fatalf("an owner needs no seating, got %v", named)
	}
}

func TestAnAdministratorHoldingTheLesserRoleIsStillRaised(t *testing.T) {
	held := map[string]string{"aaa": "admin"}
	if named := adminsMissingTheirSeat(held, []string{"aaa"}); !reflect.DeepEqual(named, []string{"aaa"}) {
		t.Fatalf("an admin seat is not an owner seat, got %v", named)
	}
}

func TestARoomHoldingEveryAdministratorNeedsNothing(t *testing.T) {
	held := map[string]string{"aaa": "owner", "bbb": "owner", "ccc": "member"}
	if named := adminsMissingTheirSeat(held, []string{"aaa", "bbb"}); len(named) != 0 {
		t.Fatalf("nothing should be named, got %v", named)
	}
}

func TestACompanyWithNoAdministratorNamesNobody(t *testing.T) {
	if named := adminsMissingTheirSeat(map[string]string{"aaa": "owner"}, nil); len(named) != 0 {
		t.Fatalf("nobody should be named, got %v", named)
	}
}

func TestAdministratorsAreSeatedOnlyInRoomsTheRelayLetsThemInto(t *testing.T) {
	relay := disposableRelayDatabase(t)
	if _, errorValue := relay.Exec(roomShapeCreateStatement); errorValue != nil {
		t.Fatalf("create the room tables: %v", errorValue)
	}
	const (
		liveRoom      = "00000000-0000-0000-0000-0000000000a1"
		archivedRoom  = "00000000-0000-0000-0000-0000000000a2"
		emptyRoom     = "00000000-0000-0000-0000-0000000000a3"
		everyoneLeft  = "00000000-0000-0000-0000-0000000000a4"
		deletedRoom   = "00000000-0000-0000-0000-0000000000a5"
		directRoom    = "00000000-0000-0000-0000-0000000000a6"
		somebodysSeat = "aa00000000000000000000000000000000000000000000000000000000000001"
	)
	holdRoom(t, relay, liveRoom, "stream", "private", "", "")
	seatInRoom(t, relay, liveRoom, somebodysSeat, false)
	holdRoom(t, relay, archivedRoom, "stream", "private", "now()", "")
	seatInRoom(t, relay, archivedRoom, somebodysSeat, false)
	holdRoom(t, relay, emptyRoom, "stream", "private", "", "")
	holdRoom(t, relay, everyoneLeft, "stream", "private", "", "")
	seatInRoom(t, relay, everyoneLeft, somebodysSeat, true)
	holdRoom(t, relay, deletedRoom, "stream", "private", "", "now()")
	seatInRoom(t, relay, deletedRoom, somebodysSeat, false)
	holdRoom(t, relay, directRoom, "dm", "private", "", "")
	seatInRoom(t, relay, directRoom, somebodysSeat, false)

	channelIDs, errorValue := everyStreamChannel(context.Background(), relay, []string{roomCommunity})
	if errorValue != nil {
		t.Fatalf("read the rooms: %v", errorValue)
	}

	if !reflect.DeepEqual(channelIDs, []string{liveRoom}) {
		t.Fatalf("only a live, unarchived company room somebody is still in can take an administrator, got %v", channelIDs)
	}
}

func TestEveryStreamChannelAsksForNothingWhenNoCommunityIsKnown(t *testing.T) {
	channelIDs, errorValue := everyStreamChannel(context.Background(), nil, nil)
	if errorValue != nil || channelIDs != nil {
		t.Fatalf("a device that knows no community should ask for nothing, got %v %v", channelIDs, errorValue)
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

func TestSeatsNotTakenAddsUpWhatEveryRoomRefused(t *testing.T) {
	planned := []adminSeatPlan{
		{ChannelID: "one", Pubkeys: []string{"aaa"}, Unseated: 1},
		{ChannelID: "two", Pubkeys: []string{"aaa", "bbb"}},
		{ChannelID: "three", Pubkeys: []string{"bbb"}, Unseated: 1},
	}
	if total := seatsNotTaken(planned); total != 2 {
		t.Fatalf("two seats went untaken, got %d", total)
	}
}

func TestSeatsNotTakenIsNothingWhenEveryRoomTookThem(t *testing.T) {
	if total := seatsNotTaken([]adminSeatPlan{{ChannelID: "one", Pubkeys: []string{"aaa"}}}); total != 0 {
		t.Fatalf("nothing should be outstanding, got %d", total)
	}
}
