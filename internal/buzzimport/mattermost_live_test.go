package buzzimport

import (
	"context"
	"database/sql"
	"os"
	"strings"
	"testing"

	_ "github.com/lib/pq"
)

// Reads a real Mattermost database so the queries are checked against the
// shipped schema rather than an assumption about it.
func TestMattermostSourceReadsARealWorkspace(t *testing.T) {
	databaseURL := os.Getenv("MATTERMOST_IMPORT_TEST_DATABASE_URL")
	teamName := os.Getenv("MATTERMOST_IMPORT_TEST_TEAM")
	if databaseURL == "" || teamName == "" {
		t.Skip("set MATTERMOST_IMPORT_TEST_DATABASE_URL and MATTERMOST_IMPORT_TEST_TEAM to run")
	}
	database, errorValue := sql.Open("postgres", databaseURL)
	if errorValue != nil {
		t.Fatalf("open mattermost database: %v", errorValue)
	}
	defer database.Close()
	source := MattermostSource{Database: database, TeamName: teamName}
	ctx := context.Background()

	users, errorValue := source.Users(ctx)
	if errorValue != nil {
		t.Fatalf("read users: %v", errorValue)
	}
	if len(users) == 0 {
		t.Fatal("expected the seeded users")
	}
	emails := map[string]bool{}
	for _, user := range users {
		emails[strings.ToLower(user.Email)] = true
	}
	if !emails["lee@example.com"] || !emails["kwak@example.com"] {
		t.Fatalf("expected the seeded addresses, got %v", emails)
	}

	channels, errorValue := source.Channels(ctx)
	if errorValue != nil {
		t.Fatalf("read channels: %v", errorValue)
	}
	var smokeChannel MattermostChannel
	for _, channel := range channels {
		if channel.Name == "import-smoke" {
			smokeChannel = channel
		}
	}
	if smokeChannel.ID == "" {
		t.Fatalf("expected the seeded channel among %d channels", len(channels))
	}
	if smokeChannel.DisplayName != "Import Smoke" || smokeChannel.Purpose == "" {
		t.Fatalf("channel metadata did not come through: %+v", smokeChannel)
	}

	memberEmails, errorValue := source.ChannelMemberEmails(ctx, smokeChannel.ID)
	if errorValue != nil {
		t.Fatalf("read channel members: %v", errorValue)
	}
	if len(memberEmails) < 2 {
		t.Fatalf("expected the seeded members, got %v", memberEmails)
	}

	posts, errorValue := source.Posts(ctx, smokeChannel.ID)
	if errorValue != nil {
		t.Fatalf("read posts: %v", errorValue)
	}
	if len(posts) != 2 {
		t.Fatalf("expected the two surviving posts, got %d: %+v", len(posts), posts)
	}
	if posts[0].RootID != "" {
		t.Fatalf("expected the root post first, got %+v", posts[0])
	}
	if posts[1].RootID != posts[0].ID {
		t.Fatalf("expected the reply to point at the root, got %+v", posts[1])
	}
	if posts[0].CreatedAt.After(posts[1].CreatedAt) {
		t.Fatal("posts must come back oldest first")
	}
	if posts[0].CreatedAt.IsZero() || posts[0].CreatedAt.Year() < 2020 {
		t.Fatalf("post time did not convert from milliseconds: %s", posts[0].CreatedAt)
	}
	for _, post := range posts {
		if strings.Contains(post.Message, "삭제될 메시지") {
			t.Fatal("deleted posts must not be imported")
		}
	}
}
