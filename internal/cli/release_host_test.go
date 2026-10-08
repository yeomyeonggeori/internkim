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
	"time"

	"github.com/yeomyeonggeori/internkim/internal/hostversion"
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

func commandsStarting(calls [][]string, name string, verb string) [][]string {
	matching := [][]string{}
	for _, call := range calls {
		if call[0] == name && call[1] == verb {
			matching = append(matching, call)
		}
	}
	return matching
}

func withoutAssetRetryWaits(t *testing.T) {
	t.Helper()
	original := waitBeforeAssetRetry
	waitBeforeAssetRetry = func(time.Duration) {}
	t.Cleanup(func() { waitBeforeAssetRetry = original })
}

func TestATestingReleaseIsADraftFilledAssetByAssetThenPublishedAsAPrerelease(t *testing.T) {
	calls := recordGitHubCommands(t, func([]string) (string, error) { return "", nil })
	directory := releaseDirectoryWithEveryAsset(t)

	if errorValue := createHostRelease(testReleaseVersion, "abc123", testingChannel, directory, io.Discard); errorValue != nil {
		t.Fatal(errorValue)
	}
	tag := "v" + testReleaseVersion
	created := (*calls)[0]
	if !slices.Equal(created[:9], []string{"release", "create", tag, "--repo", hostReleaseRepository, "--draft", "--target", "abc123", "--title"}) {
		t.Fatalf("gh was asked %v", created)
	}
	expected := append(releaseAssetNamesWithoutBottle(), blueclaw.HomebrewBottleFileName(testReleaseVersion, "arm64_ventura"), releaseChecksumsName)
	for _, argument := range created {
		if strings.HasPrefix(argument, directory) {
			t.Errorf("the draft was created carrying %s, so one stalled upload would roll the whole release back", argument)
		}
	}
	uploads := commandsStarting(*calls, "release", "upload")
	if len(uploads) != len(expected) {
		t.Fatalf("%d uploads for %d assets: %v", len(uploads), len(expected), uploads)
	}
	for _, name := range expected {
		path := filepath.Join(directory, name)
		if !slices.ContainsFunc(uploads, func(call []string) bool { return call[3] == path && slices.Contains(call, "--clobber") }) {
			t.Errorf("%s was not uploaded with --clobber", name)
		}
	}
	published := (*calls)[len(*calls)-1]
	if !slices.Equal(published, []string{"release", "edit", tag, "--repo", hostReleaseRepository, "--draft=false", "--prerelease"}) {
		t.Errorf("a testing release has to be published as a prerelease and must not become latest: %v", published)
	}
	if len(*calls) != len(expected)+2 {
		t.Errorf("a testing release reached past the release itself, and the tap is what every Mac installs: %v", *calls)
	}
}

func TestAnAssetThatTimesOutOnceIsUploadedAgainAndTheReleaseIsPublished(t *testing.T) {
	withoutAssetRetryWaits(t)
	stalled := false
	calls := recordGitHubCommands(t, func(arguments []string) (string, error) {
		if arguments[1] == "upload" && !stalled && strings.HasSuffix(arguments[3], ".deb") {
			stalled = true
			return "", errors.New("gh release upload: exit status 1: HTTP 408: Upload body timed out due to inactivity")
		}
		return "", nil
	})
	if errorValue := createHostRelease(testReleaseVersion, "abc123", testingChannel, releaseDirectoryWithEveryAsset(t), io.Discard); errorValue != nil {
		t.Fatal(errorValue)
	}
	uploads := commandsStarting(*calls, "release", "upload")
	if !slices.Equal(uploads[0], uploads[1]) {
		t.Errorf("the stalled asset was not tried again: %v %v", uploads[0], uploads[1])
	}
	if published := (*calls)[len(*calls)-1]; !slices.Contains(published, "--draft=false") {
		t.Errorf("the release was not published: %v", published)
	}
}

func TestAnAssetThatKeepsFailingLeavesTheDraftAndNamesTheAsset(t *testing.T) {
	withoutAssetRetryWaits(t)
	calls := recordGitHubCommands(t, func(arguments []string) (string, error) {
		if arguments[1] == "upload" && strings.HasSuffix(arguments[3], "internkim-arm64.deb") {
			return "", errors.New("gh release upload: HTTP 408: Upload body timed out due to inactivity")
		}
		return "", nil
	})
	errorValue := createHostRelease(testReleaseVersion, "abc123", testingChannel, releaseDirectoryWithEveryAsset(t), io.Discard)
	if errorValue == nil || !strings.Contains(errorValue.Error(), "internkim-arm64.deb") || !strings.Contains(errorValue.Error(), "draft") {
		t.Fatalf("the error does not name the asset and the draft: %v", errorValue)
	}
	for _, call := range *calls {
		if call[1] == "edit" || call[1] == "delete" {
			t.Errorf("the draft was changed after a failed upload: %v", call)
		}
	}
	failing := 0
	for _, call := range commandsStarting(*calls, "release", "upload") {
		if strings.HasSuffix(call[3], "internkim-arm64.deb") {
			failing++
		}
	}
	if failing != assetUploadAttempts {
		t.Errorf("the asset was tried %d times, not %d", failing, assetUploadAttempts)
	}
}

func TestAnUploadRefusedWithAClientErrorIsNotRetried(t *testing.T) {
	withoutAssetRetryWaits(t)
	calls := recordGitHubCommands(t, func(arguments []string) (string, error) {
		if arguments[1] == "upload" {
			return "", errors.New("gh release upload: HTTP 422: Validation Failed")
		}
		return "", nil
	})
	if errorValue := createHostRelease(testReleaseVersion, "abc123", testingChannel, releaseDirectoryWithEveryAsset(t), io.Discard); errorValue == nil {
		t.Fatal("a refused upload was reported as released")
	}
	if uploads := commandsStarting(*calls, "release", "upload"); len(uploads) != 1 {
		t.Errorf("a 422 was retried: %v", uploads)
	}
}

func TestADraftLeftByAnEarlierRunIsContinued(t *testing.T) {
	calls := recordGitHubCommands(t, func(arguments []string) (string, error) {
		if arguments[1] == "create" {
			return "a release with the same tag name already exists", errors.New("gh release create: exit status 1: a release with the same tag name already exists")
		}
		return "", nil
	})
	if errorValue := createHostRelease(testReleaseVersion, "abc123", testingChannel, releaseDirectoryWithEveryAsset(t), io.Discard); errorValue != nil {
		t.Fatal(errorValue)
	}
	if len(commandsStarting(*calls, "release", "upload")) == 0 {
		t.Error("nothing was uploaded to the existing draft")
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
	published := commandsStarting(*calls, "release", "edit")[0]
	if !slices.Contains(published, "--draft=false") || !slices.Contains(published, "--latest") || slices.Contains(published, "--prerelease") {
		t.Errorf("a stable release is what releases/latest/download answers with: %v", published)
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

func TestANamedTestedReleaseIsPromotedWithoutLookingAtTheTree(t *testing.T) {
	original := checkReleaseTree
	checkReleaseTree = func(string) error { t.Fatal("promoting a published release checked the tree"); return nil }
	t.Cleanup(func() { checkReleaseTree = original })
	tap := tapAnswers(t, "the tested formula", "the old formula")
	calls := recordGitHubCommands(t, func(arguments []string) (string, error) {
		if arguments[0] == "release" && arguments[1] == "view" {
			return `{"tagName":"v0.0.1","isPrerelease":true}`, nil
		}
		return tap(arguments)
	})
	if errorValue := runReleaseHost([]string{"--channel", "stable", "--version", "v0.0.1"}); errorValue != nil {
		t.Fatal(errorValue)
	}
	if !slices.Equal((*calls)[1], []string{"release", "edit", "v0.0.1", "--repo", hostReleaseRepository, "--prerelease=false", "--latest"}) {
		t.Errorf("the named release was not made stable: %v", *calls)
	}
}

func TestANamedReleaseIsOnlyPromotedToStable(t *testing.T) {
	calls := recordGitHubCommands(t, func([]string) (string, error) { return "", nil })
	if runReleaseHost([]string{"--channel", "testing", "--version", "v0.0.1"}) == nil {
		t.Error("a named release was accepted for testing")
	}
	if len(*calls) != 0 {
		t.Errorf("gh was asked %v", *calls)
	}
}

func TestAnOldDateReleaseCanStillBeNamedForStable(t *testing.T) {
	tap := tapAnswers(t, "the tested formula", "the old formula")
	calls := recordGitHubCommands(t, func(arguments []string) (string, error) {
		if arguments[0] == "release" && arguments[1] == "view" {
			return `{"tagName":"v2026.10.02.090000"}`, nil
		}
		return tap(arguments)
	})
	if errorValue := promoteNamedHostRelease("v2026.10.02.090000", stableChannel, io.Discard); errorValue != nil {
		t.Fatal(errorValue)
	}
	if (*calls)[0][2] != "v2026.10.02.090000" {
		t.Errorf("gh was asked %v", *calls)
	}
}

func TestANamedReleaseThatIsNotAVersionIsRefusedBeforeGitHubIsAsked(t *testing.T) {
	calls := recordGitHubCommands(t, func([]string) (string, error) { return "", nil })
	if promoteNamedHostRelease("latest", stableChannel, io.Discard) == nil || len(*calls) != 0 {
		t.Errorf("a name that is no version reached gh: %v", *calls)
	}
}

func TestAFullReleaseRefusesWithoutAMilestoneBeforeAnythingIsChecked(t *testing.T) {
	original := checkReleaseTree
	checkReleaseTree = func(string) error { t.Fatal("the tree was checked before the milestone"); return nil }
	t.Cleanup(func() { checkReleaseTree = original })
	calls := recordGitHubCommands(t, func([]string) (string, error) { return "", nil })
	for _, arguments := range [][]string{{"--channel", "testing"}, {"--channel", "stable", "--milestone", "2026.10.07.120000"}, {"--channel", "stable", "--milestone", "0.0.1+3"}} {
		if runReleaseHost(arguments) == nil {
			t.Errorf("%v was released", arguments)
		}
	}
	if len(*calls) != 0 {
		t.Errorf("gh was asked %v", *calls)
	}
}

func TestAMilestoneBelowTheLatestIsRefused(t *testing.T) {
	tree := newShippableTree(t)
	gitInDirectory(t, tree.root, "tag", "v0.0.2")
	for milestone, isRefused := range map[string]bool{"0.0.1": true, "0.0.2": false, "0.0.3": false} {
		parsed, _ := hostversion.ParseMilestone(milestone)
		if errorValue := refuseAMilestoneBelowTheLatest(tree.root, parsed); (errorValue != nil) != isRefused {
			t.Errorf("%s: %v", milestone, errorValue)
		}
	}
}
