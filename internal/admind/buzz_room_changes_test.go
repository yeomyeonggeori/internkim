package admind

import (
	"os"
	"strings"
	"testing"
)

func TestEveryRoomChangeInGoTellsTheClientsToo(t *testing.T) {
	source, errorValue := os.ReadFile("buzz_room_changes.go")
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

func TestARoomKeepsOnlyItsLatestJoiningNotices(t *testing.T) {
	source, errorValue := os.ReadFile("buzz_room_changes.go")
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	body := string(source)

	if !strings.Contains(body, "PARTITION BY channel_id ORDER BY created_at DESC") {
		t.Fatal("the newest notices a room keeps are counted per room, or one busy room decides for every other")
	}
	if !strings.Contains(body, "service.withinASweepBudget(ctx, service.keepJoiningNoticesFromEatingTheWindow)") {
		t.Fatal("pruning runs on its own tick, because the syncs that seat people run hours apart and notices pile up between them")
	}
	if joiningNoticesARoomKeeps >= 50 {
		t.Fatalf("a timeline shows fifty rows, so keeping %d leaves nothing for what people wrote", joiningNoticesARoomKeeps)
	}
}
