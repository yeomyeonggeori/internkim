package cli

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func gitInDirectory(t *testing.T, directory string, arguments ...string) string {
	t.Helper()
	identity := []string{"-c", "user.name=Sample", "-c", "user.email=sample@example.com", "-c", "protocol.file.allow=always"}
	output, errorValue := exec.Command("git", append(append([]string{"-C", directory}, identity...), arguments...)...).CombinedOutput()
	if errorValue != nil {
		t.Fatalf("git %v in %s: %s", arguments, directory, output)
	}
	return strings.TrimSpace(string(output))
}

func commitFile(t *testing.T, directory string, relativePath string, content string) {
	t.Helper()
	writeFileForTest(t, filepath.Join(directory, relativePath), content)
	gitInDirectory(t, directory, "add", relativePath)
	gitInDirectory(t, directory, "commit", "-q", "-m", "change "+relativePath)
}

func cloneWithOrigin(t *testing.T, destination string) (string, string) {
	t.Helper()
	origin := filepath.Join(t.TempDir(), "origin.git")
	gitInDirectory(t, t.TempDir(), "init", "-q", "--bare", "-b", "main", origin)
	gitInDirectory(t, filepath.Dir(destination), "clone", "-q", origin, destination)
	gitInDirectory(t, destination, "checkout", "-q", "-b", "main")
	commitFile(t, destination, "first.txt", "first")
	gitInDirectory(t, destination, "push", "-q", "origin", "main")
	return destination, origin
}

func pushFromAnotherClone(t *testing.T, origin string) {
	t.Helper()
	other := filepath.Join(t.TempDir(), "other")
	gitInDirectory(t, filepath.Dir(other), "clone", "-q", origin, other)
	commitFile(t, other, "moved-on.txt", "moved on")
	gitInDirectory(t, other, "push", "-q", "origin", "main")
}

type shippableTree struct {
	root           string
	rootOrigin     string
	blueclaw       string
	blueclawOrigin string
}

func newShippableTree(t *testing.T) shippableTree {
	t.Helper()
	parent := t.TempDir()
	root, rootOrigin := cloneWithOrigin(t, filepath.Join(parent, "root"))
	blueclawPath := filepath.Join(root, ".dependency", "blueclaw")
	if errorValue := os.MkdirAll(filepath.Dir(blueclawPath), 0o755); errorValue != nil {
		t.Fatal(errorValue)
	}
	_, blueclawOrigin := cloneWithOrigin(t, blueclawPath)
	commitFile(t, root, "web/page.txt", "page")
	gitInDirectory(t, root, "add", ".dependency/blueclaw")
	gitInDirectory(t, root, "commit", "-q", "-m", "record blueclaw")
	gitInDirectory(t, root, "push", "-q", "origin", "main")
	return shippableTree{root: root, rootOrigin: rootOrigin, blueclaw: blueclawPath, blueclawOrigin: blueclawOrigin}
}

func reasonsFor(t *testing.T, tree shippableTree, componentNames ...string) string {
	t.Helper()
	reasons, errorValue := unshippableTreeReasons(tree.root, componentNames)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	return strings.Join(reasons, "\n")
}

func TestACleanTreeOnOriginMainShips(t *testing.T) {
	tree := newShippableTree(t)
	if reasons := reasonsFor(t, tree, "web", "chatd", "fonts"); reasons != "" {
		t.Fatalf("a clean tree was refused: %s", reasons)
	}
}

func TestUncommittedEditToASourceAComponentBuildsFromIsRefused(t *testing.T) {
	tree := newShippableTree(t)
	writeFileForTest(t, filepath.Join(tree.root, "web", "page.txt"), "edited")
	if reasons := reasonsFor(t, tree, "web"); !strings.Contains(reasons, "web/page.txt") {
		t.Fatalf("an edited web source was not named: %q", reasons)
	}
}

func TestUncommittedEditInsideBlueclawIsRefusedForTheComponentsBuiltFromIt(t *testing.T) {
	tree := newShippableTree(t)
	writeFileForTest(t, filepath.Join(tree.blueclaw, "first.txt"), "edited")
	if reasons := reasonsFor(t, tree, "chatd"); !strings.Contains(reasons, "inside .dependency/blueclaw") || !strings.Contains(reasons, "first.txt") {
		t.Fatalf("an edit inside blueclaw was not named: %q", reasons)
	}
	if reasons := reasonsFor(t, tree, "fonts"); reasons != "" {
		t.Fatalf("a component that does not build from blueclaw was refused for its dirt: %s", reasons)
	}
}

func TestEditsNothingShippedBuildsFromDoNotRefuse(t *testing.T) {
	tree := newShippableTree(t)
	writeFileForTest(t, filepath.Join(tree.root, "notes.txt"), "untracked")
	commitFile(t, tree.root, "docs/page.txt", "docs")
	gitInDirectory(t, tree.root, "push", "-q", "origin", "main")
	writeFileForTest(t, filepath.Join(tree.root, "docs", "page.txt"), "edited")
	if reasons := reasonsFor(t, tree, "web"); reasons != "" {
		t.Fatalf("an unrelated edit refused the deploy: %s", reasons)
	}
}

func TestATreeThatLacksOriginMainIsRefused(t *testing.T) {
	tree := newShippableTree(t)
	pushFromAnotherClone(t, tree.rootOrigin)
	if reasons := reasonsFor(t, tree, "fonts"); !strings.Contains(reasons, "does not contain origin/main") {
		t.Fatalf("a tree behind origin/main was not refused: %q", reasons)
	}
}

func TestABlueclawThatLacksItsOriginMainIsRefused(t *testing.T) {
	tree := newShippableTree(t)
	pushFromAnotherClone(t, tree.blueclawOrigin)
	if reasons := reasonsFor(t, tree, "fonts"); !strings.Contains(reasons, "blueclaw's HEAD does not contain blueclaw's origin/main") {
		t.Fatalf("a blueclaw behind its origin/main was not refused: %q", reasons)
	}
}

func TestABlueclawCheckedOutAwayFromTheRecordedPointerIsRefused(t *testing.T) {
	tree := newShippableTree(t)
	commitFile(t, tree.blueclaw, "local.txt", "local")
	reasons := reasonsFor(t, tree, "fonts")
	if !strings.Contains(reasons, "this tree records") || !strings.Contains(reasons, "submodule update") {
		t.Fatalf("a moved blueclaw checkout was not refused: %q", reasons)
	}
}

func TestATreeWhoseOriginCannotBeFetchedIsNotGuessedAbout(t *testing.T) {
	tree := newShippableTree(t)
	gitInDirectory(t, tree.root, "remote", "set-url", "origin", filepath.Join(t.TempDir(), "gone.git"))
	if _, errorValue := unshippableTreeReasons(tree.root, []string{"fonts"}); errorValue == nil || !strings.Contains(errorValue.Error(), "git fetch origin failed") {
		t.Fatalf("an unreachable origin did not stop the check: %v", errorValue)
	}
}

func TestDeployAndItsPlanBothStopAtTheTreeCheckBeforeBuilding(t *testing.T) {
	stubDeployEffects(t)
	checkDeployTree = func(string, []string) error { return os.ErrPermission }
	prepareReleaseArtifacts = func(string) error {
		t.Fatal("a tree that must not ship was built")
		return nil
	}
	restorePublish := publishDeployRelease
	publishDeployRelease = func(string, string, string, map[string]bool, int) error {
		t.Fatal("a tree that must not ship was published")
		return nil
	}
	t.Cleanup(func() { publishDeployRelease = restorePublish })
	for _, arguments := range [][]string{{"--sim"}, {"--sim", "--plan"}} {
		if errorValue := runReleaseDeploy(arguments); errorValue != os.ErrPermission {
			t.Fatalf("runReleaseDeploy(%v) = %v, want the tree refusal", arguments, errorValue)
		}
	}
}

func TestTheTreeCheckCoversOnlyTheNarrowedComponents(t *testing.T) {
	stubDeployEffects(t)
	var checked []string
	checkDeployTree = func(_ string, componentNames []string) error {
		checked = componentNames
		return os.ErrPermission
	}
	runReleaseDeploy([]string{"--sim", "--components", "admind,web"})
	if strings.Join(checked, ",") != "admind,web" {
		t.Fatalf("the tree check covered %v", checked)
	}
}
