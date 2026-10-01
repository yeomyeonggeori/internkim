package cli

import (
	"encoding/base64"
	"errors"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/yeomyeonggeori/internkim/internal/runtime/blueclaw"
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

const testReleaseVersion = "2026.10.01.090507"

func releaseDirectoryWithEveryAsset(t *testing.T) string {
	t.Helper()
	directory := t.TempDir()
	names := append(releaseAssetNamesWithoutBottle(), blueclaw.HomebrewBottleFileName(testReleaseVersion, "arm64_ventura"))
	for _, name := range names {
		writeFileForTest(t, filepath.Join(directory, name), name)
	}
	if errorValue := writeReleaseChecksums(directory, testReleaseVersion); errorValue != nil {
		t.Fatal(errorValue)
	}
	return directory
}

func TestATestingReleaseIsAPrereleaseCarryingEveryPackageTheBottleAndTheChecksums(t *testing.T) {
	calls := recordGitHubCommands(t, func([]string) (string, error) { return "", nil })
	directory := releaseDirectoryWithEveryAsset(t)

	if errorValue := createHostRelease(testReleaseVersion, "abc123", testingChannel, directory, io.Discard); errorValue != nil {
		t.Fatal(errorValue)
	}
	tag := "v" + testReleaseVersion
	created := (*calls)[0]
	if !slices.Equal(created[:9], []string{"release", "create", tag, "--repo", hostReleaseRepository, "--target", "abc123", "--title", tag}) {
		t.Fatalf("gh was asked %v", created)
	}
	if !slices.Contains(created, "--prerelease") || slices.Contains(created, "--latest") {
		t.Errorf("a testing release has to be a prerelease and must not become latest: %v", created)
	}
	expected := append(releaseAssetNamesWithoutBottle(), blueclaw.HomebrewBottleFileName(testReleaseVersion, "arm64_ventura"), releaseChecksumsName)
	for _, name := range expected {
		if !slices.Contains(created, filepath.Join(directory, name)) {
			t.Errorf("the release does not carry %s", name)
		}
	}
	if len(*calls) != 1 {
		t.Errorf("a testing release reached past the release itself, and the tap is what every Mac installs: %v", (*calls)[1:])
	}
}

func TestTheChecksumsListTheBottleTheTarballAndTheFormula(t *testing.T) {
	checksums, errorValue := os.ReadFile(filepath.Join(releaseDirectoryWithEveryAsset(t), releaseChecksumsName))
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	for _, name := range []string{
		blueclaw.HomebrewBottleFileName(testReleaseVersion, "arm64_ventura"),
		blueclaw.HomebrewSourceTarballName(),
		blueclaw.HomebrewFormulaAssetName(),
	} {
		if !strings.Contains(string(checksums), "  "+name+"\n") {
			t.Errorf("SHA256SUMS does not list %s:\n%s", name, checksums)
		}
	}
}

func tapAnswers(t *testing.T, formula string, tapCarries string) func([]string) (string, error) {
	return func(arguments []string) (string, error) {
		switch {
		case arguments[0] == "release" && arguments[1] == "download":
			directory := arguments[slices.Index(arguments, "--dir")+1]
			writeFileForTest(t, filepath.Join(directory, blueclaw.HomebrewFormulaAssetName()), formula)
		case arguments[0] == "api" && len(arguments) == 2:
			return `{"sha":"blob-sha","content":"` + base64.StdEncoding.EncodeToString([]byte(tapCarries)) + `"}`, nil
		}
		return "", nil
	}
}

func TestAStableReleaseBecomesLatestAndGivesTheTapTheFormulaItCarries(t *testing.T) {
	calls := recordGitHubCommands(t, tapAnswers(t, "the new formula", "the old formula"))
	if errorValue := createHostRelease(testReleaseVersion, "abc123", stableChannel, releaseDirectoryWithEveryAsset(t), io.Discard); errorValue != nil {
		t.Fatal(errorValue)
	}
	created := (*calls)[0]
	if !slices.Contains(created, "--latest") || slices.Contains(created, "--prerelease") {
		t.Errorf("a stable release is what releases/latest/download answers with: %v", created)
	}
	written := (*calls)[len(*calls)-1]
	expected := []string{
		"api", "--method", "PUT", "repos/yeomyeonggeori/homebrew-tap/contents/Formula/internkim.rb",
		"-f", "message=Carry internkim v" + testReleaseVersion,
		"-f", "content=" + base64.StdEncoding.EncodeToString([]byte("the new formula")),
		"-f", "sha=blob-sha",
	}
	if !slices.Equal(written, expected) {
		t.Errorf("the tap was written with %v", written)
	}
}

func TestATapThatAlreadyCarriesTheFormulaIsNotWritten(t *testing.T) {
	calls := recordGitHubCommands(t, tapAnswers(t, "the formula", "the formula"))
	if errorValue := promoteHostRelease(gitHubRelease{TagName: "v1"}, stableChannel, io.Discard); errorValue != nil {
		t.Fatal(errorValue)
	}
	for _, call := range *calls {
		if slices.Contains(call, "PUT") {
			t.Errorf("the tap was written although it carries the formula: %v", call)
		}
	}
}

func TestAReleaseMissingAPackageOrTheBottleIsNotCreated(t *testing.T) {
	for _, removed := range []string{"internkim-amd64.rpm", blueclaw.HomebrewBottleFileName(testReleaseVersion, "arm64_ventura"), blueclaw.HomebrewFormulaAssetName()} {
		calls := recordGitHubCommands(t, func([]string) (string, error) { return "", nil })
		directory := releaseDirectoryWithEveryAsset(t)
		if errorValue := os.Remove(filepath.Join(directory, removed)); errorValue != nil {
			t.Fatal(errorValue)
		}
		errorValue := createHostRelease(testReleaseVersion, "abc123", stableChannel, directory, io.Discard)
		if errorValue == nil {
			t.Errorf("a release without %s was created", removed)
		}
		if len(*calls) != 0 {
			t.Errorf("gh was asked %v", *calls)
		}
	}
}

func TestStableOverATestedBuildPromotesItAndGivesTheTapItsFormula(t *testing.T) {
	calls := recordGitHubCommands(t, tapAnswers(t, "the tested formula", "the old formula"))
	if errorValue := promoteHostRelease(gitHubRelease{TagName: "v1", IsPrerelease: true}, stableChannel, io.Discard); errorValue != nil {
		t.Fatal(errorValue)
	}
	if !slices.Equal((*calls)[0], []string{"release", "edit", "v1", "--repo", hostReleaseRepository, "--prerelease=false", "--latest"}) {
		t.Errorf("gh was asked %v", *calls)
	}
	if !slices.Equal((*calls)[1][:3], []string{"release", "download", "v1"}) {
		t.Errorf("the tap was not given the formula the tested release carries: %v", *calls)
	}
}

func TestATestingCommitAlreadyReleasedIsRefused(t *testing.T) {
	calls := recordGitHubCommands(t, func([]string) (string, error) { return "", nil })
	for _, release := range []gitHubRelease{{TagName: "v1", IsPrerelease: true}, {TagName: "v1"}} {
		if promoteHostRelease(release, testingChannel, io.Discard) == nil {
			t.Errorf("%+v was released again on testing", release)
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
