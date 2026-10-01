package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/yeomyeonggeori/internkim/internal/runtime/blueclaw"
)

// The company host ships as a GitHub Release of this repository: stable is the
// latest release, and testing is the newest one, prerelease or not, which is
// what web/static/install.sh resolves each channel to.
const (
	hostReleaseRepository = "yeomyeonggeori/internkim"
	stableChannel         = "stable"
	testingChannel        = "testing"
)

type gitHubRelease struct {
	TagName      string `json:"tagName"`
	IsPrerelease bool   `json:"isPrerelease"`
}

var runGitHubCommand = func(arguments ...string) (string, error) {
	output, errorValue := exec.Command("gh", arguments...).CombinedOutput()
	if errorValue != nil {
		return string(output), fmt.Errorf("gh %s: %w: %s", strings.Join(arguments, " "), errorValue, strings.TrimSpace(string(output)))
	}
	return string(output), nil
}

var checkReleaseTree = refuseUnreleasableTree

func runReleaseHost(arguments []string) error {
	channel := commandArgumentValue(arguments, "--channel", "")
	if channel != stableChannel && channel != testingChannel {
		return fmt.Errorf("release host --channel takes %s or %s", stableChannel, testingChannel)
	}
	repositoryRootPath, errorValue := resolveRepositoryRootPath()
	if errorValue != nil {
		return errorValue
	}
	if errorValue := checkReleaseTree(repositoryRootPath); errorValue != nil {
		return errorValue
	}
	version := packageVersionFromRepository(repositoryRootPath)
	tag := "v" + version
	existing, isPublished, errorValue := publishedHostRelease(tag)
	if errorValue != nil {
		return errorValue
	}
	if isPublished {
		return promoteHostRelease(existing, channel, os.Stdout)
	}
	directory, errorValue := os.MkdirTemp("", "internkim-host-release-*")
	if errorValue != nil {
		return errorValue
	}
	defer os.RemoveAll(directory)
	if errorValue := buildHostRelease(repositoryRootPath, packageTargets, version, directory, linuxPackageFormats(), os.Stdout); errorValue != nil {
		return errorValue
	}
	return createHostRelease(tag, gitRevision(repositoryRootPath), channel, directory, os.Stdout)
}

// refuseUnreleasableTree holds a release to what deploy holds a device to, and
// one thing more: the commit is already on main, because a tag is public and
// permanent and a branch is neither. Together the two leave origin/main's tip,
// so no release is ever older than the one before it.
func refuseUnreleasableTree(repositoryRootPath string) error {
	blueclawPath := filepath.Join(repositoryRootPath, blueclaw.BlueclawSubmodulePath)
	if output, errorValue := gitOutput(repositoryRootPath, "fetch", "--quiet", "origin"); errorValue != nil {
		return fmt.Errorf("git fetch origin failed, so the tree cannot be checked against it: %s", output)
	}
	reasons := []string{}
	if changed := trackedChanges(repositoryRootPath); len(changed) > 0 {
		reasons = append(reasons, "uncommitted changes, which would ship under a commit's name: "+strings.Join(changed, ", "))
	}
	if changed := trackedChanges(blueclawPath); len(changed) > 0 {
		reasons = append(reasons, "uncommitted changes inside .dependency/blueclaw: "+strings.Join(changed, ", "))
	}
	if isOnMain, errorValue := gitIsAncestor(repositoryRootPath, "HEAD", "origin/main"); errorValue != nil || !isOnMain {
		reasons = append(reasons, "HEAD is not on origin/main; release a commit main already has")
	}
	reasons = append(reasons, historyReasons(repositoryRootPath, "this tree's HEAD", "origin/main")...)
	reasons = append(reasons, blueclawPointerReasons(repositoryRootPath, blueclawPath)...)
	if len(reasons) == 0 {
		return nil
	}
	return fmt.Errorf("release host refuses to publish this tree:\n  %s", strings.Join(reasons, "\n  "))
}

// publishedHostRelease asks before anything is built, so a gh that is signed
// out or cannot see the repository stops the release in seconds.
func publishedHostRelease(tag string) (gitHubRelease, bool, error) {
	output, errorValue := runGitHubCommand("release", "view", tag, "--repo", hostReleaseRepository, "--json", "tagName,isPrerelease")
	if errorValue != nil && strings.Contains(output, "release not found") {
		return gitHubRelease{}, false, nil
	}
	if errorValue != nil {
		return gitHubRelease{}, false, errorValue
	}
	var release gitHubRelease
	if errorValue := json.Unmarshal([]byte(output), &release); errorValue != nil {
		return gitHubRelease{}, false, fmt.Errorf("gh release view %s answered something other than its JSON: %w", tag, errorValue)
	}
	return release, true, nil
}

// promoteHostRelease makes a tested build stable without rebuilding it, so
// stable carries the bytes testing carried.
func promoteHostRelease(release gitHubRelease, channel string, output io.Writer) error {
	if channel == testingChannel || !release.IsPrerelease {
		return fmt.Errorf("%s is already released on %s; a new release needs a new commit on main", release.TagName, releasedChannel(release))
	}
	if _, errorValue := runGitHubCommand("release", "edit", release.TagName, "--repo", hostReleaseRepository, "--prerelease=false", "--latest"); errorValue != nil {
		return errorValue
	}
	fmt.Fprintf(output, "promoted %s from testing to stable\n", release.TagName)
	return nil
}

func releasedChannel(release gitHubRelease) string {
	if release.IsPrerelease {
		return testingChannel
	}
	return stableChannel
}

func createHostRelease(tag string, revision string, channel string, directory string, output io.Writer) error {
	assets := []string{}
	for _, name := range append(hostReleaseAssetNames(), releaseChecksumsName) {
		path := filepath.Join(directory, name)
		if _, errorValue := os.Stat(path); errorValue != nil {
			return errors.Join(fmt.Errorf("the release is missing %s", name), errorValue)
		}
		assets = append(assets, path)
	}
	arguments := []string{"release", "create", tag, "--repo", hostReleaseRepository, "--target", revision, "--title", tag, "--notes", hostReleaseNotes(channel)}
	if channel == testingChannel {
		arguments = append(arguments, "--prerelease")
	} else {
		arguments = append(arguments, "--latest")
	}
	if _, errorValue := runGitHubCommand(append(arguments, assets...)...); errorValue != nil {
		return errorValue
	}
	fmt.Fprintf(output, "released %s on %s: https://github.com/%s/releases/tag/%s\n", tag, channel, hostReleaseRepository, tag)
	return nil
}

func hostReleaseNotes(channel string) string {
	installLine := "curl -fsSL https://intern.kim/install.sh | sh -s -- host"
	if channel == testingChannel {
		installLine += " --channel testing"
	}
	return "Install or upgrade the company host:\n\n```sh\n" + installLine + "\n```\n"
}
