package admind

import (
	"context"
	"testing"
)

func TestAPrivateRoomNobodyIsLeftInIsRetiredAndNoOtherRoomIs(t *testing.T) {
	relay := disposableRelayDatabase(t)
	if _, errorValue := relay.Exec(roomShapeCreateStatement); errorValue != nil {
		t.Fatalf("create the room tables: %v", errorValue)
	}
	if _, errorValue := relay.Exec(`CREATE TABLE events (kind integer NOT NULL, channel_id uuid)`); errorValue != nil {
		t.Fatalf("create the events table: %v", errorValue)
	}
	const (
		neverHadAnybody = "00000000-0000-0000-0000-0000000000b1"
		everyoneLeft    = "00000000-0000-0000-0000-0000000000b2"
		somebodyStays   = "00000000-0000-0000-0000-0000000000b3"
		openAndEmpty    = "00000000-0000-0000-0000-0000000000b4"
		somebody        = "bb00000000000000000000000000000000000000000000000000000000000001"
	)
	holdRoom(t, relay, neverHadAnybody, "stream", "private", "", "")
	holdRoom(t, relay, everyoneLeft, "stream", "private", "", "")
	seatInRoom(t, relay, everyoneLeft, somebody, true)
	holdRoom(t, relay, somebodyStays, "stream", "private", "", "")
	seatInRoom(t, relay, somebodyStays, somebody, false)
	holdRoom(t, relay, openAndEmpty, "stream", "open", "", "")
	for _, room := range []string{neverHadAnybody, everyoneLeft, somebodyStays, openAndEmpty} {
		if _, errorValue := relay.Exec(`INSERT INTO events (kind, channel_id) VALUES (39000, $1), (9, $1)`, room); errorValue != nil {
			t.Fatalf("hold %s's events: %v", room, errorValue)
		}
	}

	retirePrivateRoomsNobodyIsIn(context.Background(), relay)

	for room, shouldBeRetired := range map[string]bool{
		neverHadAnybody: true,
		everyoneLeft:    true,
		somebodyStays:   false,
		openAndEmpty:    false,
	} {
		var isRetired bool
		var discoveryEvents, messages int
		if errorValue := relay.QueryRow(`
SELECT c.deleted_at IS NOT NULL,
  (SELECT count(*) FROM events e WHERE e.channel_id = c.id AND e.kind = 39000),
  (SELECT count(*) FROM events e WHERE e.channel_id = c.id AND e.kind = 9)
FROM channels c WHERE c.id = $1`, room).Scan(&isRetired, &discoveryEvents, &messages); errorValue != nil {
			t.Fatalf("read %s: %v", room, errorValue)
		}
		if isRetired != shouldBeRetired {
			t.Fatalf("room %s retired %v, want %v: only a private room nobody is in can never be entered again", room, isRetired, shouldBeRetired)
		}
		if shouldBeRetired && discoveryEvents != 0 {
			t.Fatalf("a retired room %s must leave every listing, but its discovery event stayed", room)
		}
		if messages != 1 {
			t.Fatalf("what was said in %s must stay in the store, got %d messages", room, messages)
		}
	}
}
