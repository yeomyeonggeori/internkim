package buzzimport

import (
	"context"
	"database/sql"
	"os"
	"strings"
	"testing"

	_ "github.com/lib/pq"
)

// Carries a seeded Mattermost channel into a live buzz relay and reads it back,
// which is the only way to know the two schemas actually line up.
func TestMattermostChannelImportsIntoBuzz(t *testing.T) {
	mattermostURL := os.Getenv("MATTERMOST_IMPORT_TEST_DATABASE_URL")
	teamName := os.Getenv("MATTERMOST_IMPORT_TEST_TEAM")
	buzzURL := os.Getenv("BUZZ_IMPORT_TEST_DATABASE_URL")
	buzzChannelID := os.Getenv("BUZZ_IMPORT_TEST_CHANNEL_ID")
	leeSecret := os.Getenv("BUZZ_IMPORT_TEST_AUTHOR_SECRET")
	kwakSecret := os.Getenv("BUZZ_IMPORT_TEST_SECOND_AUTHOR_SECRET")
	if mattermostURL == "" || teamName == "" || buzzURL == "" || buzzChannelID == "" || leeSecret == "" || kwakSecret == "" {
		t.Skip("set the mattermost and buzz import test variables to run")
	}

	mattermostDatabase, errorValue := sql.Open("postgres", mattermostURL)
	if errorValue != nil {
		t.Fatalf("open mattermost: %v", errorValue)
	}
	defer mattermostDatabase.Close()
	buzzDatabase, errorValue := sql.Open("postgres", buzzURL)
	if errorValue != nil {
		t.Fatalf("open buzz: %v", errorValue)
	}
	defer buzzDatabase.Close()

	ctx := context.Background()
	source := MattermostSource{Database: mattermostDatabase, TeamName: teamName}
	channels, errorValue := source.Channels(ctx)
	if errorValue != nil {
		t.Fatalf("read channels: %v", errorValue)
	}
	var smokeChannelID string
	for _, channel := range channels {
		if channel.Name == "import-smoke" {
			smokeChannelID = channel.ID
		}
	}
	if smokeChannelID == "" {
		t.Fatal("seeded channel is missing")
	}
	posts, errorValue := source.Posts(ctx, smokeChannelID)
	if errorValue != nil {
		t.Fatalf("read posts: %v", errorValue)
	}
	users, errorValue := source.Users(ctx)
	if errorValue != nil {
		t.Fatalf("read users: %v", errorValue)
	}
	authorEmails := map[string]string{}
	for _, user := range users {
		authorEmails[user.ID] = user.Email
	}

	resolver := stubResolver{
		secretsByEmail:    map[string]string{"lee@dawn.kim": leeSecret, "kwak@dawn.kim": kwakSecret},
		pubkeysByUsername: map[string]string{},
	}
	messages, skipped, errorValue := PlanChannelImport(ChannelImportPlan{
		BuzzChannelID: buzzChannelID,
		Posts:         posts,
		AuthorEmails:  authorEmails,
	}, resolver)
	if errorValue != nil {
		t.Fatalf("plan import: %v", errorValue)
	}
	if len(messages) == 0 {
		t.Fatalf("nothing planned; skipped %v", skipped)
	}

	var communityID string
	if errorValue := buzzDatabase.QueryRow(`SELECT community_id FROM channels WHERE id = $1`, buzzChannelID).Scan(&communityID); errorValue != nil {
		t.Fatalf("resolve community: %v", errorValue)
	}
	injector := ChannelInjector{Database: buzzDatabase, CommunityID: communityID}
	injectedEventIDs := []string{}
	for _, message := range messages {
		event, errorValue := BuildStreamEvent(message)
		if errorValue != nil {
			t.Fatalf("build event: %v", errorValue)
		}
		if errorValue := injector.InjectMessage(ctx, buzzChannelID, event); errorValue != nil {
			t.Fatalf("inject event: %v", errorValue)
		}
		injectedEventIDs = append(injectedEventIDs, event.ID)
	}
	t.Cleanup(func() {
		for _, eventID := range injectedEventIDs {
			buzzDatabase.Exec(`DELETE FROM event_mentions WHERE community_id = $1 AND event_id = decode($2,'hex')`, communityID, eventID)
			buzzDatabase.Exec(`DELETE FROM thread_metadata WHERE community_id = $1 AND event_id = decode($2,'hex')`, communityID, eventID)
			buzzDatabase.Exec(`DELETE FROM events WHERE community_id = $1 AND id = decode($2,'hex')`, communityID, eventID)
		}
	})

	for index, message := range messages {
		var storedContent string
		var storedAuthor string
		if errorValue := buzzDatabase.QueryRow(
			`SELECT content, encode(pubkey,'hex') FROM events WHERE community_id = $1 AND id = decode($2,'hex')`,
			communityID, injectedEventIDs[index],
		).Scan(&storedContent, &storedAuthor); errorValue != nil {
			t.Fatalf("read back event %d: %v", index, errorValue)
		}
		if storedContent != message.Text {
			t.Fatalf("message text changed in transit: %q", storedContent)
		}
		if storedAuthor == "" {
			t.Fatal("imported event lost its author")
		}
	}

	var threadRows int
	if errorValue := buzzDatabase.QueryRow(
		`SELECT count(*) FROM thread_metadata WHERE community_id = $1 AND event_id = decode($2,'hex')`,
		communityID, injectedEventIDs[len(injectedEventIDs)-1],
	).Scan(&threadRows); errorValue != nil {
		t.Fatalf("read thread metadata: %v", errorValue)
	}
	if threadRows != 1 {
		t.Fatalf("the reply must carry thread metadata, got %d rows", threadRows)
	}

	var replyContent string
	if errorValue := buzzDatabase.QueryRow(
		`SELECT content FROM events WHERE community_id = $1 AND id = decode($2,'hex')`,
		communityID, injectedEventIDs[len(injectedEventIDs)-1],
	).Scan(&replyContent); errorValue != nil {
		t.Fatalf("read reply: %v", errorValue)
	}
	if !strings.Contains(replyContent, "확인 부탁드립니다") {
		t.Fatalf("unexpected reply content: %q", replyContent)
	}
}
