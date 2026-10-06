package hostversion

import (
	"encoding/json"
	"os"
	"os/exec"
	"strconv"
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

func TestAnyDateVersionIsOlderThanAnyMilestonePackage(t *testing.T) {
	if !IsOlder("v2099.12.31.235959", "1:0.0.1") || !IsOlder("2099.12.31.235959", "v0.0.1") {
		t.Fatal("a date version sorted above a milestone")
	}
}

func TestAnUnreadableVersionIsOlderThanEveryReadableOne(t *testing.T) {
	if !IsOlder("", "v0.0.1") || !IsOlder("dev", "v2026.10.07.120000") || IsOlder("v0.0.1", "") {
		t.Fatal("an unreadable version did not sort lowest")
	}
}

func TestParseMilestoneTakesOnlyThePlainNumber(t *testing.T) {
	for _, accepted := range []string{"0.0.1", "v0.0.1"} {
		if _, errorValue := ParseMilestone(accepted); errorValue != nil {
			t.Fatalf("%q refused: %v", accepted, errorValue)
		}
	}
	for _, refused := range []string{"", "0.0.1+3", "1:0.0.1", "2026.10.07.120000", "latest"} {
		if _, errorValue := ParseMilestone(refused); errorValue == nil {
			t.Fatalf("%q was taken as a milestone", refused)
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

func TestForRevisionIsTheLastMilestoneAndTheCommitsSinceIt(t *testing.T) {
	directory := repositoryWithTags(t)
	atTag, errorValue := ForRevision(directory, "HEAD")
	if errorValue != nil || atTag.Package() != "1:0.0.1" {
		t.Fatalf("at the tag: %v, %v", atTag, errorValue)
	}
	for range 37 {
		commit(t, directory)
	}
	version, errorValue := ForRevision(directory, "HEAD")
	if errorValue != nil || version.Release() != "0.0.1+37" || version.Package() != "1:0.0.1+37" {
		t.Fatalf("37 commits on: %v, %v", version, errorValue)
	}
}

func TestForRevisionChoosesTheHighestMilestoneNotTheNewestTag(t *testing.T) {
	directory := repositoryWithTags(t)
	commit(t, directory)
	run(t, directory, "tag", "v0.0.10")
	commit(t, directory)
	run(t, directory, "tag", "v0.0.2")
	commit(t, directory)
	version, errorValue := ForRevision(directory, "HEAD")
	if errorValue != nil || version.Release() != "0.0.10+2" {
		t.Fatalf("got %v, %v", version, errorValue)
	}
}

func TestForRevisionRefusesBeforeTheFirstMilestone(t *testing.T) {
	directory := t.TempDir()
	run(t, directory, "init", "-q")
	commit(t, directory)
	run(t, directory, "tag", "v2026.10.07.120000")
	if _, errorValue := ForRevision(directory, "HEAD"); errorValue != ErrNoMilestone {
		t.Fatalf("got %v", errorValue)
	}
}

func TestAHostOnlyBuildSortsBetweenItsMilestoneAndTheNext(t *testing.T) {
	for commits := 1; commits < 1000; commits *= 10 {
		build := "0.0.1+" + strconv.Itoa(commits)
		if !IsOlder("1:0.0.1", build) || !IsOlder(build, "1:0.0.2") {
			t.Fatalf("%s is not between 0.0.1 and 0.0.2", build)
		}
	}
}

func TestAReleaseTagIsTheVPrefixedDateOrMilestone(t *testing.T) {
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
