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
		"blueclaw-posix-helper sync",
		"$WORKSPACE_PATH/private/people/$person_id",
		"$person_path/tmp",
		"$person_path/artifacts",
		"chmod 700 \"$person_path\" \"$person_path/tmp\" \"$person_path/artifacts\"",
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
