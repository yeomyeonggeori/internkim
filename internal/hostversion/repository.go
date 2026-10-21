package hostversion

import (
	"errors"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

var ErrNoRelease = errors.New("no vMAJOR.MINOR.PATCH release is tagged yet; the first is made with tools/ship-host --release")

func Latest(repositoryPath string, revision string) (Version, error) {
	output, errorValue := git(repositoryPath, "tag", "--merged", revision)
	if errorValue != nil {
		return Version{}, errorValue
	}
	latest, isFound := Version{Kind: Semantic}, false
	for _, tag := range strings.Fields(output) {
		version, errorValue := ParseReleaseTag(tag)
		if errorValue != nil || version.Kind != Semantic {
			continue
		}
		if !isFound || Compare(version, latest) > 0 {
			latest, isFound = version, true
		}
	}
	if !isFound {
		return latest, ErrNoRelease
	}
	return latest, nil
}

func NextForRevision(repositoryPath string, revision string, bump Bump) (Version, error) {
	latest, errorValue := Latest(repositoryPath, revision)
	if errorValue != nil && !errors.Is(errorValue, ErrNoRelease) {
		return Version{}, errorValue
	}
	return latest.Next(bump), nil
}

func ForRevision(repositoryPath string, revision string) (Version, error) {
	latest, errorValue := Latest(repositoryPath, revision)
	if errorValue != nil {
		return Version{}, errorValue
	}
	output, errorValue := git(repositoryPath, "rev-list", "--count", latest.Tag()+".."+revision)
	if errorValue != nil {
		return Version{}, errorValue
	}
	commits, errorValue := strconv.Atoi(strings.TrimSpace(output))
	if errorValue != nil {
		return Version{}, fmt.Errorf("git rev-list --count answered %q", output)
	}
	return latest.WithCommits(commits), nil
}

func git(repositoryPath string, arguments ...string) (string, error) {
	output, errorValue := exec.Command("git", append([]string{"-C", repositoryPath}, arguments...)...).CombinedOutput()
	if errorValue != nil {
		return "", fmt.Errorf("git %s: %w: %s", strings.Join(arguments, " "), errorValue, strings.TrimSpace(string(output)))
	}
	return string(output), nil
}
