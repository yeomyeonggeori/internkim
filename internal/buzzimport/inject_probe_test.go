package buzzimport

import (
	"context"
	"database/sql"
	"os"
	"testing"
	"time"

	_ "github.com/lib/pq"
)

// Runs only when a buzz relay database is reachable, so the normal suite stays hermetic.
func TestInjectedMessageLandsInTheRelayDatabase(t *testing.T) {
	databaseURL := os.Getenv("BUZZ_IMPORT_TEST_DATABASE_URL")
	channelID := os.Getenv("BUZZ_IMPORT_TEST_CHANNEL_ID")
	authorSecret := os.Getenv("BUZZ_IMPORT_TEST_AUTHOR_SECRET")
	if databaseURL == "" || channelID == "" || authorSecret == "" {
		t.Skip("set BUZZ_IMPORT_TEST_DATABASE_URL, BUZZ_IMPORT_TEST_CHANNEL_ID and BUZZ_IMPORT_TEST_AUTHOR_SECRET to run")
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

	sentAt := time.Date(2026, time.March, 14, 9, 20, 0, 0, time.UTC)
	event, errorValue := BuildStreamEvent(ImportedMessage{
		ChannelID:       channelID,
		AuthorSecretHex: authorSecret,
		Text:            "[import probe] 2026-03-14 원본 시각 보존",
		SentAt:          sentAt,
	})
	if errorValue != nil {
		t.Fatalf("build event: %v", errorValue)
	}

	injector := ChannelInjector{Database: database, CommunityID: communityID}
	if errorValue := injector.InjectMessage(context.Background(), channelID, event); errorValue != nil {
		t.Fatalf("inject message: %v", errorValue)
	}

	var storedAt time.Time
	if errorValue := database.QueryRow(
		`SELECT created_at FROM events WHERE community_id = $1 AND id = decode($2, 'hex')`,
		communityID, event.ID,
	).Scan(&storedAt); errorValue != nil {
		t.Fatalf("read back injected event: %v", errorValue)
	}
	if !storedAt.UTC().Equal(sentAt) {
		t.Fatalf("expected the original send time to survive, got %s", storedAt.UTC())
	}

	if errorValue := injector.InjectMessage(context.Background(), channelID, event); errorValue != nil {
		t.Fatalf("re-injecting the same message must be a no-op, got %v", errorValue)
	}
	var storedCount int
	if errorValue := database.QueryRow(
		`SELECT count(*) FROM events WHERE community_id = $1 AND id = decode($2, 'hex')`,
		communityID, event.ID,
	).Scan(&storedCount); errorValue != nil {
		t.Fatalf("count injected event: %v", errorValue)
	}
	if storedCount != 1 {
		t.Fatalf("expected one stored row after a repeated import, got %d", storedCount)
	}

	t.Cleanup(func() {
		database.Exec(`DELETE FROM event_mentions WHERE community_id = $1 AND event_id = decode($2, 'hex')`, communityID, event.ID)
		database.Exec(`DELETE FROM thread_metadata WHERE community_id = $1 AND event_id = decode($2, 'hex')`, communityID, event.ID)
		database.Exec(`DELETE FROM events WHERE community_id = $1 AND id = decode($2, 'hex')`, communityID, event.ID)
	})
}
