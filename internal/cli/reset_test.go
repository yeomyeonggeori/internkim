package cli

import (
	"strings"
	"testing"
)

func TestBlueclawHistoryResetScriptDeletesMemoryConversationAndMattermostState(t *testing.T) {
	script := blueclawHistoryResetScript(false)
	requiredFragments := []string{
		"task_run",
		"raw_event",
		"conversation",
		"memory_record",
		"memory_source",
		"graphiti_episode",
		"graphiti_namespace",
		"name 'kuzu*'",
		"resetting Mattermost visible posts",
		"UPDATE posts",
		"WHERE deleteat = 0",
	}
	for _, fragment := range requiredFragments {
		if !strings.Contains(script, fragment) {
			t.Fatalf("expected reset script to include %q", fragment)
		}
	}
}

func TestBlueclawHistoryResetScriptKeepsIdentityAndPolicyState(t *testing.T) {
	script := blueclawHistoryResetScript(false)
	forbiddenFragments := []string{
		"TRUNCATE TABLE person",
		"TRUNCATE TABLE person_email",
		"TRUNCATE TABLE platform_account",
		"TRUNCATE TABLE policy_revision",
		"TRUNCATE TABLE policy_channel_rule",
	}
	for _, fragment := range forbiddenFragments {
		if strings.Contains(script, fragment) {
			t.Fatalf("expected reset script to keep %q", fragment)
		}
	}
}

func TestBlueclawHistoryResetScriptCanKeepMattermostPostsForDebugging(t *testing.T) {
	script := blueclawHistoryResetScript(true)
	forbiddenFragments := []string{
		"resetting Mattermost visible posts",
		"UPDATE posts",
		"systemctl stop mattermost",
	}
	for _, fragment := range forbiddenFragments {
		if strings.Contains(script, fragment) {
			t.Fatalf("expected keep-mattermost reset script to omit %q", fragment)
		}
	}
}

func TestBlueclawHistoryResetScriptDeletesMattermostVisiblePostsByDefault(t *testing.T) {
	script := blueclawHistoryResetScript(false)
	requiredFragments := []string{
		"resetting Mattermost visible posts",
		"UPDATE posts",
		"WHERE deleteat = 0",
		"DELETE FROM reactions",
		"DELETE FROM threadmemberships",
		"DELETE FROM threads",
	}
	for _, fragment := range requiredFragments {
		if !strings.Contains(script, fragment) {
			t.Fatalf("expected Mattermost reset script to include %q", fragment)
		}
	}
}
