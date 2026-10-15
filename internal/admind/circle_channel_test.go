package admind

import (
	"context"
	"database/sql"
	"reflect"
	"testing"
)

const theAgent = "a9e0000000000000000000000000000000000000000000000000000000000001"

func TestACircleChannelSeatsWhoeverTheCircleGained(t *testing.T) {
	held := map[string]string{theAgent: "owner", "aaa": "member"}
	wanted := map[string]bool{"aaa": true, "bbb": true}

	plan := planCircleSeats(held, wanted, theAgent)

	if !reflect.DeepEqual(plan, circleSeatPlan{Seat: []string{"bbb"}, Unseat: []string{}}) {
		t.Fatalf("only the person new to the circle should be seated, got %+v", plan)
	}
}

func TestACircleChannelUnseatsWhoeverTheCircleLostOrNeverHad(t *testing.T) {
	held := map[string]string{theAgent: "owner", "aaa": "member", "left": "member", "added-by-hand": "member"}
	wanted := map[string]bool{"aaa": true}

	plan := planCircleSeats(held, wanted, theAgent)

	if !reflect.DeepEqual(plan, circleSeatPlan{Seat: []string{}, Unseat: []string{"added-by-hand", "left"}}) {
		t.Fatalf("everyone the circle does not name should be unseated, got %+v", plan)
	}
}

func TestACircleChannelLowersAnyoneRaisedAboveTheRest(t *testing.T) {
	held := map[string]string{theAgent: "owner", "aaa": "owner"}

	plan := planCircleSeats(held, map[string]bool{"aaa": true}, theAgent)

	if !reflect.DeepEqual(plan.Seat, []string{"aaa"}) {
		t.Fatalf("a member who owns the channel could add people by hand, so they are seated again as a member, got %+v", plan)
	}
}

func TestACircleChannelNeverUnseatsTheAgentThatKeepsIt(t *testing.T) {
	plan := planCircleSeats(map[string]string{theAgent: "owner"}, map[string]bool{}, theAgent)

	if len(plan.Seat) != 0 || len(plan.Unseat) != 0 {
		t.Fatalf("an empty circle leaves the agent alone in its channel, got %+v", plan)
	}
}

func TestAChannelIsLiveRetiredOrAbsent(t *testing.T) {
	relay := disposableRelayDatabase(t)
	if _, errorValue := relay.Exec(roomShapeCreateStatement); errorValue != nil {
		t.Fatalf("create the room tables: %v", errorValue)
	}
	const (
		liveRoom    = "00000000-0000-0000-0000-0000000000b1"
		retiredRoom = "00000000-0000-0000-0000-0000000000b2"
		absentRoom  = "00000000-0000-0000-0000-0000000000b3"
	)
	holdRoom(t, relay, liveRoom, "stream", "private", "", "")
	holdRoom(t, relay, retiredRoom, "stream", "private", "", "now()")

	for channelID, expected := range map[string]buzzChannelLife{
		liveRoom: buzzChannelLive, retiredRoom: buzzChannelRetired, absentRoom: buzzChannelAbsent, "": buzzChannelAbsent,
	} {
		life, errorValue := buzzChannelLifeOf(context.Background(), relay, channelID)
		if errorValue != nil {
			t.Fatalf("read %q: %v", channelID, errorValue)
		}
		if life != expected {
			t.Fatalf("channel %q should read %d, got %d", channelID, expected, life)
		}
	}
}

func holdRoomOpenedBy(t *testing.T, relay *sql.DB, channelID, visibility, creatorPubkey string) {
	t.Helper()
	_, errorValue := relay.Exec(`
INSERT INTO channels (id, community_id, name, channel_type, visibility, created_by)
VALUES ($1, $2, 'room', 'stream', $3, decode($4, 'hex'))`,
		channelID, roomCommunity, visibility, creatorPubkey)
	if errorValue != nil {
		t.Fatalf("hold room %s: %v", channelID, errorValue)
	}
}
