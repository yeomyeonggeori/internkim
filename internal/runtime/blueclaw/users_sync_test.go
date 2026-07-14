package blueclaw

import (
	"strings"
	"testing"
)

func TestUsersSyncScriptReadsCanonicalAdminEmailWithLegacyFallback(t *testing.T) {
	script := InternKimUsersSyncScript()

	for _, fragment := range []string{
		"/root/.internkim/config/admin-email",
		"/root/.internkim/admin-email",
		`[ "$email" = "$admin_email" ] && continue`,
	} {
		if !strings.Contains(script, fragment) {
			t.Fatalf("expected users sync script to include %q", fragment)
		}
	}
}

func TestUsersSyncScriptOnlyPersistsInvitablePolicyTargets(t *testing.T) {
	script := InternKimUsersSyncScript()

	for _, fragment := range []string{
		`select((.userID // "") != "" and (.email // "") != "")`,
		"else\n    empty\n  end",
		`cut -f2 "$desired_records_path"`,
		`done < "$desired_records_path"`,
		`jusers="$(jq -R . "$desired_path" | jq -s .)"`,
		`--argjson users "$jusers"`,
	} {
		if !strings.Contains(script, fragment) {
			t.Fatalf("expected users sync script to include %q", fragment)
		}
	}
	if strings.Contains(script, `.users[]? | ["", .`) {
		t.Fatal("legacy email-only users must not become policy or state targets")
	}
	desiredRecordsIndex := strings.Index(script, `select((.userID // "") != "" and (.email // "") != "")`)
	desiredEmailsIndex := strings.Index(script, `cut -f2 "$desired_records_path"`)
	inviteIndex := strings.Index(script, `done < "$desired_records_path"`)
	stateIndex := strings.Index(script, `--argjson users "$jusers"`)
	if desiredRecordsIndex < 0 || desiredEmailsIndex < desiredRecordsIndex || inviteIndex < desiredEmailsIndex || stateIndex < inviteIndex {
		t.Fatalf("expected one filtered target set to drive policy invitations and persisted state, got %s", script)
	}
}

func TestUsersSyncScriptMaintainsWorkspaceDirectoriesWithoutHostPolicySync(t *testing.T) {
	script := InternKimUsersSyncScript()

	for _, fragment := range []string{
		`{personID:$personID,email:$email,circles:`,
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
	if strings.Contains(script, "blueclaw-posix-helper sync") || strings.Contains(script, "sync_posix_policy") {
		t.Fatal("users sync must leave POSIX policy reconciliation to the authoritative guest runtime")
	}
}

func TestUsersSyncScriptPreservesLocalTestUsers(t *testing.T) {
	script := InternKimUsersSyncScript()

	for _, fragment := range []string{
		"is_preserved_local_email()",
		"*@internkim.test) return 0",
		"write_removable_policy_emails",
		"if is_preserved_local_email \"$email\"; then",
	} {
		if !strings.Contains(script, fragment) {
			t.Fatalf("expected users sync script to include %q", fragment)
		}
	}
}

func TestTemporaryCleanupScriptOnlyTargetsPersonTaskTemporaryDirectories(t *testing.T) {
	script := InternKimBlueclawTemporaryCleanupScript()

	for _, fragment := range []string{
		"/root/.blueclaw/workspace",
		"/private/people",
		"-path \"$PEOPLE_PATH/*/tmp/*\"",
		"-mtime +6",
		"-exec rm -rf {}",
	} {
		if !strings.Contains(script, fragment) {
			t.Fatalf("expected cleanup script to include %q", fragment)
		}
	}
	if strings.Contains(script, "artifacts") {
		t.Fatal("cleanup script must not target promoted artifacts")
	}
}

func TestTemporaryCleanupTimerRunsDaily(t *testing.T) {
	timer := InternKimBlueclawTemporaryCleanupTimerUnit()

	for _, fragment := range []string{
		"OnUnitActiveSec=1d",
		"Unit=internkim-blueclaw-tmp-clean.service",
		"WantedBy=timers.target",
	} {
		if !strings.Contains(timer, fragment) {
			t.Fatalf("expected cleanup timer to include %q", fragment)
		}
	}
}
