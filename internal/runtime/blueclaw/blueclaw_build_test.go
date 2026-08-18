package blueclaw

import (
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestABuildRefusesACheckoutThatIsNotOnMainRatherThanRewindingIt(t *testing.T) {
	blueclawDirectory := buildTestSubmodule(t)
	offMainRevision := runBuildTestGit(t, blueclawDirectory, "rev-parse", "HEAD")

	errorValue := requireBlueclawCheckoutIsOnMain(blueclawDirectory)

	if errorValue == nil {
		t.Fatal("a build that moves the checkout discards what the parent pinned, and the pointer bump becomes a backwards deploy")
	}
	if !strings.Contains(errorValue.Error(), offMainRevision[:12]) {
		t.Fatalf("the refusal has to name the revision it found: %v", errorValue)
	}
	if !strings.Contains(errorValue.Error(), "INTERNKIM_BLUECLAW_USE_LOCAL=1") {
		t.Fatalf("someone deliberately building an unmerged blueclaw needs to be told how: %v", errorValue)
	}
	if runBuildTestGit(t, blueclawDirectory, "rev-parse", "HEAD") != offMainRevision {
		t.Fatal("the check must not move the checkout it is checking")
	}
}

func TestACheckoutOnMainIsAccepted(t *testing.T) {
	blueclawDirectory := buildTestSubmodule(t)
	runBuildTestGit(t, blueclawDirectory, "checkout", "--quiet", "--detach", "origin/main")

	if errorValue := requireBlueclawCheckoutIsOnMain(blueclawDirectory); errorValue != nil {
		t.Fatalf("expected a checkout on main to build: %v", errorValue)
	}
}

func buildTestSubmodule(t *testing.T) string {
	t.Helper()
	originDirectory := filepath.Join(t.TempDir(), "origin")
	runBuildTestGit(t, "", "init", "--quiet", "-b", "main", originDirectory)
	runBuildTestGit(t, originDirectory, "commit", "--quiet", "--allow-empty", "-m", "first")
	runBuildTestGit(t, originDirectory, "commit", "--quiet", "--allow-empty", "-m", "second")

	workingDirectory := filepath.Join(t.TempDir(), "blueclaw")
	runBuildTestGit(t, "", "clone", "--quiet", originDirectory, workingDirectory)
	runBuildTestGit(t, workingDirectory, "fetch", "--quiet", "origin", "main")
	runBuildTestGit(t, workingDirectory, "commit", "--quiet", "--allow-empty", "-m", "not merged to main")
	return workingDirectory
}

func runBuildTestGit(t *testing.T, directory string, arguments ...string) string {
	t.Helper()
	command := exec.Command("git", arguments...)
	command.Dir = directory
	command.Env = append(command.Environ(),
		"GIT_AUTHOR_NAME=test", "GIT_AUTHOR_EMAIL=test@example.com",
		"GIT_COMMITTER_NAME=test", "GIT_COMMITTER_EMAIL=test@example.com")
	output, errorValue := command.CombinedOutput()
	if errorValue != nil {
		t.Fatalf("git %v: %s: %v", arguments, strings.TrimSpace(string(output)), errorValue)
	}
	return strings.TrimSpace(string(output))
}
