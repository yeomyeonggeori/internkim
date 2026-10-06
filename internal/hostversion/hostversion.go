package hostversion

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

const MilestoneEpoch = 1

var (
	datePattern      = regexp.MustCompile(`^(\d{4})\.(\d{2})\.(\d{2})\.(\d{6})$`)
	milestonePattern = regexp.MustCompile(`^(0|[1-9]\d{0,8})\.(0|[1-9]\d{0,8})\.(0|[1-9]\d{0,8})(?:\+([1-9]\d{0,8}))?$`)
)

type Kind int

const (
	Date Kind = iota
	Milestone
)

type Version struct {
	Kind    Kind
	Date    string
	Major   int
	Minor   int
	Patch   int
	Commits int
}

func Parse(text string) (Version, error) {
	trimmed := strings.TrimPrefix(strings.TrimSpace(text), "v")
	epoch, release, hasEpoch := strings.Cut(trimmed, ":")
	if !hasEpoch {
		epoch, release = "", trimmed
	}
	version, errorValue := parseRelease(release)
	if errorValue != nil {
		return Version{}, fmt.Errorf("%q is not a host version: %w", text, errorValue)
	}
	if hasEpoch && epoch != strconv.Itoa(version.Epoch()) {
		return Version{}, fmt.Errorf("%q is not a host version: %s versions carry epoch %d, not %q", text, version.kindName(), version.Epoch(), epoch)
	}
	return version, nil
}

func ParseMilestone(text string) (Version, error) {
	version, errorValue := Parse(text)
	if errorValue != nil {
		return Version{}, errorValue
	}
	if version.Kind != Milestone || version.Commits != 0 || strings.Contains(text, ":") {
		return Version{}, fmt.Errorf("%q is not a milestone version; a milestone is MAJOR.MINOR.PATCH, such as 0.0.1", text)
	}
	return version, nil
}

func ParseReleaseTag(text string) (Version, error) {
	version, errorValue := Parse(text)
	if errorValue != nil {
		return Version{}, errorValue
	}
	if !strings.HasPrefix(text, "v") || strings.Contains(text, ":") || version.Commits != 0 {
		return Version{}, fmt.Errorf("%q is not a release tag; a release is tagged v0.0.1 or, for the date releases before it, v2026.10.07.120000", text)
	}
	return version, nil
}

func parseRelease(release string) (Version, error) {
	if found := datePattern.FindStringSubmatch(release); found != nil {
		return Version{Kind: Date, Date: strings.Join(found[1:], ".")}, nil
	}
	if found := milestonePattern.FindStringSubmatch(release); found != nil {
		return Version{Kind: Milestone, Major: number(found[1]), Minor: number(found[2]), Patch: number(found[3]), Commits: number(found[4])}, nil
	}
	return Version{}, fmt.Errorf("expected MAJOR.MINOR.PATCH, MAJOR.MINOR.PATCH+COMMITS or YYYY.MM.DD.HHMMSS")
}

func number(digits string) int {
	value, _ := strconv.Atoi(digits)
	return value
}

func (version Version) Epoch() int {
	if version.Kind == Milestone {
		return MilestoneEpoch
	}
	return 0
}

func (version Version) kindName() string {
	if version.Kind == Milestone {
		return "milestone"
	}
	return "date"
}

func (version Version) Release() string {
	if version.Kind == Date {
		return version.Date
	}
	release := fmt.Sprintf("%d.%d.%d", version.Major, version.Minor, version.Patch)
	if version.Commits == 0 {
		return release
	}
	return fmt.Sprintf("%s+%d", release, version.Commits)
}

func (version Version) Tag() string {
	return "v" + version.Release()
}

func (version Version) Package() string {
	if version.Epoch() == 0 {
		return version.Release()
	}
	return fmt.Sprintf("%d:%s", version.Epoch(), version.Release())
}

func (version Version) Base() Version {
	version.Commits = 0
	return version
}

func (version Version) WithCommits(commits int) Version {
	version.Commits = commits
	return version
}

func Compare(left Version, right Version) int {
	if byEpoch := compareNumbers(left.Epoch(), right.Epoch()); byEpoch != 0 {
		return byEpoch
	}
	if left.Kind == Date {
		return strings.Compare(left.Date, right.Date)
	}
	for _, pair := range [][2]int{{left.Major, right.Major}, {left.Minor, right.Minor}, {left.Patch, right.Patch}, {left.Commits, right.Commits}} {
		if byPart := compareNumbers(pair[0], pair[1]); byPart != 0 {
			return byPart
		}
	}
	return 0
}

func compareNumbers(left int, right int) int {
	switch {
	case left < right:
		return -1
	case left > right:
		return 1
	}
	return 0
}

func IsOlder(version string, than string) bool {
	left, leftError := Parse(version)
	right, rightError := Parse(than)
	switch {
	case leftError != nil && rightError != nil:
		return version < than
	case leftError != nil:
		return true
	case rightError != nil:
		return false
	}
	return Compare(left, right) < 0
}
