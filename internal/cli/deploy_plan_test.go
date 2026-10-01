package cli

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yeomyeonggeori/internkim/internal/releaseset"
	"github.com/yeomyeonggeori/internkim/internal/runtime/blueclaw"
)

func deviceHolding(revisions map[string]string) map[string]releaseset.Component {
	components := map[string]releaseset.Component{}
	for name, revision := range revisions {
		components[name] = releaseset.Component{Revision: revision}
	}
	return components
}

func treeBuilding(revisions map[string]string) func(string) string {
	return func(name string) string { return revisions[name] }
}

func neverAhead(string, string) bool { return false }

func everythingKnown(string) bool { return true }

func selectWith(held map[string]string, tree map[string]string, isKnown func(string) bool, isAncestor func(string, string) bool) (map[string]bool, []string) {
	return selectDeployComponents(deviceHolding(held), treeBuilding(tree), isKnown, isAncestor)
}

func TestSelectionTakesWhatDiffers(t *testing.T) {
	selected, refusals := selectWith(
		map[string]string{"skills": "old", "admind": "same"},
		map[string]string{"skills": "new", "admind": "same"},
		everythingKnown, neverAhead,
	)
	if len(refusals) != 0 || len(selected) != 1 || !selected["skills"] {
		t.Fatalf("selected %v refused %v, want only skills", selected, refusals)
	}
}

func TestSelectionAddsTheProtocolPartner(t *testing.T) {
	selected, _ := selectWith(
		map[string]string{"capabilityd": "old", "blueclawPayload": "same"},
		map[string]string{"capabilityd": "new", "blueclawPayload": "same"},
		everythingKnown, neverAhead,
	)
	if !selected["capabilityd"] || !selected["blueclawPayload"] {
		t.Fatalf("selected %v, want capabilityd with blueclawPayload", selected)
	}
}

func TestSelectionRefusesAComponentTheDeviceIsAheadOn(t *testing.T) {
	_, refusals := selectWith(
		map[string]string{"relay": "device", "skills": "old"},
		map[string]string{"relay": "tree", "skills": "new"},
		everythingKnown,
		func(older string, newer string) bool { return older == "tree" && newer == "device" },
	)
	joined := strings.Join(refusals, "\n")
	if !strings.Contains(joined, "relay") || strings.Contains(joined, "skills") {
		t.Fatalf("refusals %q, want exactly relay", joined)
	}
}

func TestSelectionRefusesADeviceCommitThisCheckoutDoesNotKnow(t *testing.T) {
	unknownCommit := strings.Repeat("a", 40)
	selected, refusals := selectWith(
		map[string]string{"relay": unknownCommit, "skills": "old"},
		map[string]string{"relay": "tree", "skills": "new"},
		func(revision string) bool { return revision != unknownCommit },
		neverAhead,
	)
	if len(refusals) != 1 || !strings.Contains(refusals[0], "relay") || !strings.Contains(refusals[0], "does not know") {
		t.Fatalf("refusals %q, want relay named as unknown", refusals)
	}
	if selected["relay"] {
		t.Fatal("a component the device holds at an unknown commit was shipped over")
	}
}

func TestSelectionShipsOverARevisionThatIsNotACommit(t *testing.T) {
	selected, refusals := selectWith(
		map[string]string{"web": "1790637690492"},
		map[string]string{"web": strings.Repeat("b", 40)},
		func(string) bool { return false }, neverAhead,
	)
	if len(refusals) != 0 || !selected["web"] {
		t.Fatalf("selected %v refused %v, want web shipped", selected, refusals)
	}
}

func TestSelectionIsEmptyWhenTheDeviceMatches(t *testing.T) {
	revisions := map[string]string{"skills": "same", "web": "same"}
	selected, refusals := selectWith(revisions, revisions, everythingKnown, neverAhead)
	if len(refusals) != 0 || selected == nil || len(selected) != 0 {
		t.Fatalf("selected %v refused %v, want an empty set", selected, refusals)
	}
}

func TestNarrowedSetIsKeptAndTheProtocolGuardStillRefuses(t *testing.T) {
	stubDeployEffects(t)
	plan, errorValue := chooseDeployComponents("", map[string]bool{"capabilityd": true},
		func() (map[string]releaseset.Component, error) { return nil, nil }, everythingKnown, neverAhead)
	if errorValue != nil || len(plan.selected) != 1 || !plan.selected["capabilityd"] {
		t.Fatalf("narrowed set changed: %v, %v", plan.selected, errorValue)
	}
	errorValue = refuseToSplitTheProtocol(
		map[string]releaseset.Component{"capabilityd": {}},
		map[string]releaseset.Component{"capabilityd": {}, "blueclawPayload": {}},
	)
	if errorValue == nil {
		t.Fatal("a narrowed set that splits the protocol was allowed")
	}
}

func stubDeployEffects(t *testing.T) {
	t.Helper()
	restorePrepare, restoreFetch, restoreCheck := prepareReleaseArtifacts, fetchReleaseHistory, checkDeployTree
	prepareReleaseArtifacts = func(string) error { return nil }
	fetchReleaseHistory = func(string) {}
	checkDeployTree = func(string, []string) error { return nil }
	t.Cleanup(func() {
		prepareReleaseArtifacts, fetchReleaseHistory, checkDeployTree = restorePrepare, restoreFetch, restoreCheck
	})
}

func writePayloadManifest(t *testing.T, repositoryRootPath string, revision string) {
	t.Helper()
	directoryPath := filepath.Join(repositoryRootPath, blueclaw.BlueclawPayloadArtifactPath)
	if errorValue := os.MkdirAll(directoryPath, 0o755); errorValue != nil {
		t.Fatal(errorValue)
	}
	document := `{"blueclawRevision":"` + revision + `"}`
	if errorValue := os.WriteFile(filepath.Join(directoryPath, "manifest.json"), []byte(document), 0o644); errorValue != nil {
		t.Fatal(errorValue)
	}
}

func TestStalePayloadArtifactIsRebuiltBeforeItsRevisionIsRead(t *testing.T) {
	repositoryRootPath := t.TempDir()
	writePayloadManifest(t, repositoryRootPath, "three-days-old")
	treeRevision := gitRevision(repositoryRootPath)
	held := map[string]releaseset.Component{}
	for _, name := range ReleaseComponentNames() {
		held[name] = releaseset.Component{Revision: releaseComponentRevision(name, repositoryRootPath, treeRevision)}
	}
	held["blueclawPayload"] = releaseset.Component{Revision: "current"}
	stubDeployEffects(t)
	prepareReleaseArtifacts = func(rootPath string) error {
		writePayloadManifest(t, rootPath, "current")
		return nil
	}
	plan, errorValue := chooseDeployComponents(
		repositoryRootPath, nil,
		func() (map[string]releaseset.Component, error) { return held, nil },
		everythingKnown, neverAhead,
	)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if plan.selected["blueclawPayload"] {
		t.Fatal("the payload was compared against the stale artifact, not a rebuilt one")
	}
}

func TestPlanNeverPublishes(t *testing.T) {
	repositoryRootPath, errorValue := resolveRepositoryRootPath()
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	stubDeployEffects(t)
	treeRevision := gitRevision(repositoryRootPath)
	held := map[string]releaseset.Component{}
	for _, name := range ReleaseComponentNames() {
		held[name] = releaseset.Component{Revision: releaseComponentRevision(name, repositoryRootPath, treeRevision)}
	}
	held["skills"] = releaseset.Component{Revision: "old"}
	restoreRead, restorePublish := readDeviceReleaseComponents, publishDeployRelease
	published := false
	readDeviceReleaseComponents = func(commandTarget) (map[string]releaseset.Component, error) { return held, nil }
	publishDeployRelease = func(string, string, string, map[string]bool, int) error {
		published = true
		return nil
	}
	t.Cleanup(func() { readDeviceReleaseComponents, publishDeployRelease = restoreRead, restorePublish })
	if errorValue := runReleaseDeploy([]string{"--plan", "--sim"}); errorValue != nil {
		t.Fatal(errorValue)
	}
	if published {
		t.Fatal("--plan published a release")
	}
}

func TestBoardUIRevisionFollowsTheSourcesItReads(t *testing.T) {
	repositoryRootPath := t.TempDir()
	for _, command := range [][]string{{"init", "-q"}, {"-c", "user.name=t", "-c", "user.email=t@example.com", "commit", "-q", "--allow-empty", "-m", "first"}} {
		if output, errorValue := exec.Command("git", append([]string{"-C", repositoryRootPath}, command...)...).CombinedOutput(); errorValue != nil {
			t.Fatalf("git %v: %s", command, output)
		}
	}
	writeFileForTest(t, filepath.Join(repositoryRootPath, "web", "src", "first.ts"), "export {}")
	runGitForTest(t, repositoryRootPath, "add", ".")
	runGitForTest(t, repositoryRootPath, "-c", "user.name=t", "-c", "user.email=t@example.com", "commit", "-q", "-m", "first web")
	before := releaseComponentRevision("web", repositoryRootPath, gitRevision(repositoryRootPath))
	writeFileForTest(t, filepath.Join(repositoryRootPath, "docs", "page.mdx"), "unrelated")
	runGitForTest(t, repositoryRootPath, "add", ".")
	runGitForTest(t, repositoryRootPath, "-c", "user.name=t", "-c", "user.email=t@example.com", "commit", "-q", "-m", "docs")
	if after := releaseComponentRevision("web", repositoryRootPath, gitRevision(repositoryRootPath)); after != before {
		t.Fatalf("a docs commit moved the web revision: %s -> %s", before, after)
	}
	writeFileForTest(t, filepath.Join(repositoryRootPath, "web", "src", "page.ts"), "export {}")
	runGitForTest(t, repositoryRootPath, "add", ".")
	runGitForTest(t, repositoryRootPath, "-c", "user.name=t", "-c", "user.email=t@example.com", "commit", "-q", "-m", "web")
	if after := releaseComponentRevision("web", repositoryRootPath, gitRevision(repositoryRootPath)); after == before {
		t.Fatal("a commit to web/ did not move the web revision")
	}
}

func writeFileForTest(t *testing.T, path string, content string) {
	t.Helper()
	if errorValue := os.MkdirAll(filepath.Dir(path), 0o755); errorValue != nil {
		t.Fatal(errorValue)
	}
	if errorValue := os.WriteFile(path, []byte(content), 0o644); errorValue != nil {
		t.Fatal(errorValue)
	}
}

func runGitForTest(t *testing.T, repositoryRootPath string, arguments ...string) {
	t.Helper()
	if output, errorValue := exec.Command("git", append([]string{"-C", repositoryRootPath}, arguments...)...).CombinedOutput(); errorValue != nil {
		t.Fatalf("git %v: %s", arguments, output)
	}
}

func TestVerificationFailsWhenTheDeviceHoldsAnotherRevision(t *testing.T) {
	deployed := map[string]releaseComponentBrief{"skills": {Revision: "new"}, "web": {Revision: "old"}}
	errorValue := verifyDeployedRevisions(
		[]string{"skills", "web"},
		treeBuilding(map[string]string{"skills": "new", "web": "new"}),
		deployed,
	)
	if errorValue == nil || !strings.Contains(errorValue.Error(), "web") || strings.Contains(errorValue.Error(), "skills") {
		t.Fatalf("verification did not name exactly web: %v", errorValue)
	}
}

func TestSupervisorRevisionFollowsTheBlueclawItIsBuiltFrom(t *testing.T) {
	repositoryRootPath, errorValue := resolveRepositoryRootPath()
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	blueclawRevision := strings.TrimSpace(runCmd("git", "-C", filepath.Join(repositoryRootPath, blueclaw.BlueclawSubmodulePath), "rev-parse", "HEAD"))
	if blueclawRevision == "" {
		t.Skip("the blueclaw submodule is not checked out")
	}
	if revision := releaseComponentRevision("blueclawSupervisor", repositoryRootPath, "repository-head"); revision != blueclawRevision {
		t.Fatalf("the supervisor is built from blueclaw %s but its revision is %s, so every commit here ships it", blueclawRevision, revision)
	}
}
