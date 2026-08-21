package blueclaw

import (
	"strings"
	"testing"
)

func TestUsersSyncScriptRecordsOnlyDirectoryRecordsThatNameAPerson(t *testing.T) {
	script := InternKimUsersSyncScript()

	for _, fragment := range []string{
		`select((.userID // "") != "" and (.email // "") != "")`,
		"else\n    empty\n  end",
		`sort -u > "$desired_path"`,
		`jusers="$(jq -R . "$desired_path" | jq -s .)"`,
		`--argjson users "$jusers"`,
		`install -m 600 "$next_state_path" "$STATE_PATH"`,
	} {
		if !strings.Contains(script, fragment) {
			t.Fatalf("expected users sync script to include %q", fragment)
		}
	}
	if strings.Contains(script, `.users[]? | ["", .`) {
		t.Fatal("legacy email-only users must not become persisted state targets")
	}
	filterIndex := strings.Index(script, `select((.userID // "") != "" and (.email // "") != "")`)
	desiredEmailsIndex := strings.Index(script, `sort -u > "$desired_path"`)
	stateIndex := strings.Index(script, `--argjson users "$jusers"`)
	if filterIndex < 0 || desiredEmailsIndex < filterIndex || stateIndex < desiredEmailsIndex {
		t.Fatalf("expected one filtered target set to drive persisted state, got %s", script)
	}
}

func TestUsersSyncScriptMaintainsWorkspaceDirectoriesWithoutHostPolicySync(t *testing.T) {
	script := InternKimUsersSyncScript()

	for _, fragment := range []string{
		"refresh_current_policy()",
		`request_or_exit "blueclaw policy read" "$current_policy_path" "$BLUECLAW_URL/admin/api/policy"`,
		"blueclaw-posix-helper sync",
		"--policy \"$current_policy_path\"",
		"install -d -m 0711 \"$WORKSPACE_PATH/private\" \"$WORKSPACE_PATH/private/people\" \"$WORKSPACE_PATH/circles\"",
		"chmod 0711 \"$WORKSPACE_PATH/private\" \"$WORKSPACE_PATH/private/people\" \"$WORKSPACE_PATH/circles\"",
		"$WORKSPACE_PATH/private/people/$person_id",
		"$person_path/tmp",
		"$person_path/artifacts",
		"chmod 2770 \"$person_path\" \"$person_path/tmp\" \"$person_path/artifacts\"",
	} {
		if !strings.Contains(script, fragment) {
			t.Fatalf("expected users sync script to include %q", fragment)
		}
	}
	if strings.Contains(script, `--policy "$POLICY_PATH"`) {
		t.Fatal("expected POSIX sync to avoid the pre-launch host policy copy")
	}
}

func TestUsersSyncScriptLeavesRosterRemovalToTheHost(t *testing.T) {
	script := InternKimUsersSyncScript()

	for _, fragment := range []string{
		"is_preserved_local_email",
		"admin_email",
		"write_removable_policy_emails",
		"policy_removable_path",
	} {
		if strings.Contains(script, fragment) {
			t.Fatalf("removal safety rule %q belongs where the removal happens, which is admind, not this script", fragment)
		}
	}
}
