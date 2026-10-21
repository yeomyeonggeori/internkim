package hostversion

import (
	"encoding/json"
	"os"
	"os/exec"
	"testing"
)

type fixture struct {
	Valid []struct {
		Input   string `json:"input"`
		Tag     string `json:"tag"`
		Package string `json:"package"`
	} `json:"valid"`
	Invalid     []string `json:"invalid"`
	Ascending   []string `json:"ascending"`
	ReleaseTags struct {
		Accepted []string `json:"accepted"`
		Refused  []string `json:"refused"`
	} `json:"releaseTags"`
}

func loadFixture(t *testing.T) fixture {
	t.Helper()
	contents, errorValue := os.ReadFile("testdata/versions.json")
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	var loaded fixture
	if errorValue := json.Unmarshal(contents, &loaded); errorValue != nil {
		t.Fatal(errorValue)
	}
	return loaded
}

func TestParseNamesEachValidVersionTheSameWayEverywhere(t *testing.T) {
	for _, valid := range loadFixture(t).Valid {
		version, errorValue := Parse(valid.Input)
		if errorValue != nil {
			t.Fatalf("%q was refused: %v", valid.Input, errorValue)
		}
		if version.Tag() != valid.Tag || version.Package() != valid.Package {
			t.Fatalf("%q is tag %s and package %s; want %s and %s", valid.Input, version.Tag(), version.Package(), valid.Tag, valid.Package)
		}
	}
}

func TestParseRefusesWhatIsNotAVersion(t *testing.T) {
	for _, invalid := range loadFixture(t).Invalid {
		if version, errorValue := Parse(invalid); errorValue == nil {
			t.Fatalf("%q parsed as %+v", invalid, version)
		}
	}
}

func TestEveryVersionSortsAboveTheOnesBeforeItInTheList(t *testing.T) {
	ascending := loadFixture(t).Ascending
	for earlier := range ascending {
		for later := earlier + 1; later < len(ascending); later++ {
			if !IsOlder(ascending[earlier], ascending[later]) || IsOlder(ascending[later], ascending[earlier]) {
				t.Fatalf("%s should be older than %s", ascending[earlier], ascending[later])
			}
		}
		if IsOlder(ascending[earlier], ascending[earlier]) {
			t.Fatalf("%s is older than itself", ascending[earlier])
		}
	}
}

func TestAnyDateVersionIsOlderThanAnySemanticPackage(t *testing.T) {
	if !IsOlder("v2099.12.31.235959", "1:0.0.1") || !IsOlder("2099.12.31.235959", "v0.0.1") {
		t.Fatal("a date version sorted above MAJOR.MINOR.PATCH")
	}
}

func TestAnUnreadableVersionIsOlderThanEveryReadableOne(t *testing.T) {
	if !IsOlder("", "v0.0.1") || !IsOlder("dev", "v2026.10.07.120000") || IsOlder("v0.0.1", "") {
		t.Fatal("an unreadable version did not sort lowest")
	}
}

func TestEachBumpRaisesItsPartAndResetsTheOnesBelow(t *testing.T) {
	installed, _ := Parse("1:1.4.7")
	for bump, want := range map[Bump]string{Patch: "1.4.8", Minor: "1.5.0", Major: "2.0.0"} {
		if got := installed.Next(bump).Release(); got != want {
			t.Errorf("%s of 1.4.7 is %s, want %s", bump, got, want)
		}
	}
}

func TestABuildCountedFromATagIsBumpedFromTheTag(t *testing.T) {
	build, _ := Parse("0.0.1+194")
	if got := build.Next(Patch).Release(); got != "0.0.2" {
		t.Fatalf("the patch after 0.0.1+194 is %s", got)
	}
}

func TestNoBumpNamedIsAPatch(t *testing.T) {
	for text, want := range map[string]Bump{"": Patch, "patch": Patch, "minor": Minor, "major": Major} {
		if bump, errorValue := ParseBump(text); errorValue != nil || bump != want {
			t.Errorf("%q is %q (%v)", text, bump, errorValue)
		}
	}
	for _, refused := range []string{"0.0.2", "Patch", "build"} {
		if _, errorValue := ParseBump(refused); errorValue == nil {
			t.Errorf("%q was taken as a bump", refused)
		}
	}
}

func commit(t *testing.T, directory string) {
	t.Helper()
	run(t, directory, "commit", "--allow-empty", "-q", "-m", "work")
}

func run(t *testing.T, directory string, arguments ...string) {
	t.Helper()
	command := exec.Command("git", append([]string{"-C", directory, "-c", "user.name=Sample", "-c", "user.email=sample@example.com", "-c", "commit.gpgsign=false", "-c", "tag.gpgsign=false"}, arguments...)...)
	if output, errorValue := command.CombinedOutput(); errorValue != nil {
		t.Fatalf("git %v: %v: %s", arguments, errorValue, output)
	}
}

func repositoryWithTags(t *testing.T) string {
	t.Helper()
	directory := t.TempDir()
	run(t, directory, "init", "-q")
	commit(t, directory)
	run(t, directory, "tag", "v2026.10.07.120000")
	commit(t, directory)
	run(t, directory, "tag", "v0.0.1")
	return directory
}

func TestTheNextVersionRaisesTheHighestTagNotTheNewest(t *testing.T) {
	directory := repositoryWithTags(t)
	commit(t, directory)
	run(t, directory, "tag", "v0.0.10")
	commit(t, directory)
	run(t, directory, "tag", "v0.0.2")
	commit(t, directory)
	version, errorValue := NextForRevision(directory, "HEAD", Patch)
	if errorValue != nil || version.Package() != "1:0.0.11" {
		t.Fatalf("got %v, %v", version, errorValue)
	}
}

func TestTheNextVersionOfARevisionAlreadyTaggedIsStillANewNumber(t *testing.T) {
	version, errorValue := NextForRevision(repositoryWithTags(t), "HEAD", Patch)
	if errorValue != nil || version.Release() != "0.0.2" {
		t.Fatalf("got %v, %v", version, errorValue)
	}
}

func TestBeforeTheFirstTagTheNextVersionIsTheFirst(t *testing.T) {
	directory := t.TempDir()
	run(t, directory, "init", "-q")
	commit(t, directory)
	run(t, directory, "tag", "v2026.10.07.120000")
	version, errorValue := NextForRevision(directory, "HEAD", Patch)
	if errorValue != nil || version.Release() != "0.0.1" {
		t.Fatalf("got %v, %v", version, errorValue)
	}
}

func TestAHostOnlyBuildIsTheLatestReleaseAndTheCommitsSinceIt(t *testing.T) {
	directory := repositoryWithTags(t)
	atTag, errorValue := ForRevision(directory, "HEAD")
	if errorValue != nil || atTag.Package() != "1:0.0.1" {
		t.Fatalf("at the tag: %v, %v", atTag, errorValue)
	}
	for range 194 {
		commit(t, directory)
	}
	version, errorValue := ForRevision(directory, "HEAD")
	if errorValue != nil || version.Release() != "0.0.1+194" || version.Package() != "1:0.0.1+194" {
		t.Fatalf("194 commits on: %v, %v", version, errorValue)
	}
}

func TestAHostOnlyBuildBeforeTheFirstReleaseIsRefused(t *testing.T) {
	directory := t.TempDir()
	run(t, directory, "init", "-q")
	commit(t, directory)
	run(t, directory, "tag", "v2026.10.07.120000")
	if _, errorValue := ForRevision(directory, "HEAD"); errorValue != ErrNoRelease {
		t.Fatalf("got %v", errorValue)
	}
}

func TestHostOnlyBuildsAndReleasesSortInTheOrderTheyWereMade(t *testing.T) {
	chain := []string{"0.0.1+194", "0.0.1+200", "0.0.2", "0.0.2+3", "0.1.0"}
	for earlier := range chain {
		for later := earlier + 1; later < len(chain); later++ {
			if !IsOlder("1:"+chain[earlier], "1:"+chain[later]) || IsOlder(chain[later], chain[earlier]) {
				t.Fatalf("%s should sort below %s", chain[earlier], chain[later])
			}
		}
	}
}

func TestTheVersionProductionRunsSortsBelowTheNextRelease(t *testing.T) {
	for _, pair := range [][2]string{{"0.0.1+194", "0.0.2"}, {"1:0.0.1+194", "1:0.0.2"}, {"v0.0.1+194", "v0.0.2"}} {
		if !IsOlder(pair[0], pair[1]) || IsOlder(pair[1], pair[0]) {
			t.Fatalf("%s does not sort below %s", pair[0], pair[1])
		}
	}
	if !IsOlder("0.0.1", "0.0.1+194") {
		t.Fatal("0.0.1+194 sorts below the tag it was counted from")
	}
}

func TestAReleaseTagIsTheVPrefixedDateOrMajorMinorPatch(t *testing.T) {
	tags := loadFixture(t).ReleaseTags
	for _, accepted := range tags.Accepted {
		if _, errorValue := ParseReleaseTag(accepted); errorValue != nil {
			t.Errorf("%q was refused: %v", accepted, errorValue)
		}
	}
	for _, refused := range tags.Refused {
		if _, errorValue := ParseReleaseTag(refused); errorValue == nil {
			t.Errorf("%q was taken as a release tag", refused)
		}
	}
}
