package cli

import (
	"strings"
	"testing"
)

func TestVerifyMattermostScriptDeletesTestMessagesAndBotReplies(t *testing.T) {
	script := verifyMattermostScript()
	requiredFragments := []string{
		"delete_post",
		"delete_verify_replies",
		"http://localhost:8065/api/v4/posts/$post_id",
		"invited_post_id",
		"uninvited_post_id",
		".user_id == $bot_user_id and .create_at >= $test_started_at",
	}
	for _, fragment := range requiredFragments {
		if !strings.Contains(script, fragment) {
			t.Fatalf("expected Mattermost verify cleanup to include %q", fragment)
		}
	}
}
