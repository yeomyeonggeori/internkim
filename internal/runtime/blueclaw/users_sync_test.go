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
	} {
		if !strings.Contains(script, fragment) {
			t.Fatalf("expected users sync script to include %q", fragment)
		}
	}
}

func TestUsersSyncScriptRefreshesPOSIXWorkspaceAfterPolicyChanges(t *testing.T) {
	script := InternKimUsersSyncScript()

	for _, fragment := range []string{
		`{personID:$personID,email:$email,circles:`,
		"refresh_current_policy()",
		"curl -fsS \"$BLUECLAW_URL/admin/api/policy\" > \"$current_policy_path\"",
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
