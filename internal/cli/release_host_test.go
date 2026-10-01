package cli

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func recordGitHubCommands(t *testing.T, answer func(arguments []string) (string, error)) *[][]string {
	t.Helper()
	calls := [][]string{}
	original := runGitHubCommand
	runGitHubCommand = func(arguments ...string) (string, error) {
		calls = append(calls, arguments)
		return answer(arguments)
	}
	t.Cleanup(func() { runGitHubCommand = original })
	return &calls
}

func releaseDirectoryWithEveryAsset(t *testing.T) string {
	t.Helper()
	directory := t.TempDir()
	for _, name := range hostReleaseAssetNames() {
		if errorValue := os.WriteFile(filepath.Join(directory, name), []byte(name), 0o644); errorValue != nil {
			t.Fatal(errorValue)
		}
	}
	if errorValue := writeReleaseChecksums(directory); errorValue != nil {
		t.Fatal(errorValue)
	}
	return directory
}

func TestATestingReleaseIsAPrereleaseCarryingEveryPackageAndTheChecksums(t *testing.T) {
	calls := recordGitHubCommands(t, func([]string) (string, error) { return "", nil })
	directory := releaseDirectoryWithEveryAsset(t)

	if errorValue := createHostRelease("v2026.10.01.090507", "abc123", testingChannel, directory, io.Discard); errorValue != nil {
		t.Fatal(errorValue)
	}
	created := (*calls)[0]
	if !slices.Equal(created[:9], []string{"release", "create", "v2026.10.01.090507", "--repo", hostReleaseRepository, "--target", "abc123", "--title", "v2026.10.01.090507"}) {
		t.Fatalf("gh was asked %v", created)
	}
	if !slices.Contains(created, "--prerelease") || slices.Contains(created, "--latest") {
		t.Errorf("a testing release has to be a prerelease and must not become latest: %v", created)
	}
	for _, name := range append(hostReleaseAssetNames(), releaseChecksumsName) {
		if !slices.Contains(created, filepath.Join(directory, name)) {
			t.Errorf("the release does not carry %s", name)
		}
	}
}

func TestAStableReleaseBecomesLatest(t *testing.T) {
	calls := recordGitHubCommands(t, func([]string) (string, error) { return "", nil })
	if errorValue := createHostRelease("v1", "abc123", stableChannel, releaseDirectoryWithEveryAsset(t), io.Discard); errorValue != nil {
		t.Fatal(errorValue)
	}
	created := (*calls)[0]
	if !slices.Contains(created, "--latest") || slices.Contains(created, "--prerelease") {
		t.Errorf("a stable release is what releases/latest/download answers with: %v", created)
	}
}

func TestAReleaseMissingAPackageIsNotCreated(t *testing.T) {
	calls := recordGitHubCommands(t, func([]string) (string, error) { return "", nil })
	directory := releaseDirectoryWithEveryAsset(t)
	if errorValue := os.Remove(filepath.Join(directory, "internkim-amd64.rpm")); errorValue != nil {
		t.Fatal(errorValue)
	}
	errorValue := createHostRelease("v1", "abc123", stableChannel, directory, io.Discard)
	if errorValue == nil || !strings.Contains(errorValue.Error(), "internkim-amd64.rpm") {
		t.Fatalf("a release without the amd64 rpm was created: %v", errorValue)
	}
	if len(*calls) != 0 {
		t.Errorf("gh was asked %v", *calls)
	}
}

func TestStableOverATestedBuildPromotesItInsteadOfRebuilding(t *testing.T) {
	calls := recordGitHubCommands(t, func([]string) (string, error) { return "", nil })
	if errorValue := promoteHostRelease(gitHubRelease{TagName: "v1", IsPrerelease: true}, stableChannel, io.Discard); errorValue != nil {
		t.Fatal(errorValue)
	}
	if !slices.Equal((*calls)[0], []string{"release", "edit", "v1", "--repo", hostReleaseRepository, "--prerelease=false", "--latest"}) {
		t.Errorf("gh was asked %v", *calls)
	}
}

func TestACommitAlreadyReleasedOnItsChannelIsRefused(t *testing.T) {
	calls := recordGitHubCommands(t, func([]string) (string, error) { return "", nil })
	for _, attempt := range []struct {
		release gitHubRelease
		channel string
	}{
		{gitHubRelease{TagName: "v1", IsPrerelease: true}, testingChannel},
		{gitHubRelease{TagName: "v1"}, stableChannel},
		{gitHubRelease{TagName: "v1"}, testingChannel},
	} {
		if promoteHostRelease(attempt.release, attempt.channel, io.Discard) == nil {
			t.Errorf("%+v was released again on %s", attempt.release, attempt.channel)
		}
	}
	if len(*calls) != 0 {
		t.Errorf("gh was asked %v", *calls)
	}
}

func TestOnlyAMissingReleaseCountsAsUnpublished(t *testing.T) {
	recordGitHubCommands(t, func([]string) (string, error) { return "release not found\n", errors.New("exit status 1") })
	if _, isPublished, errorValue := publishedHostRelease("v1"); errorValue != nil || isPublished {
		t.Fatalf("a missing release read as published=%v, %v", isPublished, errorValue)
	}

	recordGitHubCommands(t, func([]string) (string, error) {
		return "To get started with GitHub CLI, please run:  gh auth login\n", errors.New("exit status 4")
	})
	if _, _, errorValue := publishedHostRelease("v1"); errorValue == nil {
		t.Fatal("a signed-out gh read as no release, and the build would run for minutes before the upload failed")
	}

	recordGitHubCommands(t, func([]string) (string, error) { return `{"tagName":"v1","isPrerelease":true}`, nil })
	release, isPublished, errorValue := publishedHostRelease("v1")
	if errorValue != nil || !isPublished || !release.IsPrerelease {
		t.Fatalf("a published prerelease read as %+v, %v, %v", release, isPublished, errorValue)
	}
}

func TestReleaseHostNamesItsChannelsAndRefusesAnyOther(t *testing.T) {
	original := checkReleaseTree
	checkReleaseTree = func(string) error { t.Fatal("the tree was checked before the channel"); return nil }
	t.Cleanup(func() { checkReleaseTree = original })
	for _, arguments := range [][]string{{}, {"--channel", "beta"}} {
		errorValue := runReleaseHost(arguments)
		if errorValue == nil || !strings.Contains(errorValue.Error(), "stable or testing") {
			t.Errorf("%v: %v", arguments, errorValue)
		}
	}
}

func releaseRefusal(t *testing.T, tree shippableTree) string {
	t.Helper()
	errorValue := refuseUnreleasableTree(tree.root)
	if errorValue == nil {
		return ""
	}
	return errorValue.Error()
}

func TestACleanCommitOnOriginMainIsReleasable(t *testing.T) {
	if refusal := releaseRefusal(t, newShippableTree(t)); refusal != "" {
		t.Fatalf("a clean tree at origin/main was refused: %s", refusal)
	}
}

func TestADirtyTreeIsNotReleased(t *testing.T) {
	tree := newShippableTree(t)
	writeFileForTest(t, filepath.Join(tree.root, "web", "page.txt"), "edited")
	if refusal := releaseRefusal(t, tree); !strings.Contains(refusal, "web/page.txt") {
		t.Fatalf("an edited tree was not refused by name: %q", refusal)
	}
}

func TestACommitMainDoesNotHaveIsNotReleased(t *testing.T) {
	tree := newShippableTree(t)
	commitFile(t, tree.root, "branch.txt", "only on a branch")
	if refusal := releaseRefusal(t, tree); !strings.Contains(refusal, "HEAD is not on origin/main") {
		t.Fatalf("a commit ahead of origin/main was not refused: %q", refusal)
	}
}

func TestACommitBehindOriginMainIsNotReleased(t *testing.T) {
	tree := newShippableTree(t)
	pushFromAnotherClone(t, tree.rootOrigin)
	if refusal := releaseRefusal(t, tree); !strings.Contains(refusal, "does not contain origin/main") {
		t.Fatalf("a commit behind origin/main was released, and its version would be older than main's: %q", refusal)
	}
}
