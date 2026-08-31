package admind

import (
	"os"
	"path/filepath"
	"testing"
)

func workspaceHoldingTheFormerCircle(t *testing.T, contents map[string]string) string {
	t.Helper()
	workspace := t.TempDir()
	for path, content := range contents {
		full := filepath.Join(workspace, filepath.FromSlash(path))
		if errorValue := os.MkdirAll(filepath.Dir(full), 0o755); errorValue != nil {
			t.Fatalf("make %s: %v", full, errorValue)
		}
		if errorValue := os.WriteFile(full, []byte(content), 0o644); errorValue != nil {
			t.Fatalf("write %s: %v", full, errorValue)
		}
	}
	return workspace
}

func TestTheFormerStaffCircleBecomesTheMemberCircle(t *testing.T) {
	workspace := workspaceHoldingTheFormerCircle(t, map[string]string{
		"circles/staff/sites/s1/index.html": "a site",
		"circles/staff/inbox/note.md":       "a note",
	})
	service := &Service{Configuration: Configuration{BlueclawWorkspacePath: workspace}}

	service.renameTheFormerStaffCircle()

	if _, errorValue := os.Stat(filepath.Join(workspace, "circles", "staff")); !os.IsNotExist(errorValue) {
		t.Fatal("the old circle is still there")
	}
	for _, path := range []string{"circles/member/sites/s1/index.html", "circles/member/inbox/note.md"} {
		if _, errorValue := os.Stat(filepath.Join(workspace, filepath.FromSlash(path))); errorValue != nil {
			t.Fatalf("%s did not come across: %v", path, errorValue)
		}
	}
}

// A device that has already been through this must not lose what the new
// circle holds, and must not fail on the second boot either.
func TestTheMigrationLeavesWhatTheNewCircleAlreadyHolds(t *testing.T) {
	workspace := workspaceHoldingTheFormerCircle(t, map[string]string{
		"circles/staff/sites/s1/index.html":  "the old copy",
		"circles/staff/inbox/note.md":        "a note",
		"circles/member/sites/s1/index.html": "the one in use",
	})
	service := &Service{Configuration: Configuration{BlueclawWorkspacePath: workspace}}

	service.renameTheFormerStaffCircle()

	held, errorValue := os.ReadFile(filepath.Join(workspace, "circles", "member", "sites", "s1", "index.html"))
	if errorValue != nil || string(held) != "the one in use" {
		t.Fatalf("the site in use was overwritten: %q %v", held, errorValue)
	}
	if _, errorValue := os.Stat(filepath.Join(workspace, "circles", "member", "inbox", "note.md")); errorValue != nil {
		t.Fatalf("the inbox did not come across: %v", errorValue)
	}
}

func TestTheMigrationDoesNothingWhenThereIsNoFormerCircle(t *testing.T) {
	workspace := workspaceHoldingTheFormerCircle(t, map[string]string{
		"circles/member/inbox/note.md": "a note",
	})
	service := &Service{Configuration: Configuration{BlueclawWorkspacePath: workspace}}

	service.renameTheFormerStaffCircle()

	if _, errorValue := os.Stat(filepath.Join(workspace, "circles", "member", "inbox", "note.md")); errorValue != nil {
		t.Fatalf("a workspace that needed nothing lost something: %v", errorValue)
	}
}
