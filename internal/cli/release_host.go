package cli

import (
	"encoding/base64"
	"encoding/json"
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
// what web/static/install.sh resolves each channel to on Linux. A Mac installs
// through the Homebrew tap, whose formula is the one the stable release carries.
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
	existing, isPublished, errorValue := publishedHostRelease(hostReleaseTag(version))
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
	everyFormat, errorValue := releaseFormatsNamed("")
	if errorValue != nil {
		return errorValue
	}
	if errorValue := buildHostRelease(repositoryRootPath, packageTargets, version, directory, everyFormat, os.Stdout); errorValue != nil {
		return errorValue
	}
	return createHostRelease(version, gitRevision(repositoryRootPath), channel, directory, os.Stdout)
}

func hostReleaseTag(version string) string {
	return "v" + version
}

// hostReleaseDownloadURL is where GitHub serves one release's assets, which the
// formula names as its url and its bottle's root_url.
func hostReleaseDownloadURL(version string) string {
	return "https://github.com/" + hostReleaseRepository + "/releases/download/" + hostReleaseTag(version)
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
// stable carries the bytes testing carried. Stable over stable gives the tap the
// release's formula again, which is how a tap update that failed is retried.
func promoteHostRelease(release gitHubRelease, channel string, output io.Writer) error {
	if channel == testingChannel {
		return fmt.Errorf("%s is already released on %s; a new release needs a new commit on main", release.TagName, releasedChannel(release))
	}
	if release.IsPrerelease {
		if _, errorValue := runGitHubCommand("release", "edit", release.TagName, "--repo", hostReleaseRepository, "--prerelease=false", "--latest"); errorValue != nil {
			return errorValue
		}
		fmt.Fprintf(output, "promoted %s from testing to stable\n", release.TagName)
	}
	return publishFormulaToTap(release.TagName, output)
}

func releasedChannel(release gitHubRelease) string {
	if release.IsPrerelease {
		return testingChannel
	}
	return stableChannel
}

func createHostRelease(version string, revision string, channel string, directory string, output io.Writer) error {
	assets, errorValue := completeReleaseAssets(directory, version)
	if errorValue != nil {
		return errorValue
	}
	tag := hostReleaseTag(version)
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
	if channel == testingChannel {
		return nil
	}
	return publishFormulaToTap(tag, output)
}

// completeReleaseAssets is the paths a release uploads, refused unless the
// directory holds every package, the Homebrew tarball and formula, exactly one
// bottle, and the checksums.
func completeReleaseAssets(directory string, version string) ([]string, error) {
	present, errorValue := releaseAssetsIn(directory, version)
	if errorValue != nil {
		return nil, errorValue
	}
	missing := []string{}
	for _, name := range append(releaseAssetNamesWithoutBottle(), releaseChecksumsName) {
		if _, errorValue := os.Stat(filepath.Join(directory, name)); errorValue != nil {
			missing = append(missing, name)
		}
	}
	bottles, errorValue := homebrewBottlesIn(directory, version)
	if errorValue != nil {
		return nil, errorValue
	}
	if len(bottles) != 1 {
		missing = append(missing, fmt.Sprintf("exactly one Homebrew bottle of %s (it holds %d)", version, len(bottles)))
	}
	if len(missing) > 0 {
		return nil, fmt.Errorf("the release is missing %s", strings.Join(missing, ", "))
	}
	paths := []string{}
	for _, name := range append(present, releaseChecksumsName) {
		paths = append(paths, filepath.Join(directory, name))
	}
	return paths, nil
}

func hostReleaseNotes(channel string) string {
	installLine := "curl -fsSL https://intern.kim/install.sh | sh -s -- host"
	if channel == testingChannel {
		installLine += " --channel testing"
	}
	return "Install or upgrade the company host:\n\n```sh\n" + installLine + "\n```\n"
}

// publishFormulaToTap commits the formula a stable release carries to the tap,
// which is the only way a formula reaches it. The copy is the release's own
// asset, so a promoted testing build gives the tap the formula it was tested with.
func publishFormulaToTap(tag string, output io.Writer) error {
	formula, errorValue := releasedFormula(tag)
	if errorValue != nil {
		return errorValue
	}
	current, errorValue := tapFormula()
	if errorValue != nil {
		return errorValue
	}
	if current.Content == formula {
		fmt.Fprintf(output, "%s already carries the formula of %s\n", blueclaw.HomebrewTap(), tag)
		return nil
	}
	arguments := []string{
		"api", "--method", "PUT", tapFormulaPath(),
		"-f", "message=Carry " + blueclaw.CompanyPackageName + " " + tag,
		"-f", "content=" + base64.StdEncoding.EncodeToString([]byte(formula)),
	}
	if current.SHA != "" {
		arguments = append(arguments, "-f", "sha="+current.SHA)
	}
	if _, errorValue := runGitHubCommand(arguments...); errorValue != nil {
		return errorValue
	}
	fmt.Fprintf(output, "gave %s the formula of %s\n", blueclaw.HomebrewTap(), tag)
	return nil
}

func releasedFormula(tag string) (string, error) {
	directory, errorValue := os.MkdirTemp("", "internkim-formula-*")
	if errorValue != nil {
		return "", errorValue
	}
	defer os.RemoveAll(directory)
	if _, errorValue := runGitHubCommand("release", "download", tag, "--repo", hostReleaseRepository,
		"--pattern", blueclaw.HomebrewFormulaAssetName(), "--dir", directory); errorValue != nil {
		return "", errorValue
	}
	formula, errorValue := os.ReadFile(filepath.Join(directory, blueclaw.HomebrewFormulaAssetName()))
	if errorValue != nil {
		return "", fmt.Errorf("%s carries no %s: %w", tag, blueclaw.HomebrewFormulaAssetName(), errorValue)
	}
	return string(formula), nil
}

type tapFile struct {
	SHA     string
	Content string
}

// tapFormula is the formula the tap carries now, and its blob SHA, which the
// contents API needs to replace it. A tap with no formula yet answers empty.
func tapFormula() (tapFile, error) {
	answer, errorValue := runGitHubCommand("api", tapFormulaPath())
	if errorValue != nil && strings.Contains(answer, "HTTP 404") {
		return tapFile{}, nil
	}
	if errorValue != nil {
		return tapFile{}, errorValue
	}
	var contents struct {
		SHA     string `json:"sha"`
		Content string `json:"content"`
	}
	if errorValue := json.Unmarshal([]byte(answer), &contents); errorValue != nil {
		return tapFile{}, fmt.Errorf("the contents API answered something other than its JSON for %s: %w", tapFormulaPath(), errorValue)
	}
	decoded, errorValue := base64.StdEncoding.DecodeString(strings.ReplaceAll(contents.Content, "\n", ""))
	if errorValue != nil {
		return tapFile{}, fmt.Errorf("the tap's formula is not base64: %w", errorValue)
	}
	return tapFile{SHA: contents.SHA, Content: string(decoded)}, nil
}

func tapFormulaPath() string {
	return "repos/" + blueclaw.HomebrewTapRepository() + "/contents/" + blueclaw.HomebrewFormulaFileName()
}
