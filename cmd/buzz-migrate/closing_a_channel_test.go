package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The relay's channels table carries archived_at and deleted_at side by side,
// and only one of them takes a channel out of a listing. Writing the other sets
// a column nobody reads and reports a channel closed that everybody still sees.
func TestTheColumnWeSetIsTheOneTheRelayHides(t *testing.T) {
	relaySource := filepath.Join("..", "..", ".dependency", "buzz-relay", "src", "crates", "buzz-db", "src", "channel.rs")
	source, errorValue := os.ReadFile(relaySource)
	if errorValue != nil {
		t.Skipf("the relay's source is not in this checkout (%v), so nothing here can be compared", errorValue)
	}
	relay := string(source)

	start := strings.Index(relay, "pub async fn list_channels")
	if start < 0 {
		t.Fatal("the relay no longer has list_channels, so this guard is reading the wrong file")
	}
	if !strings.Contains(relay[start:], "WHERE community_id = $1 AND deleted_at IS NULL") {
		t.Error("list_channels no longer hides a channel by deleted_at, so closing one by that column stopped working")
	}

	importer, errorValue := os.ReadFile("main.go")
	if errorValue != nil {
		t.Fatalf("read the importer: %v", errorValue)
	}
	if !strings.Contains(string(importer), "UPDATE channels SET deleted_at = NOW()") {
		t.Error("the importer closes a channel by a column other than the one the relay hides by")
	}
}
