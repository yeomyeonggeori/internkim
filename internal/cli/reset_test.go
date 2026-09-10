package cli

import (
	"strings"
	"testing"
)

func TestBlueclawHistoryResetScriptDeletesMemoryAndConversationState(t *testing.T) {
	script := blueclawHistoryResetScript()
	requiredFragments := []string{
		"task_run",
		"raw_event",
		"conversation",
		"memory_record",
		"memory_source",
		"memory_job",
		"memory_profile",
		"memory_fact",
		"memory_episode",
	}
	for _, fragment := range requiredFragments {
		if !strings.Contains(script, fragment) {
			t.Fatalf("expected reset script to include %q", fragment)
		}
	}
}

func TestBlueclawHistoryResetScriptTruncatesMemoryTablesDependentsFirst(t *testing.T) {
	script := blueclawHistoryResetScript()
	tableOrder := []string{"memory_job", "memory_profile", "memory_fact", "memory_episode"}
	previousIndex := -1
	for _, tableName := range tableOrder {
		index := strings.Index(script, tableName)
		if index <= previousIndex {
			t.Fatalf("expected %q to be truncated after its dependents, got:\n%s", tableName, script)
		}
		previousIndex = index
	}
	if !strings.Contains(script, "memory_episode\nRESTART IDENTITY CASCADE") {
		t.Fatalf("expected the memory tables to be truncated with CASCADE, got:\n%s", script)
	}
}

func TestBlueclawHistoryResetScriptKeepsIdentityAndPolicyState(t *testing.T) {
	script := blueclawHistoryResetScript()
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

func TestBlueclawHistoryResetScriptLeavesTheMessengerAlone(t *testing.T) {
	script := blueclawHistoryResetScript()
	forbiddenFragments := []string{
		"UPDATE posts",
		"DELETE FROM reactions",
		"DELETE FROM threadmemberships",
		"DELETE FROM threads",
	}
	for _, fragment := range forbiddenFragments {
		if strings.Contains(script, fragment) {
			t.Fatalf("expected reset script to omit %q", fragment)
		}
	}
}
