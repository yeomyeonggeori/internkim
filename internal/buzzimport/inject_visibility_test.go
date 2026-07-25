package buzzimport

import (
	"context"
	"database/sql"
	"os"
	"testing"
	"time"

	_ "github.com/lib/pq"
)

// Leaves an injected message in place so the relay read path can be checked
// from a client; runs only when explicitly pointed at a relay database.
func TestInjectedMessageIsLeftForClientVerification(t *testing.T) {
	databaseURL := os.Getenv("BUZZ_IMPORT_KEEP_DATABASE_URL")
	channelID := os.Getenv("BUZZ_IMPORT_KEEP_CHANNEL_ID")
	authorSecret := os.Getenv("BUZZ_IMPORT_KEEP_AUTHOR_SECRET")
	if databaseURL == "" || channelID == "" || authorSecret == "" {
		t.Skip("set BUZZ_IMPORT_KEEP_* to inject a message that stays behind")
	}
	database, errorValue := sql.Open("postgres", databaseURL)
	if errorValue != nil {
		t.Fatalf("open relay database: %v", errorValue)
	}
	defer database.Close()

	var communityID string
	if errorValue := database.QueryRow(`SELECT community_id FROM channels WHERE id = $1`, channelID).Scan(&communityID); errorValue != nil {
		t.Fatalf("resolve community: %v", errorValue)
	}

	event, errorValue := BuildStreamEvent(ImportedMessage{
		ChannelID:       channelID,
		AuthorSecretHex: authorSecret,
		Text:            "[import] 2026-03-14 이관된 과거 메시지",
		SentAt:          time.Date(2026, time.March, 14, 9, 20, 0, 0, time.UTC),
	})
	if errorValue != nil {
		t.Fatalf("build event: %v", errorValue)
	}
	injector := ChannelInjector{Database: database, CommunityID: communityID}
	if errorValue := injector.InjectMessage(context.Background(), channelID, event); errorValue != nil {
		t.Fatalf("inject message: %v", errorValue)
	}
	t.Logf("injected event %s into channel %s", event.ID, channelID)
}
