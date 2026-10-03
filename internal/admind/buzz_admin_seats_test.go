package admind

import (
	"context"
	"reflect"
	"strings"
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

func TestEveryStreamChannelLeavesOutWhatIsNotACompanyChannel(t *testing.T) {
	for _, left := range []string{
		"community_id = ANY($1::uuid[])",
		"channel_type = 'stream'",
		"deleted_at IS NULL",
	} {
		if !strings.Contains(everyStreamChannelQuery, left) {
			t.Fatalf("the channel query no longer says %q", left)
		}
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
