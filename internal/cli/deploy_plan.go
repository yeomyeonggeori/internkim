package cli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"gitlab.com/eastriver/internkim/internal/releaseset"
	"gitlab.com/eastriver/internkim/internal/runtime/blueclaw"
)

var prepareReleaseArtifacts = buildReleaseArtifacts

var fetchReleaseHistory = fetchRepositories

func buildReleaseArtifacts(repositoryRootPath string) error {
	fmt.Println("Building the blueclaw payload")
	if errorValue := runBuildStep(repositoryRootPath, nil, "make", "prepare-blueclaw-payload"); errorValue != nil {
		return errorValue
	}
	webRevision := releaseComponentRevision("web", repositoryRootPath, gitRevision(repositoryRootPath))
	if builtBoardUIRevision(repositoryRootPath) == webRevision {
		return nil
	}
	fmt.Println("Building the board UI")
	stamp := []string{"INTERNKIM_WEB_REVISION=" + webRevision}
	return runBuildStep(filepath.Join(repositoryRootPath, "web"), stamp, "bun", "run", "build:board")
}

func runBuildStep(directoryPath string, environment []string, name string, arguments ...string) error {
	command := exec.Command(name, arguments...)
	command.Dir = directoryPath
	command.Env = append(os.Environ(), environment...)
	output, errorValue := command.CombinedOutput()
	if errorValue != nil {
		return fmt.Errorf("%s %s failed: %s", name, strings.Join(arguments, " "), strings.TrimSpace(string(output)))
	}
	return nil
}

func builtBoardUIRevision(repositoryRootPath string) string {
	version, errorValue := readAdminUIVersion(filepath.Join(repositoryRootPath, "build", "board-ui"))
	if errorValue != nil {
		return ""
	}
	return version
}

func fetchRepositories(repositoryRootPath string) {
	for _, repositoryPath := range []string{repositoryRootPath, filepath.Join(repositoryRootPath, blueclaw.BlueclawSubmodulePath)} {
		exec.Command("git", "-C", repositoryPath, "fetch", "--quiet").Run()
	}
}

func revisionIsKnown(repositoryRootPath string, revision string) bool {
	commit, _, _ := strings.Cut(revision, "-dirty-")
	for _, repositoryPath := range []string{repositoryRootPath, filepath.Join(repositoryRootPath, blueclaw.BlueclawSubmodulePath)} {
		if exec.Command("git", "-C", repositoryPath, "cat-file", "-e", commit+"^{commit}").Run() == nil {
			return true
		}
	}
	return false
}

var commitIdentifier = regexp.MustCompile(`^[0-9a-f]{40}(-dirty-[0-9a-f]+)?$`)

func revisionIsAncestor(repositoryRootPath string, older string, newer string) bool {
	for _, repositoryPath := range []string{repositoryRootPath, filepath.Join(repositoryRootPath, blueclaw.BlueclawSubmodulePath)} {
		if exec.Command("git", "-C", repositoryPath, "merge-base", "--is-ancestor", older, newer).Run() == nil {
			return true
		}
	}
	return false
}

type deployPlan struct {
	selected map[string]bool
	held     map[string]releaseset.Component
	refusals []string
}

// selected is nil for the whole set and empty when the device already holds
// everything this tree builds.
func chooseDeployComponents(
	repositoryRootPath string,
	narrowedComponentNames map[string]bool,
	readDevice func() (map[string]releaseset.Component, error),
	isKnown func(revision string) bool,
	isAncestor func(older string, newer string) bool,
) (deployPlan, error) {
	if errorValue := prepareReleaseArtifacts(repositoryRootPath); errorValue != nil {
		return deployPlan{}, errorValue
	}
	held, readError := readDevice()
	if len(narrowedComponentNames) > 0 {
		return deployPlan{selected: narrowedComponentNames, held: held}, nil
	}
	if readError != nil {
		return deployPlan{}, fmt.Errorf("read what the device holds: %w", readError)
	}
	if len(held) == 0 {
		return deployPlan{held: held}, nil
	}
	fetchReleaseHistory(repositoryRootPath)
	treeRevision := gitRevision(repositoryRootPath)
	selected, refusals := selectDeployComponents(held, func(name string) string {
		return releaseComponentRevision(name, repositoryRootPath, treeRevision)
	}, isKnown, isAncestor)
	return deployPlan{selected: selected, held: held, refusals: refusals}, nil
}

func selectDeployComponents(
	held map[string]releaseset.Component,
	revisionOf func(string) string,
	isKnown func(revision string) bool,
	isAncestor func(older string, newer string) bool,
) (map[string]bool, []string) {
	selected := map[string]bool{}
	refusals := []string{}
	for _, name := range ReleaseComponentNames() {
		expected := revisionOf(name)
		carried, isHeld := held[name]
		if expected == "" || (isHeld && carried.Revision == expected) {
			continue
		}
		if isHeld && commitIdentifier.MatchString(carried.Revision) && !isKnown(carried.Revision) {
			refusals = append(refusals, fmt.Sprintf("%s (device holds %s, which this checkout does not know; fetch or check what was deployed)", name, shortRevision(carried.Revision)))
			continue
		}
		if isHeld && isAncestor(expected, carried.Revision) {
			refusals = append(refusals, describeComponentAhead(name, carried.Revision, expected))
			continue
		}
		selected[name] = true
	}
	sort.Strings(refusals)
	addProtocolPartners(selected, held)
	return selected, refusals
}

func refusalError(refusals []string) error {
	return fmt.Errorf(
		"deploy refuses to ship over:\n  %s\nfetch, update this branch from origin/main, or name the component in --components to overwrite it on purpose",
		strings.Join(refusals, "\n  "),
	)
}

func describeComponentAhead(name string, carried string, expected string) string {
	return fmt.Sprintf("%s (the device is ahead: it holds %s, this tree builds %s)", name, shortRevision(carried), shortRevision(expected))
}

func addProtocolPartners(selected map[string]bool, held map[string]releaseset.Component) {
	if len(selected) == 0 {
		return
	}
	speaksProtocol := false
	for _, name := range componentsThatSpeakOneProtocol {
		speaksProtocol = speaksProtocol || selected[name]
	}
	if !speaksProtocol {
		return
	}
	for _, name := range componentsThatSpeakOneProtocol {
		if _, isHeld := held[name]; isHeld {
			selected[name] = true
		}
	}
}

func printDeploySelection(names []string, held map[string]releaseset.Component, revisionOf func(string) string) {
	for _, name := range names {
		fmt.Printf("Shipping %s\n", describeShippedComponent(name, held[name].Revision, revisionOf(name)))
	}
}

func describeShippedComponent(name string, carried string, expected string) string {
	if carried == "" {
		return fmt.Sprintf("%s (new, this tree builds %s)", name, shortRevision(expected))
	}
	return fmt.Sprintf("%s (%s -> %s)", name, shortRevision(carried), shortRevision(expected))
}

func verifyDeployedRevisions(
	shippedNames []string,
	revisionOf func(string) string,
	deployed map[string]releaseComponentBrief,
) error {
	sort.Strings(shippedNames)
	mismatched := []string{}
	for _, name := range shippedNames {
		expected, actual := revisionOf(name), deployed[name].Revision
		fmt.Printf("Verify %s: device %s, tree %s\n", name, shortRevision(actual), shortRevision(expected))
		if actual != expected {
			mismatched = append(mismatched, name)
		}
	}
	if len(mismatched) == 0 {
		return nil
	}
	return fmt.Errorf("the device does not hold this tree's revision of: %s", strings.Join(mismatched, ", "))
}
