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
		"graphiti_episode",
		"graphiti_namespace",
		"name 'kuzu*'",
	}
	for _, fragment := range requiredFragments {
		if !strings.Contains(script, fragment) {
			t.Fatalf("expected reset script to include %q", fragment)
		}
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
