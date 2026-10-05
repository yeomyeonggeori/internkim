package admind

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const roomWhoseMembersChanged = "00000000-0000-0000-0000-0000000000f1"

func serviceRunningAFakeBuzzAdmin(t *testing.T, relayKeySetting string) (*Service, string) {
	t.Helper()
	service := serviceHoldingTheSeed(t)
	directory := t.TempDir()
	recordPath := filepath.Join(directory, "ran")
	commandPath := filepath.Join(directory, "buzz-admin")
	script := "#!/bin/sh\nprintf '%s|%s|%s\\n' \"$*\" \"$BUZZ_RELAY_PRIVATE_KEY\" \"$RELAY_URL\" >> " + recordPath + "\n"
	if errorValue := os.WriteFile(commandPath, []byte(script), 0o700); errorValue != nil {
		t.Fatalf("write fake buzz-admin: %v", errorValue)
	}
	service.Configuration.BuzzAdminCommandPath = commandPath
	service.Configuration.BuzzDatabaseURL = "postgres://buzz@127.0.0.1:1/buzz?sslmode=disable"
	service.Configuration.BuzzRelayPublicURL = "wss://relay.example.com"
	if relayKeySetting != "" {
		service.Configuration.BuzzRelayKeyPath = writeTestFile(t, relayKeySetting+"\n")
	}
	return service, recordPath
}

func TestARoomKeepsItsDiscoveryEventsWhenNoClientCouldBeTold(t *testing.T) {
	unreachableRelay, errorValue := sql.Open("postgres", "postgres://buzz@127.0.0.1:1/buzz?sslmode=disable")
	if errorValue != nil {
		t.Fatalf("open the relay handle: %v", errorValue)
	}
	defer unreachableRelay.Close()

	for name, relayKeySetting := range map[string]string{
		"the host holds no relay key":         "",
		"the relay key file names no key":     "BUZZ_RELAY_PRIVATE_KEY=",
		"the relay key file holds other keys": "DATABASE_URL=postgres://elsewhere",
	} {
		service, recordPath := serviceRunningAFakeBuzzAdmin(t, relayKeySetting)

		errorValue := service.tellClientsWhoIsInTheRoom(context.Background(), unreachableRelay, roomWhoseMembersChanged)

		if !errors.Is(errorValue, errNoClientCanBeTold) {
			t.Fatalf("%s: the room's discovery events must be left alone before anything touches the relay, got %v", name, errorValue)
		}
		if _, statError := os.Stat(recordPath); statError == nil {
			t.Fatalf("%s: buzz-admin ran without the relay's key, so it would sign as nobody", name)
		}
	}
}

func TestARoomChangeIsWrittenAgainAsTheRelay(t *testing.T) {
	relay := disposableRelayDatabase(t)
	if _, errorValue := relay.Exec(`CREATE TABLE events (kind integer NOT NULL, channel_id uuid)`); errorValue != nil {
		t.Fatalf("create the events table: %v", errorValue)
	}
	if _, errorValue := relay.Exec(`INSERT INTO events (kind, channel_id) VALUES (39000, $1), (39002, $1), (9, $1)`, roomWhoseMembersChanged); errorValue != nil {
		t.Fatalf("hold the room's events: %v", errorValue)
	}
	service, recordPath := serviceRunningAFakeBuzzAdmin(t, "BUZZ_RELAY_PRIVATE_KEY=relay-signing-key")

	if errorValue := service.tellClientsWhoIsInTheRoom(context.Background(), relay, roomWhoseMembersChanged); errorValue != nil {
		t.Fatalf("tell the clients: %v", errorValue)
	}

	ran, errorValue := os.ReadFile(recordPath)
	if errorValue != nil {
		t.Fatalf("buzz-admin never ran: %v", errorValue)
	}
	if got := strings.TrimSpace(string(ran)); got != "reconcile-channels|relay-signing-key|wss://relay.example.com" {
		t.Fatalf("the room must be written again by the relay's own key for its public community, got %q", got)
	}
	var kinds []int
	rows, errorValue := relay.Query(`SELECT kind FROM events WHERE channel_id = $1 ORDER BY kind`, roomWhoseMembersChanged)
	if errorValue != nil {
		t.Fatalf("read the room's events: %v", errorValue)
	}
	defer rows.Close()
	for rows.Next() {
		var kind int
		if errorValue := rows.Scan(&kind); errorValue != nil {
			t.Fatal(errorValue)
		}
		kinds = append(kinds, kind)
	}
	if len(kinds) != 1 || kinds[0] != 9 {
		t.Fatalf("only the room's discovery events give way to the rewrite, its messages stay: %v", kinds)
	}
}
